package teams

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrTeamNotFound      = errors.New("team not found")
	ErrTeamAlreadyExists = errors.New("team with this name already exists")
	ErrMemberNotFound    = errors.New("user is not a member of this team")
	ErrUserNotFound      = errors.New("user not found")
	ErrNotParticipant    = errors.New("only participants can be on a team")
	ErrAlreadyInTeam     = errors.New("user is already in a team")
	ErrNotInTeam         = errors.New("you are not in a team")
	ErrInvalidCode       = errors.New("invalid join code")
	ErrTeamFull          = errors.New("team is full")
	ErrTeamFrozen        = errors.New("teams are frozen")
	ErrNotLeader         = errors.New("only the team leader can do this")
	ErrLeaderNotMember   = errors.New("leader must be a member of the team")
	ErrCannotKickSelf    = errors.New("use leave to remove yourself from the team")
)

const (
	settingFrozenAll   = "teams.frozen_all"
	settingMaxTeamSize = "teams.max_size"

	// Unambiguous alphabet: no 0/O or 1/I/L.
	joinCodeAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"
	joinCodeLength   = 6
	joinCodeRetries  = 8
)

// querier is satisfied by both *pgxpool.Pool and pgx.Tx.
type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type Repository struct {
	db             *pgxpool.Pool
	defaultMaxSize int
}

func NewRepository(db *pgxpool.Pool, defaultMaxSize int) *Repository {
	if defaultMaxSize <= 0 {
		defaultMaxSize = 4
	}
	return &Repository{db: db, defaultMaxSize: defaultMaxSize}
}

func generateJoinCode() (string, error) {
	max := big.NewInt(int64(len(joinCodeAlphabet)))
	b := make([]byte, joinCodeLength)
	for i := range b {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", fmt.Errorf("generate join code: %w", err)
		}
		b[i] = joinCodeAlphabet[n.Int64()]
	}
	return string(b), nil
}

func normalizeCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

func isConstraint(err error, name string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == name
}

// ---- settings ----

func (r *Repository) loadSettings(ctx context.Context, q querier) (SettingsResponse, error) {
	s := SettingsResponse{MaxTeamSize: r.defaultMaxSize, MaxTeamSizeSource: "environment"}

	rows, err := q.Query(ctx, `SELECT key, value FROM app_settings WHERE key = ANY($1)`,
		[]string{settingFrozenAll, settingMaxTeamSize})
	if err != nil {
		return s, fmt.Errorf("load team settings: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var key string
		var raw json.RawMessage
		if err := rows.Scan(&key, &raw); err != nil {
			return s, fmt.Errorf("scan team setting: %w", err)
		}
		switch key {
		case settingFrozenAll:
			_ = json.Unmarshal(raw, &s.FrozenAll)
		case settingMaxTeamSize:
			var n int
			if json.Unmarshal(raw, &n) == nil && n > 0 {
				s.MaxTeamSize = n
				s.MaxTeamSizeSource = "database"
			}
		}
	}
	return s, rows.Err()
}

func (r *Repository) GetSettings(ctx context.Context) (SettingsResponse, error) {
	return r.loadSettings(ctx, r.db)
}

func (r *Repository) setSetting(ctx context.Context, key string, value any, updatedBy string) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode setting: %w", err)
	}
	_, err = r.db.Exec(ctx, `
		INSERT INTO app_settings (key, value, updated_by) VALUES ($1, $2, $3)
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_by = EXCLUDED.updated_by, updated_at = CURRENT_TIMESTAMP`,
		key, raw, updatedBy)
	if err != nil {
		return fmt.Errorf("set setting: %w", err)
	}
	return nil
}

func (r *Repository) SetFrozenAll(ctx context.Context, frozen bool, updatedBy string) error {
	return r.setSetting(ctx, settingFrozenAll, frozen, updatedBy)
}

func (r *Repository) SetMaxTeamSize(ctx context.Context, size int, updatedBy string) error {
	return r.setSetting(ctx, settingMaxTeamSize, size, updatedBy)
}

// ---- participant flows ----

// lockParticipant locks the user row and returns their current team (nil when none).
func lockParticipant(ctx context.Context, tx pgx.Tx, userID string) (*string, error) {
	var role string
	var teamID *string
	err := tx.QueryRow(ctx,
		`SELECT role::text, team_id FROM users WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`, userID,
	).Scan(&role, &teamID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("lock user: %w", err)
	}
	if role != "participant" {
		return nil, ErrNotParticipant
	}
	return teamID, nil
}

// lockUser takes a row lock on a user so team changes for them serialize.
func lockUser(ctx context.Context, tx pgx.Tx, userID string) error {
	var id string
	err := tx.QueryRow(ctx, `SELECT id FROM users WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`, userID).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrUserNotFound
		}
		return fmt.Errorf("lock user: %w", err)
	}
	return nil
}

type teamLock struct {
	ID       string
	LeaderID *string
	Frozen   bool
	MaxSize  *int
}

func (t teamLock) effectiveMax(s SettingsResponse) int {
	if t.MaxSize != nil {
		return *t.MaxSize
	}
	return s.MaxTeamSize
}

func (t teamLock) effectiveFrozen(s SettingsResponse) bool {
	return t.Frozen || s.FrozenAll
}

// lockTeam locks a team row by id or by join code.
func lockTeam(ctx context.Context, tx pgx.Tx, where string, arg string) (*teamLock, error) {
	var t teamLock
	err := tx.QueryRow(ctx,
		`SELECT id, leader_id, is_frozen, max_size FROM teams WHERE `+where+` FOR UPDATE`, arg,
	).Scan(&t.ID, &t.LeaderID, &t.Frozen, &t.MaxSize)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTeamNotFound
		}
		return nil, fmt.Errorf("lock team: %w", err)
	}
	return &t, nil
}

func memberCount(ctx context.Context, tx pgx.Tx, teamID string) (int, error) {
	var n int
	if err := tx.QueryRow(ctx,
		`SELECT COUNT(*) FROM users WHERE team_id = $1 AND deleted_at IS NULL`, teamID).Scan(&n); err != nil {
		return 0, fmt.Errorf("count members: %w", err)
	}
	return n, nil
}

// CreateTeam creates a team led by userID and returns its id.
func (r *Repository) CreateTeam(ctx context.Context, userID, name string) (string, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin create team tx: %w", err)
	}
	defer tx.Rollback(ctx)

	current, err := lockParticipant(ctx, tx, userID)
	if err != nil {
		return "", err
	}
	if current != nil {
		return "", ErrAlreadyInTeam
	}
	settings, err := r.loadSettings(ctx, tx)
	if err != nil {
		return "", err
	}
	if settings.FrozenAll {
		return "", ErrTeamFrozen
	}

	var teamID string
	for attempt := 0; ; attempt++ {
		code, err := generateJoinCode()
		if err != nil {
			return "", err
		}
		// Savepoint so a code collision doesn't abort the whole transaction.
		sp, err := tx.Begin(ctx)
		if err != nil {
			return "", fmt.Errorf("begin savepoint: %w", err)
		}
		err = sp.QueryRow(ctx,
			`INSERT INTO teams (name, leader_id, join_code, created_by) VALUES ($1, $2, $3, $2) RETURNING id`,
			strings.TrimSpace(name), userID, code).Scan(&teamID)
		if err == nil {
			if err := sp.Commit(ctx); err != nil {
				return "", fmt.Errorf("release savepoint: %w", err)
			}
			break
		}
		_ = sp.Rollback(ctx)
		if isConstraint(err, "teams_name_key") {
			return "", ErrTeamAlreadyExists
		}
		if isConstraint(err, "teams_join_code_key") && attempt < joinCodeRetries {
			continue
		}
		return "", fmt.Errorf("insert team: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`UPDATE users SET team_id = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`, teamID, userID); err != nil {
		return "", fmt.Errorf("link creator to team: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit create team tx: %w", err)
	}
	return teamID, nil
}

// JoinByCode adds userID to the team that owns the code and returns the team id.
func (r *Repository) JoinByCode(ctx context.Context, userID, code string) (string, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin join tx: %w", err)
	}
	defer tx.Rollback(ctx)

	current, err := lockParticipant(ctx, tx, userID)
	if err != nil {
		return "", err
	}
	if current != nil {
		return "", ErrAlreadyInTeam
	}

	team, err := lockTeam(ctx, tx, "join_code = $1", normalizeCode(code))
	if err != nil {
		if errors.Is(err, ErrTeamNotFound) {
			return "", ErrInvalidCode
		}
		return "", err
	}
	settings, err := r.loadSettings(ctx, tx)
	if err != nil {
		return "", err
	}
	if team.effectiveFrozen(settings) {
		return "", ErrTeamFrozen
	}
	count, err := memberCount(ctx, tx, team.ID)
	if err != nil {
		return "", err
	}
	if count >= team.effectiveMax(settings) {
		return "", ErrTeamFull
	}

	if _, err := tx.Exec(ctx,
		`UPDATE users SET team_id = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`, team.ID, userID); err != nil {
		return "", fmt.Errorf("join team: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit join tx: %w", err)
	}
	return team.ID, nil
}

// removeMemberTx detaches userID from teamID. A departing leader hands leadership to the
// longest-standing remaining member; a team left with nobody is deleted.
// It returns the team's remaining member count (0 means the team was deleted).
func removeMemberTx(ctx context.Context, tx pgx.Tx, team *teamLock, userID string) (int, error) {
	tag, err := tx.Exec(ctx,
		`UPDATE users SET team_id = NULL, updated_at = CURRENT_TIMESTAMP WHERE id = $1 AND team_id = $2`,
		userID, team.ID)
	if err != nil {
		return 0, fmt.Errorf("remove member from team: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return 0, ErrMemberNotFound
	}

	remaining, err := memberCount(ctx, tx, team.ID)
	if err != nil {
		return 0, err
	}
	if remaining == 0 {
		if _, err := tx.Exec(ctx, `DELETE FROM teams WHERE id = $1`, team.ID); err != nil {
			return 0, fmt.Errorf("delete empty team: %w", err)
		}
		return 0, nil
	}

	if team.LeaderID != nil && *team.LeaderID == userID {
		if _, err := tx.Exec(ctx, `
			UPDATE teams SET updated_at = CURRENT_TIMESTAMP, leader_id = (
				SELECT id FROM users WHERE team_id = $1 AND deleted_at IS NULL
				ORDER BY created_at ASC, id ASC LIMIT 1
			) WHERE id = $1`, team.ID); err != nil {
			return 0, fmt.Errorf("transfer team leadership: %w", err)
		}
	}
	return remaining, nil
}

// LeaveTeam removes userID from their team and returns the team id they left.
func (r *Repository) LeaveTeam(ctx context.Context, userID string) (string, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin leave tx: %w", err)
	}
	defer tx.Rollback(ctx)

	current, err := lockParticipant(ctx, tx, userID)
	if err != nil {
		return "", err
	}
	if current == nil {
		return "", ErrNotInTeam
	}
	team, err := lockTeam(ctx, tx, "id = $1", *current)
	if err != nil {
		return "", err
	}
	settings, err := r.loadSettings(ctx, tx)
	if err != nil {
		return "", err
	}
	if team.effectiveFrozen(settings) {
		return "", ErrTeamFrozen
	}
	if _, err := removeMemberTx(ctx, tx, team, userID); err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit leave tx: %w", err)
	}
	return team.ID, nil
}

// leaderTeamTx locks the caller's team and verifies they lead it.
func (r *Repository) leaderTeamTx(ctx context.Context, tx pgx.Tx, userID string) (*teamLock, SettingsResponse, error) {
	var zero SettingsResponse
	current, err := lockParticipant(ctx, tx, userID)
	if err != nil {
		return nil, zero, err
	}
	if current == nil {
		return nil, zero, ErrNotInTeam
	}
	team, err := lockTeam(ctx, tx, "id = $1", *current)
	if err != nil {
		return nil, zero, err
	}
	if team.LeaderID == nil || *team.LeaderID != userID {
		return nil, zero, ErrNotLeader
	}
	settings, err := r.loadSettings(ctx, tx)
	if err != nil {
		return nil, zero, err
	}
	return team, settings, nil
}

// RenameOwnTeam renames the team led by userID.
func (r *Repository) RenameOwnTeam(ctx context.Context, userID, name string) (string, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin rename tx: %w", err)
	}
	defer tx.Rollback(ctx)

	team, settings, err := r.leaderTeamTx(ctx, tx, userID)
	if err != nil {
		return "", err
	}
	if team.effectiveFrozen(settings) {
		return "", ErrTeamFrozen
	}
	if _, err := tx.Exec(ctx,
		`UPDATE teams SET name = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`,
		strings.TrimSpace(name), team.ID); err != nil {
		if isConstraint(err, "teams_name_key") {
			return "", ErrTeamAlreadyExists
		}
		return "", fmt.Errorf("rename team: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit rename tx: %w", err)
	}
	return team.ID, nil
}

// RegenerateOwnCode issues a fresh join code for the team led by userID.
func (r *Repository) RegenerateOwnCode(ctx context.Context, userID string) (string, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin regenerate tx: %w", err)
	}
	defer tx.Rollback(ctx)

	team, _, err := r.leaderTeamTx(ctx, tx, userID)
	if err != nil {
		return "", err
	}
	if err := regenerateCodeTx(ctx, tx, team.ID); err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit regenerate tx: %w", err)
	}
	return team.ID, nil
}

func regenerateCodeTx(ctx context.Context, tx pgx.Tx, teamID string) error {
	for attempt := 0; attempt <= joinCodeRetries; attempt++ {
		code, err := generateJoinCode()
		if err != nil {
			return err
		}
		sp, err := tx.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin savepoint: %w", err)
		}
		_, err = sp.Exec(ctx,
			`UPDATE teams SET join_code = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`, code, teamID)
		if err == nil {
			return sp.Commit(ctx)
		}
		_ = sp.Rollback(ctx)
		if !isConstraint(err, "teams_join_code_key") {
			return fmt.Errorf("regenerate join code: %w", err)
		}
	}
	return errors.New("could not generate a unique join code")
}

// KickMember lets the leader remove another member from their team.
func (r *Repository) KickMember(ctx context.Context, leaderID, targetID string) (string, error) {
	if leaderID == targetID {
		return "", ErrCannotKickSelf
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin kick tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := lockUser(ctx, tx, targetID); err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return "", ErrMemberNotFound
		}
		return "", err
	}
	team, settings, err := r.leaderTeamTx(ctx, tx, leaderID)
	if err != nil {
		return "", err
	}
	if team.effectiveFrozen(settings) {
		return "", ErrTeamFrozen
	}
	if _, err := removeMemberTx(ctx, tx, team, targetID); err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit kick tx: %w", err)
	}
	return team.ID, nil
}

// ---- reads ----

func (r *Repository) GetTeamByID(ctx context.Context, id string) (*TeamDetailResponse, error) {
	settings, err := r.loadSettings(ctx, r.db)
	if err != nil {
		return nil, err
	}

	var resp TeamDetailResponse
	var leaderID *string
	err = r.db.QueryRow(ctx, `
		SELECT id, name, join_code, leader_id, is_frozen, max_size, created_at, updated_at
		FROM teams WHERE id = $1`, id,
	).Scan(&resp.ID, &resp.Name, &resp.JoinCode, &leaderID, &resp.IsFrozen, &resp.MaxSize, &resp.CreatedAt, &resp.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTeamNotFound
		}
		return nil, fmt.Errorf("get team by id: %w", err)
	}
	resp.EffectiveFrozen = resp.IsFrozen || settings.FrozenAll
	resp.EffectiveMaxSize = settings.MaxTeamSize
	if resp.MaxSize != nil {
		resp.EffectiveMaxSize = *resp.MaxSize
	}

	rows, err := r.db.Query(ctx, `
		SELECT id, email, name, phone_number, role, is_password_configured
		FROM users
		WHERE team_id = $1 AND deleted_at IS NULL
		ORDER BY created_at ASC, id ASC`, id)
	if err != nil {
		return nil, fmt.Errorf("fetch team members: %w", err)
	}
	defer rows.Close()

	resp.Members = make([]TeamMemberSummary, 0)
	for rows.Next() {
		var m TeamMemberSummary
		if err := rows.Scan(&m.ID, &m.Email, &m.Name, &m.PhoneNumber, &m.Role, &m.IsPasswordConfigured); err != nil {
			return nil, fmt.Errorf("scan member row: %w", err)
		}
		if leaderID != nil && m.ID == *leaderID {
			m.IsLeader = true
			leader := m
			resp.Leader = &leader
		}
		resp.Members = append(resp.Members, m)
	}
	return &resp, rows.Err()
}

func (r *Repository) GetTeamByUserID(ctx context.Context, userID string) (*TeamDetailResponse, error) {
	var teamID *string
	err := r.db.QueryRow(ctx, "SELECT team_id FROM users WHERE id = $1 AND deleted_at IS NULL", userID).Scan(&teamID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTeamNotFound
		}
		return nil, fmt.Errorf("query user team: %w", err)
	}
	if teamID == nil {
		return nil, ErrTeamNotFound
	}
	return r.GetTeamByID(ctx, *teamID)
}

// judgeTeamsCondition matches teams holding a slot in a panel the given judge ($n) is assigned to.
const judgeTeamsCondition = `t.id IN (
	SELECT s.team_id FROM judging_slots s
	JOIN judging_panel_judges pj ON pj.panel_id = s.panel_id
	WHERE pj.judge_id = $%d
)`

// IsTeamAssignedToJudge reports whether teamID is scheduled in any panel judgeID is assigned to.
func (r *Repository) IsTeamAssignedToJudge(ctx context.Context, teamID, judgeID string) (bool, error) {
	var ok bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM judging_slots s
			JOIN judging_panel_judges pj ON pj.panel_id = s.panel_id
			WHERE s.team_id = $1 AND pj.judge_id = $2
		)`, teamID, judgeID).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("check judge team assignment: %w", err)
	}
	return ok, nil
}

func (r *Repository) ListTeams(ctx context.Context, filter TeamFilter) ([]TeamResponse, int64, error) {
	settings, err := r.loadSettings(ctx, r.db)
	if err != nil {
		return nil, 0, err
	}

	var conditions []string
	var args []any
	argIndex := 1

	if filter.Search != "" {
		conditions = append(conditions, fmt.Sprintf("(LOWER(t.name) LIKE $%d OR t.join_code = UPPER($%d))", argIndex, argIndex+1))
		args = append(args, "%"+strings.ToLower(filter.Search)+"%", strings.TrimSpace(filter.Search))
		argIndex += 2
	}
	if filter.JudgeID != nil {
		conditions = append(conditions, fmt.Sprintf(judgeTeamsCondition, argIndex))
		args = append(args, *filter.JudgeID)
		argIndex++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	var totalCount int64
	if err := r.db.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM teams t %s", whereClause), args...).Scan(&totalCount); err != nil {
		return nil, 0, fmt.Errorf("count teams: %w", err)
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	} else if limit > 100 {
		limit = 100
	}
	page := filter.Page
	if page <= 0 {
		page = 1
	}

	query := fmt.Sprintf(`
		SELECT t.id, t.name, t.join_code, t.leader_id, u.name, u.email,
		       COUNT(m.id) AS member_count, t.is_frozen, t.max_size, t.created_at, t.updated_at
		FROM teams t
		LEFT JOIN users u ON t.leader_id = u.id AND u.deleted_at IS NULL
		LEFT JOIN users m ON t.id = m.team_id AND m.deleted_at IS NULL
		%s
		GROUP BY t.id, t.name, t.join_code, t.leader_id, u.name, u.email, t.is_frozen, t.max_size, t.created_at, t.updated_at
		ORDER BY t.created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIndex, argIndex+1)
	args = append(args, limit, (page-1)*limit)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list teams query: %w", err)
	}
	defer rows.Close()

	teamsList := make([]TeamResponse, 0, limit)
	for rows.Next() {
		var t TeamResponse
		if err := rows.Scan(
			&t.ID, &t.Name, &t.JoinCode, &t.LeaderID, &t.LeaderName, &t.LeaderEmail,
			&t.MemberCount, &t.IsFrozen, &t.MaxSize, &t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan team row: %w", err)
		}
		t.EffectiveFrozen = t.IsFrozen || settings.FrozenAll
		t.EffectiveMaxSize = settings.MaxTeamSize
		if t.MaxSize != nil {
			t.EffectiveMaxSize = *t.MaxSize
		}
		teamsList = append(teamsList, t)
	}
	return teamsList, totalCount, rows.Err()
}

// ---- staff flows (not blocked by freezes) ----

func (r *Repository) UpdateTeam(ctx context.Context, id string, req UpdateTeamRequest) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin update team tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := lockTeam(ctx, tx, "id = $1", id); err != nil {
		return err
	}

	var sets []string
	var args []any
	argIndex := 1

	if req.Name != nil {
		sets = append(sets, fmt.Sprintf("name = $%d", argIndex))
		args = append(args, strings.TrimSpace(*req.Name))
		argIndex++
	}
	if req.LeaderID != nil {
		var isMember bool
		if err := tx.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM users WHERE id = $1 AND team_id = $2 AND deleted_at IS NULL)`,
			*req.LeaderID, id).Scan(&isMember); err != nil {
			return fmt.Errorf("check leader membership: %w", err)
		}
		if !isMember {
			return ErrLeaderNotMember
		}
		sets = append(sets, fmt.Sprintf("leader_id = $%d", argIndex))
		args = append(args, *req.LeaderID)
		argIndex++
	}
	if req.MaxSize != nil {
		if *req.MaxSize == 0 {
			sets = append(sets, "max_size = NULL")
		} else {
			sets = append(sets, fmt.Sprintf("max_size = $%d", argIndex))
			args = append(args, *req.MaxSize)
			argIndex++
		}
	}

	if len(sets) > 0 {
		sets = append(sets, "updated_at = CURRENT_TIMESTAMP")
		args = append(args, id)
		if _, err := tx.Exec(ctx,
			fmt.Sprintf("UPDATE teams SET %s WHERE id = $%d", strings.Join(sets, ", "), argIndex), args...); err != nil {
			if isConstraint(err, "teams_name_key") {
				return ErrTeamAlreadyExists
			}
			return fmt.Errorf("update team: %w", err)
		}
	}
	return tx.Commit(ctx)
}

func (r *Repository) SetFrozen(ctx context.Context, id string, frozen bool) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE teams SET is_frozen = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`, frozen, id)
	if err != nil {
		return fmt.Errorf("set team frozen: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrTeamNotFound
	}
	return nil
}

func (r *Repository) RegenerateCode(ctx context.Context, id string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin regenerate tx: %w", err)
	}
	defer tx.Rollback(ctx)
	if _, err := lockTeam(ctx, tx, "id = $1", id); err != nil {
		return err
	}
	if err := regenerateCodeTx(ctx, tx, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) DeleteTeam(ctx context.Context, id string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin delete team tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, "UPDATE users SET team_id = NULL WHERE team_id = $1", id); err != nil {
		return fmt.Errorf("detach team members: %w", err)
	}
	tag, err := tx.Exec(ctx, "DELETE FROM teams WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("delete team: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrTeamNotFound
	}
	return tx.Commit(ctx)
}

// AddMember lets staff place a participant who has no team into teamID.
func (r *Repository) AddMember(ctx context.Context, teamID, userID string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin add member tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Lock order is always user then team, matching join/leave.
	current, err := lockParticipant(ctx, tx, userID)
	if err != nil {
		return err
	}
	team, err := lockTeam(ctx, tx, "id = $1", teamID)
	if err != nil {
		return err
	}
	if current != nil {
		return ErrAlreadyInTeam
	}
	settings, err := r.loadSettings(ctx, tx)
	if err != nil {
		return err
	}
	count, err := memberCount(ctx, tx, team.ID)
	if err != nil {
		return err
	}
	if count >= team.effectiveMax(settings) {
		return ErrTeamFull
	}
	if _, err := tx.Exec(ctx,
		`UPDATE users SET team_id = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`, team.ID, userID); err != nil {
		return fmt.Errorf("add member to team: %w", err)
	}
	return tx.Commit(ctx)
}

// RemoveMember lets staff remove a member; leadership transfer and empty-team cleanup still apply.
func (r *Repository) RemoveMember(ctx context.Context, teamID, userID string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin remove member tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := lockUser(ctx, tx, userID); err != nil {
		return err
	}
	team, err := lockTeam(ctx, tx, "id = $1", teamID)
	if err != nil {
		return err
	}
	if _, err := removeMemberTx(ctx, tx, team, userID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
