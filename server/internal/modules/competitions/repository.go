package competitions

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrCompetitionNotFound = errors.New("competition not found")
	ErrSlugAlreadyExists   = errors.New("competition slug already in use in this event")
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// CreateCompetition inserts a new competition encapsulated inside eventID.
func (r *Repository) CreateCompetition(ctx context.Context, eventID, slug, name string, description *string) (*Competition, error) {
	query := `
		INSERT INTO competitions (event_id, slug, name, description)
		VALUES ($1, LOWER($2), $3, $4)
		RETURNING id, event_id, slug, name, description, created_at, updated_at
	`
	var c Competition
	if err := r.db.QueryRow(ctx, query, eventID, slug, name, description).Scan(
		&c.ID, &c.EventID, &c.Slug, &c.Name, &c.Description, &c.CreatedAt, &c.UpdatedAt,
	); err != nil {
		if strings.Contains(err.Error(), "uq_competitions_event_slug") || strings.Contains(err.Error(), "competitions_slug_key") {
			return nil, ErrSlugAlreadyExists
		}
		return nil, fmt.Errorf("insert competition: %w", err)
	}
	return &c, nil
}

// GetCompetitionBySlug retrieves a competition by its eventID and vanity slug.
func (r *Repository) GetCompetitionBySlug(ctx context.Context, eventID, slug string) (*Competition, error) {
	query := `
		SELECT id, event_id, slug, name, description, created_at, updated_at
		FROM competitions
		WHERE event_id = $1 AND LOWER(slug) = LOWER($2)
	`
	var c Competition
	err := r.db.QueryRow(ctx, query, eventID, slug).Scan(
		&c.ID, &c.EventID, &c.Slug, &c.Name, &c.Description, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCompetitionNotFound
		}
		return nil, fmt.Errorf("get competition by slug: %w", err)
	}
	return &c, nil
}

// GetCompetitionByID retrieves a competition by its UUID.
func (r *Repository) GetCompetitionByID(ctx context.Context, id string) (*Competition, error) {
	query := `
		SELECT id, event_id, slug, name, description, created_at, updated_at
		FROM competitions
		WHERE id = $1
	`
	var c Competition
	err := r.db.QueryRow(ctx, query, id).Scan(
		&c.ID, &c.EventID, &c.Slug, &c.Name, &c.Description, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCompetitionNotFound
		}
		return nil, fmt.Errorf("get competition by id: %w", err)
	}
	return &c, nil
}

// ListCompetitionsByEvent lists all competitions belonging to an event.
func (r *Repository) ListCompetitionsByEvent(ctx context.Context, eventID string) ([]Competition, error) {
	query := `
		SELECT id, event_id, slug, name, description, created_at, updated_at
		FROM competitions
		WHERE event_id = $1
		ORDER BY created_at ASC
	`
	rows, err := r.db.Query(ctx, query, eventID)
	if err != nil {
		return nil, fmt.Errorf("list competitions by event: %w", err)
	}
	defer rows.Close()

	list := make([]Competition, 0)
	for rows.Next() {
		var c Competition
		if err := rows.Scan(&c.ID, &c.EventID, &c.Slug, &c.Name, &c.Description, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan competition: %w", err)
		}
		list = append(list, c)
	}
	return list, rows.Err()
}

// UpdateCompetition updates metadata (slug, name, description) for a competition.
func (r *Repository) UpdateCompetition(ctx context.Context, id string, slug, name, description *string) (*Competition, error) {
	var sets []string
	var args []any
	argIndex := 1

	if slug != nil {
		sets = append(sets, fmt.Sprintf("slug = LOWER($%d)", argIndex))
		args = append(args, *slug)
		argIndex++
	}
	if name != nil {
		sets = append(sets, fmt.Sprintf("name = $%d", argIndex))
		args = append(args, *name)
		argIndex++
	}
	if description != nil {
		sets = append(sets, fmt.Sprintf("description = $%d", argIndex))
		args = append(args, *description)
		argIndex++
	}
	if len(sets) == 0 {
		return r.GetCompetitionByID(ctx, id)
	}

	sets = append(sets, "updated_at = CURRENT_TIMESTAMP")
	query := fmt.Sprintf("UPDATE competitions SET %s WHERE id = $%d RETURNING id, event_id, slug, name, description, created_at, updated_at", strings.Join(sets, ", "), argIndex)
	args = append(args, id)

	var c Competition
	err := r.db.QueryRow(ctx, query, args...).Scan(
		&c.ID, &c.EventID, &c.Slug, &c.Name, &c.Description, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCompetitionNotFound
		}
		if strings.Contains(err.Error(), "uq_competitions_event_slug") || strings.Contains(err.Error(), "competitions_slug_key") {
			return nil, ErrSlugAlreadyExists
		}
		return nil, fmt.Errorf("update competition: %w", err)
	}
	return &c, nil
}

// DeleteCompetition deletes a competition by ID and returns the deleted entity for cache invalidation and auditing.
func (r *Repository) DeleteCompetition(ctx context.Context, id string) (*Competition, error) {
	comp, err := r.GetCompetitionByID(ctx, id)
	if err != nil {
		return nil, err
	}
	tag, err := r.db.Exec(ctx, `DELETE FROM competitions WHERE id = $1`, id)
	if err != nil {
		return nil, fmt.Errorf("delete competition: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrCompetitionNotFound
	}
	return comp, nil
}
