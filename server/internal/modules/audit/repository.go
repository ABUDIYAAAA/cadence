package audit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) InsertAuditLog(ctx context.Context, e Event) error {
	e = SanitizeEvent(e)

	metaJSON, err := json.Marshal(e.Metadata)
	if err != nil || len(e.Metadata) == 0 {
		metaJSON = []byte("{}")
	}

	createdAt := e.Timestamp
	if createdAt.IsZero() {
		createdAt = time.Now()
	}

	query := `
		INSERT INTO audit_logs (
			event_id, competition_id, actor_id, actor_email, actor_role, action, target_type, target_id,
			description, metadata, ip_address, user_agent, device_id, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`

	_, err = r.db.Exec(ctx, query,
		e.EventID, e.CompetitionID, e.ActorID, e.ActorEmail, e.ActorRole, e.Action, e.TargetType, e.TargetID,
		e.Description, metaJSON, e.IPAddress, e.UserAgent, e.DeviceID, createdAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && (pgErr.Code == "23503" || pgErr.Code == "22P02") {
			// If a referenced foreign key (actor_id, event_id, competition_id) no longer exists or is malformed,
			// preserve the audit event with NULL references.
			_, err = r.db.Exec(ctx, query,
				nil, nil, nil, e.ActorEmail, e.ActorRole, e.Action, e.TargetType, e.TargetID,
				e.Description, metaJSON, e.IPAddress, e.UserAgent, e.DeviceID, createdAt,
			)
		}
	}
	if err != nil {
		return fmt.Errorf("insert audit log: %w", err)
	}

	return nil
}

// IsPermanentAuditInsertError reports whether an InsertAuditLog error is a permanent
// PostgreSQL data or constraint error (SQLSTATE class 22 or 23) that will never succeed on retry.
func IsPermanentAuditInsertError(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return strings.HasPrefix(pgErr.Code, "22") || strings.HasPrefix(pgErr.Code, "23")
}

func (r *Repository) ListAuditLogs(ctx context.Context, filter AuditLogFilter) ([]AuditLogResponse, int64, error) {
	var conditions []string
	var args []any
	argIndex := 1

	if filter.EventID != "" {
		conditions = append(conditions, fmt.Sprintf("(event_id = $%d OR event_id IS NULL)", argIndex))
		args = append(args, filter.EventID)
		argIndex++
	}

	if filter.CompetitionID != "" {
		conditions = append(conditions, fmt.Sprintf("competition_id = $%d", argIndex))
		args = append(args, filter.CompetitionID)
		argIndex++
	}

	if filter.Action != "" {
		conditions = append(conditions, fmt.Sprintf("action = $%d", argIndex))
		args = append(args, filter.Action)
		argIndex++
	}

	if filter.ActorID != "" {
		conditions = append(conditions, fmt.Sprintf("actor_id = $%d", argIndex))
		args = append(args, filter.ActorID)
		argIndex++
	}

	if filter.TargetType != "" {
		conditions = append(conditions, fmt.Sprintf("target_type = $%d", argIndex))
		args = append(args, filter.TargetType)
		argIndex++
	}

	if filter.TargetID != "" {
		conditions = append(conditions, fmt.Sprintf("target_id = $%d", argIndex))
		args = append(args, filter.TargetID)
		argIndex++
	}

	if filter.Search != "" {
		searchTerm := "%" + strings.ToLower(filter.Search) + "%"
		conditions = append(conditions, fmt.Sprintf("(LOWER(description) LIKE $%d OR LOWER(actor_email) LIKE $%d)", argIndex, argIndex))
		args = append(args, searchTerm)
		argIndex++
	}

	if filter.FromDate != nil {
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", argIndex))
		args = append(args, *filter.FromDate)
		argIndex++
	}

	if filter.ToDate != nil {
		conditions = append(conditions, fmt.Sprintf("created_at <= $%d", argIndex))
		args = append(args, *filter.ToDate)
		argIndex++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM audit_logs %s", whereClause)
	var totalCount int64
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&totalCount); err != nil {
		return nil, 0, fmt.Errorf("count audit logs: %w", err)
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	} else if limit > 200 {
		limit = 200
	}

	page := filter.Page
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit

	query := fmt.Sprintf(`
		SELECT id, event_id, competition_id, actor_id, actor_email, actor_role, action, target_type, target_id,
		       description, metadata, ip_address, user_agent, device_id, created_at
		FROM audit_logs
		%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIndex, argIndex+1)

	args = append(args, limit, offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list audit logs query: %w", err)
	}
	defer rows.Close()

	list := make([]AuditLogResponse, 0, limit)
	for rows.Next() {
		var item AuditLogResponse
		var metaBytes []byte
		if err := rows.Scan(
			&item.ID, &item.EventID, &item.CompetitionID, &item.ActorID, &item.ActorEmail, &item.ActorRole, &item.Action,
			&item.TargetType, &item.TargetID, &item.Description, &metaBytes,
			&item.IPAddress, &item.UserAgent, &item.DeviceID, &item.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan audit log: %w", err)
		}
		item.Metadata = make(map[string]any)
		if len(metaBytes) > 0 {
			_ = json.Unmarshal(metaBytes, &item.Metadata)
		}
		list = append(list, item)
	}

	return list, totalCount, nil
}
