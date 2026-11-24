package teams

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"devclubxnst.com/cadence/internal/modules/auth"
	"devclubxnst.com/cadence/internal/modules/competitions"
	"devclubxnst.com/cadence/pkg/audit"
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
	ErrUserIsBanned      = errors.New("you are banned from this event")
)

const (
	DefaultCompetitionID = "00000000-0000-0000-0000-000000000001"

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
	auditor        audit.Recorder
}

// EvictedTeamChange captures a team dissolution or leadership succession caused by member eviction.
type EvictedTeamChange struct {
	TeamID        string
	TeamName      string
	CompetitionID string
	EventID       string
	UserID        string
	Deleted       bool
	NewLeaderID   *string
}

func NewRepository(db *pgxpool.Pool, defaultMaxSize int) *Repository {
	if defaultMaxSize <= 0 {
		defaultMaxSize = 4
	}
	return &Repository{db: db, defaultMaxSize: defaultMaxSize}
}

// WithAuditor attaches an audit recorder to the repository so cross-module evictions record team.update and team.delete audit events.
func (r *Repository) WithAuditor(auditor audit.Recorder) *Repository {
	r.auditor = auditor
	return r
}

// RecordEvictionAudits emits team.delete or team.update audit events for each team affected by an eviction.
func (r *Repository) RecordEvictionAudits(ctx context.Context, auditor audit.Recorder, changes []EvictedTeamChange) {
	rec := auditor
	if rec == nil {
		rec = r.auditor
	}
	if rec == nil || len(changes) == 0 {
		return
	}
	var actorID, actorEmail, actorRole *string
	if claims, _ := auth.GetUserClaims(ctx); claims != nil {
		actorID = &claims.UserID
		actorEmail = &claims.Email
		role := auth.GetEffectiveRole(ctx)
		actorRole = &role
	}
	for _, ch := range changes {
		teamID := ch.TeamID
		var evID *string
		if ch.EventID != "" {
			eid := ch.EventID
			evID = &eid
		}
		if ch.Deleted {
			rec.Record(ctx, audit.Event{
				EventID:     evID,
				ActorID:     actorID,
				ActorEmail:  actorEmail,
				ActorRole:   actorRole,
				Action:      audit.ActionTeamDelete,
				TargetType:  audit.TargetTeam,
				TargetID:    &teamID,
				Description: fmt.Sprintf("Dissolved empty team %s after member %s left or was evicted", ch.TeamName, ch.UserID),
				Metadata: map[string]any{
					"team_id":        ch.TeamID,
					"team_name":      ch.TeamName,
					"competition_id": ch.CompetitionID,
					"evicted_user":   ch.UserID,
				},
			})
		} else if ch.NewLeaderID != nil {
			rec.Record(ctx, audit.Event{
				EventID:     evID,
				ActorID:     actorID,
				ActorEmail:  actorEmail,
				ActorRole:   actorRole,
				Action:      audit.ActionTeamUpdate,
				TargetType:  audit.TargetTeam,
				TargetID:    &teamID,
				Description: fmt.Sprintf("Transferred leadership of team %s to %s after leader %s was evicted", ch.TeamName, *ch.NewLeaderID, ch.UserID),
				Metadata: map[string]any{
					"team_id":        ch.TeamID,
					"team_name":      ch.TeamName,
					"competition_id": ch.CompetitionID,
					"new_leader_id":  *ch.NewLeaderID,
					"former_leader":  ch.UserID,
				},
			})
		}
	}
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

const defaultEventID = "00000000-0000-0000-0000-000000000001"

func resolveEventIDFromContext(ctx context.Context) string {
	if compCtx, ok := competitions.FromContext(ctx); ok && compCtx != nil && compCtx.EventID != "" {
		return compCtx.EventID
	}
	if comp, ok := competitions.GetCompetition(ctx); ok && comp != nil && comp.EventID != "" {
		return comp.EventID
	}
	return defaultEventID
}

func resolveCompetitionIDFromContext(ctx context.Context) (string, bool) {
	if compCtx, ok := competitions.FromContext(ctx); ok && compCtx != nil && compCtx.CompetitionID != "" {
		return compCtx.CompetitionID, true
	}
	if comp, ok := competitions.GetCompetition(ctx); ok && comp != nil && comp.ID != "" {
		return comp.ID, true
	}
	return "", false
}

func compSettingKey(compID, key string) string {
	if compID == "" {
		compID = DefaultCompetitionID
	}
	return fmt.Sprintf("comp:%s:%s", compID, key)
}

// ---- settings ----

func (r *Repository) loadSettings(ctx context.Context, q querier) (SettingsResponse, error) {
	compID, _ := resolveCompetitionIDFromContext(ctx)
	return r.loadSettingsForComp(ctx, q, compID)
}

func (r *Repository) loadSettingsForComp(ctx context.Context, q querier, compID string) (SettingsResponse, error) {
	s := SettingsResponse{MaxTeamSize: r.defaultMaxSize, MaxTeamSizeSource: "environment"}
	eventID := resolveEventIDFromContext(ctx)
	frozenKey := compSettingKey(compID, settingFrozenAll)
	maxSizeKey := compSettingKey(compID, settingMaxTeamSize)

	rows, err := q.Query(ctx, `SELECT key, value FROM app_settings WHERE event_id = $1 AND key = ANY($2)`,
		eventID, []string{frozenKey, maxSizeKey})
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
		case frozenKey:
			_ = json.Unmarshal(raw, &s.FrozenAll)
		case maxSizeKey:
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
	eventID := resolveEventIDFromContext(ctx)
	compID, _ := resolveCompetitionIDFromContext(ctx)
	scopedKey := compSettingKey(compID, key)
	_, err = r.db.Exec(ctx, `
		INSERT INTO app_settings (event_id, key, value, updated_by) VALUES ($1, $2, $3, NULLIF($4, '')::uuid)
		ON CONFLICT (event_id, key) DO UPDATE SET value = EXCLUDED.value, updated_by = EXCLUDED.updated_by, updated_at = CURRENT_TIMESTAMP`,
		eventID, scopedKey, raw, updatedBy)
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

// lockParticipant locks the user row, verifies event membership & non-banned status via the competition's parent event,
// and returns their current team in compID (nil when none).
func lockParticipant(ctx context.Context, tx pgx.Tx, compID, userID string) (*string, error) {
	if compID == "" {
		compID = DefaultCompetitionID
	}
	var role string
	var isBanned bool
	var teamID *string
	err := tx.QueryRow(ctx,
		`SELECT COALESCE(ue.role::text, 'participant'), COALESCE(ue.is_banned, FALSE), tm.team_id
		 FROM users u
		 LEFT JOIN competitions c ON c.id = $1
		 LEFT JOIN user_events ue ON ue.user_id = u.id AND ue.event_id = COALESCE(c.event_id, '00000000-0000-0000-0000-000000000001'::uuid)
		 LEFT JOIN team_members tm ON tm.user_id = u.id AND tm.competition_id = $1
		 WHERE u.id = $2 AND u.deleted_at IS NULL FOR UPDATE OF u`, compID, userID,
	).Scan(&role, &isBanned, &teamID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("lock user: %w", err)
	}
	if isBanned {
		return nil, ErrUserIsBanned
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
	ID            string
	Name          string
	CompetitionID string
	LeaderID      *string
	Frozen        bool
	MaxSize       *int
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

// lockTeam locks a team row by where condition and arguments.
func lockTeam(ctx context.Context, tx pgx.Tx, where string, args ...any) (*teamLock, error) {
	var t teamLock
	err := tx.QueryRow(ctx,
		`SELECT id, name, competition_id, leader_id, is_frozen, max_size FROM teams WHERE `+where+` FOR UPDATE`, args...,
	).Scan(&t.ID, &t.Name, &t.CompetitionID, &t.LeaderID, &t.Frozen, &t.MaxSize)
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
		`SELECT COUNT(*) FROM team_members tm JOIN users u ON u.id = tm.user_id WHERE tm.team_id = $1 AND u.deleted_at IS NULL`, teamID).Scan(&n); err != nil {
		return 0, fmt.Errorf("count members: %w", err)
	}
	return n, nil
}

// CreateTeam creates a team led by userID in compID and returns its id.
func (r *Repository) CreateTeam(ctx context.Context, compID, userID, name string) (string, error) {
	if compID == "" {
		compID = DefaultCompetitionID
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin create team tx: %w", err)
	}
	defer tx.Rollback(ctx)

	current, err := lockParticipant(ctx, tx, compID, userID)
	if err != nil {
		return "", err
	}
	if current != nil {
		return "", ErrAlreadyInTeam
	}
	settings, err := r.loadSettingsForComp(ctx, tx, compID)
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
			`INSERT INTO teams (competition_id, name, leader_id, join_code, created_by) VALUES ($1, $2, $3, $4, $3) RETURNING id`,
			compID, strings.TrimSpace(name), userID, code).Scan(&teamID)
		if err == nil {
			if err := sp.Commit(ctx); err != nil {
				return "", fmt.Errorf("release savepoint: %w", err)
			}
			break
		}
		_ = sp.Rollback(ctx)
		if isConstraint(err, "teams_name_key") || isConstraint(err, "teams_competition_id_name_key") {
			return "", ErrTeamAlreadyExists
		}
		if (isConstraint(err, "teams_join_code_key") || isConstraint(err, "teams_competition_id_join_code_key")) && attempt < joinCodeRetries {
			continue
		}
		return "", fmt.Errorf("insert team: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO team_members (team_id, user_id, competition_id) VALUES ($1, $2, $3)`, teamID, userID, compID); err != nil {
		return "", fmt.Errorf("insert team member: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit create team tx: %w", err)
	}
	return teamID, nil
}

// JoinByCode adds userID to the team that owns the code within compID and returns the team id.
func (r *Repository) JoinByCode(ctx context.Context, compID, userID, code string) (string, error) {
	if compID == "" {
		compID = DefaultCompetitionID
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin join tx: %w", err)
	}
	defer tx.Rollback(ctx)

	current, err := lockParticipant(ctx, tx, compID, userID)
	if err != nil {
		return "", err
	}
	if current != nil {
		return "", ErrAlreadyInTeam
	}

	team, err := lockTeam(ctx, tx, "competition_id = $1 AND join_code = $2", compID, normalizeCode(code))
	if err != nil {
		if errors.Is(err, ErrTeamNotFound) {
			return "", ErrInvalidCode
		}
		return "", err
	}
	settings, err := r.loadSettingsForComp(ctx, tx, compID)
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
		`INSERT INTO team_members (team_id, user_id, competition_id) VALUES ($1, $2, $3)`, team.ID, userID, compID); err != nil {
		return "", fmt.Errorf("insert team member: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit join tx: %w", err)
	}
	return team.ID, nil
}

// removeMemberTx detaches userID from teamID. A departing leader hands leadership to the
// longest-standing remaining member; a team left with nobody is deleted.
// It returns the team's remaining member count (0 means the team was deleted) and any new leader ID.
func removeMemberTx(ctx context.Context, tx pgx.Tx, team *teamLock, userID string) (int, *string, error) {
	tag, err := tx.Exec(ctx,
		`DELETE FROM team_members WHERE team_id = $1 AND user_id = $2`,
		team.ID, userID)
	if err != nil {
		return 0, nil, fmt.Errorf("remove member from team: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return 0, nil, ErrMemberNotFound
	}

	remaining, err := memberCount(ctx, tx, team.ID)
	if err != nil {
		return 0, nil, err
	}
	if remaining == 0 {
		if _, err := tx.Exec(ctx, `DELETE FROM teams WHERE id = $1`, team.ID); err != nil {
			return 0, nil, fmt.Errorf("delete empty team: %w", err)
		}
		return 0, nil, nil
	}

	var newLeaderID *string
	if team.LeaderID != nil && *team.LeaderID == userID {
		if err := tx.QueryRow(ctx, `
			UPDATE teams SET updated_at = CURRENT_TIMESTAMP, leader_id = (
				SELECT tm.user_id FROM team_members tm
				JOIN users u ON u.id = tm.user_id
				WHERE tm.team_id = $1 AND u.deleted_at IS NULL
				ORDER BY tm.joined_at ASC, tm.user_id ASC LIMIT 1
			) WHERE id = $1
			RETURNING leader_id`, team.ID).Scan(&newLeaderID); err != nil {
			return 0, nil, fmt.Errorf("transfer team leadership: %w", err)
		}
	}
	return remaining, newLeaderID, nil
}

// LeaveTeam removes userID from their team in compID and returns the team id they left.
func (r *Repository) LeaveTeam(ctx context.Context, compID, userID string) (string, error) {
	if compID == "" {
		compID = DefaultCompetitionID
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin leave tx: %w", err)
	}
	defer tx.Rollback(ctx)

	current, err := lockParticipant(ctx, tx, compID, userID)
	if err != nil {
		return "", err
	}
	if current == nil {
		return "", ErrNotInTeam
	}
	team, err := lockTeam(ctx, tx, "competition_id = $1 AND id = $2", compID, *current)
	if err != nil {
		return "", err
	}
	settings, err := r.loadSettingsForComp(ctx, tx, compID)
	if err != nil {
		return "", err
	}
	if team.effectiveFrozen(settings) {
		return "", ErrTeamFrozen
	}
	if _, _, err := removeMemberTx(ctx, tx, team, userID); err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit leave tx: %w", err)
	}
	return team.ID, nil
}

// leaderTeamTx locks the caller's team and verifies they lead it.
func (r *Repository) leaderTeamTx(ctx context.Context, tx pgx.Tx, compID, userID string) (*teamLock, SettingsResponse, error) {
	var zero SettingsResponse
	if compID == "" {
		compID = DefaultCompetitionID
	}
	current, err := lockParticipant(ctx, tx, compID, userID)
	if err != nil {
		return nil, zero, err
	}
	if current == nil {
		return nil, zero, ErrNotInTeam
	}
	team, err := lockTeam(ctx, tx, "competition_id = $1 AND id = $2", compID, *current)
	if err != nil {
		return nil, zero, err
	}
	if team.LeaderID == nil || *team.LeaderID != userID {
		return nil, zero, ErrNotLeader
	}
	settings, err := r.loadSettingsForComp(ctx, tx, compID)
	if err != nil {
		return nil, zero, err
	}
	return team, settings, nil
}

// RenameOwnTeam renames the team led by userID.
func (r *Repository) RenameOwnTeam(ctx context.Context, compID, userID, name string) (string, error) {
	if compID == "" {
		compID = DefaultCompetitionID
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin rename tx: %w", err)
	}
	defer tx.Rollback(ctx)

	team, settings, err := r.leaderTeamTx(ctx, tx, compID, userID)
	if err != nil {
		return "", err
	}
	if team.effectiveFrozen(settings) {
		return "", ErrTeamFrozen
	}
	if _, err := tx.Exec(ctx,
		`UPDATE teams SET name = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`,
		strings.TrimSpace(name), team.ID); err != nil {
		if isConstraint(err, "teams_name_key") || isConstraint(err, "teams_competition_id_name_key") {
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
func (r *Repository) RegenerateOwnCode(ctx context.Context, compID, userID string) (string, error) {
	if compID == "" {
		compID = DefaultCompetitionID
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin regenerate tx: %w", err)
	}
	defer tx.Rollback(ctx)

	team, _, err := r.leaderTeamTx(ctx, tx, compID, userID)
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
		if !isConstraint(err, "teams_join_code_key") && !isConstraint(err, "teams_competition_id_join_code_key") {
			return fmt.Errorf("regenerate join code: %w", err)
		}
	}
	return errors.New("could not generate a unique join code")
}

// KickMember lets the leader remove another member from their team.
func (r *Repository) KickMember(ctx context.Context, compID, leaderID, targetID string) (string, error) {
	if compID == "" {
		compID = DefaultCompetitionID
	}
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
	team, settings, err := r.leaderTeamTx(ctx, tx, compID, leaderID)
	if err != nil {
		return "", err
	}
	if team.effectiveFrozen(settings) {
		return "", ErrTeamFrozen
	}
	if _, _, err := removeMemberTx(ctx, tx, team, targetID); err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit kick tx: %w", err)
	}
	return team.ID, nil
}

// TransferLeadership transfers leadership of a team to another active member.
func (r *Repository) TransferLeadership(ctx context.Context, compID, currentLeaderID, newLeaderID string) (string, error) {
	if compID == "" {
		compID = DefaultCompetitionID
	}
	if currentLeaderID == newLeaderID {
		return "", nil
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin transfer leadership tx: %w", err)
	}
	defer tx.Rollback(ctx)

	team, _, err := r.leaderTeamTx(ctx, tx, compID, currentLeaderID)
	if err != nil {
		return "", err
	}

	var isMember bool
	err = tx.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM team_members tm JOIN users u ON u.id = tm.user_id WHERE tm.user_id = $1 AND tm.team_id = $2 AND u.deleted_at IS NULL)`,
		newLeaderID, team.ID).Scan(&isMember)
	if err != nil || !isMember {
		return "", ErrLeaderNotMember
	}

	var isBanned bool
	_ = tx.QueryRow(ctx,
		`SELECT COALESCE(ue.is_banned, FALSE)
		 FROM competitions c
		 JOIN user_events ue ON ue.event_id = c.event_id AND ue.user_id = $2
		 WHERE c.id = $1`,
		compID, newLeaderID).Scan(&isBanned)
	if isBanned {
		return "", ErrUserIsBanned
	}

	if _, err := tx.Exec(ctx,
		`UPDATE teams SET leader_id = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`,
		newLeaderID, team.ID); err != nil {
		return "", fmt.Errorf("update team leader: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit transfer leadership: %w", err)
	}
	return team.ID, nil
}

// EvictUserFromTeamTx evicts userID from their team in compID within an existing transaction.
// If the user was leader, leadership is automatically transferred to the earliest-joined remaining member.
// If no members remain, the team is deleted.
func (r *Repository) EvictUserFromTeamTx(ctx context.Context, tx pgx.Tx, compID, userID string) (teamID string, remaining int, newLeaderID *string, err error) {
	if compID == "" {
		compID = DefaultCompetitionID
	}

	var t teamLock
	err = tx.QueryRow(ctx, `
		SELECT t.id, t.name, t.competition_id, t.leader_id, t.is_frozen, t.max_size
		FROM teams t
		JOIN team_members tm ON tm.team_id = t.id
		WHERE tm.user_id = $1 AND t.competition_id = $2
		FOR UPDATE OF t
	`, userID, compID).Scan(&t.ID, &t.Name, &t.CompetitionID, &t.LeaderID, &t.Frozen, &t.MaxSize)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", 0, nil, nil
		}
		return "", 0, nil, fmt.Errorf("find team for eviction: %w", err)
	}

	rem, updatedLeaderID, err := removeMemberTx(ctx, tx, &t, userID)
	if err != nil {
		return "", 0, nil, err
	}

	return t.ID, rem, updatedLeaderID, nil
}

// EvictUserFromTeam evicts userID from their team in compID.
func (r *Repository) EvictUserFromTeam(ctx context.Context, compID, userID string) (string, int, *string, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", 0, nil, err
	}
	defer tx.Rollback(ctx)

	teamID, rem, newLeader, err := r.EvictUserFromTeamTx(ctx, tx, compID, userID)
	if err != nil {
		return "", 0, nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return "", 0, nil, err
	}
	return teamID, rem, newLeader, nil
}

// EvictUserFromEventTeamsTx evicts userID from every team they belong to across all competitions in eventID.
func (r *Repository) EvictUserFromEventTeamsTx(ctx context.Context, tx pgx.Tx, eventID, userID string) ([]EvictedTeamChange, error) {
	rows, err := tx.Query(ctx, `
		SELECT t.id, t.name, t.competition_id, t.leader_id, t.is_frozen, t.max_size
		FROM team_members tm
		JOIN teams t ON t.id = tm.team_id
		JOIN competitions c ON c.id = t.competition_id
		WHERE tm.user_id = $1 AND c.event_id = $2
		FOR UPDATE OF t
	`, userID, eventID)
	if err != nil {
		return nil, err
	}
	var affected []teamLock
	for rows.Next() {
		var tl teamLock
		if err := rows.Scan(&tl.ID, &tl.Name, &tl.CompetitionID, &tl.LeaderID, &tl.Frozen, &tl.MaxSize); err != nil {
			rows.Close()
			return nil, err
		}
		affected = append(affected, tl)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	changes := make([]EvictedTeamChange, 0, len(affected))
	for i := range affected {
		tl := &affected[i]
		rem, newLeaderID, err := removeMemberTx(ctx, tx, tl, userID)
		if err != nil {
			return nil, err
		}
		changes = append(changes, EvictedTeamChange{
			TeamID:        tl.ID,
			TeamName:      tl.Name,
			CompetitionID: tl.CompetitionID,
			EventID:       eventID,
			UserID:        userID,
			Deleted:       rem == 0,
			NewLeaderID:   newLeaderID,
		})
	}
	return changes, nil
}

// EvictUserFromAllTeamsTx evicts userID from all teams across all competitions and events.
func (r *Repository) EvictUserFromAllTeamsTx(ctx context.Context, tx pgx.Tx, userID string) ([]EvictedTeamChange, error) {
	rows, err := tx.Query(ctx, `
		SELECT t.id, t.name, t.competition_id, COALESCE(c.event_id::text, ''), t.leader_id, t.is_frozen, t.max_size
		FROM team_members tm
		JOIN teams t ON t.id = tm.team_id
		LEFT JOIN competitions c ON c.id = t.competition_id
		WHERE tm.user_id = $1
		FOR UPDATE OF t
	`, userID)
	if err != nil {
		return nil, err
	}
	type lockedTeamWithEvent struct {
		lock    teamLock
		eventID string
	}
	var affected []lockedTeamWithEvent
	for rows.Next() {
		var item lockedTeamWithEvent
		if err := rows.Scan(
			&item.lock.ID,
			&item.lock.Name,
			&item.lock.CompetitionID,
			&item.eventID,
			&item.lock.LeaderID,
			&item.lock.Frozen,
			&item.lock.MaxSize,
		); err != nil {
			rows.Close()
			return nil, err
		}
		affected = append(affected, item)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	changes := make([]EvictedTeamChange, 0, len(affected))
	for i := range affected {
		tl := &affected[i].lock
		rem, newLeaderID, err := removeMemberTx(ctx, tx, tl, userID)
		if err != nil {
			return nil, err
		}
		changes = append(changes, EvictedTeamChange{
			TeamID:        tl.ID,
			TeamName:      tl.Name,
			CompetitionID: tl.CompetitionID,
			EventID:       affected[i].eventID,
			UserID:        userID,
			Deleted:       rem == 0,
			NewLeaderID:   newLeaderID,
		})
	}
	return changes, nil
}

// EvictAndAuditEventTeamsTx evicts userID from every team in eventID within tx and returns a post-commit audit callback.
func (r *Repository) EvictAndAuditEventTeamsTx(ctx context.Context, tx pgx.Tx, eventID, userID string, auditor audit.Recorder) (func(), error) {
	changes, err := r.EvictUserFromEventTeamsTx(ctx, tx, eventID, userID)
	if err != nil {
		return nil, err
	}
	return func() {
		r.RecordEvictionAudits(ctx, auditor, changes)
	}, nil
}

// EvictAndAuditAllTeamsTx evicts userID from all teams across all competitions within tx and returns a post-commit audit callback.
func (r *Repository) EvictAndAuditAllTeamsTx(ctx context.Context, tx pgx.Tx, userID string, auditor audit.Recorder) (func(), error) {
	changes, err := r.EvictUserFromAllTeamsTx(ctx, tx, userID)
	if err != nil {
		return nil, err
	}
	return func() {
		r.RecordEvictionAudits(ctx, auditor, changes)
	}, nil
}

// EvictUserFromAnyTeam evicts a user from all teams across all competitions in a standalone transaction.
func (r *Repository) EvictUserFromAnyTeam(ctx context.Context, userID string) ([]EvictedTeamChange, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin evict team tx: %w", err)
	}
	defer tx.Rollback(ctx)

	changes, err := r.EvictUserFromAllTeamsTx(ctx, tx, userID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	r.RecordEvictionAudits(ctx, nil, changes)
	return changes, nil
}

// ---- reads ----

func (r *Repository) GetTeamByID(ctx context.Context, id string) (*TeamDetailResponse, error) {
	var resp TeamDetailResponse
	var leaderID *string
	if compID, hasComp := resolveCompetitionIDFromContext(ctx); hasComp {
		err := r.db.QueryRow(ctx, `
			SELECT id, competition_id, name, join_code, leader_id, is_frozen, max_size, created_at, updated_at
			FROM teams WHERE id = $1 AND competition_id = $2`, id, compID,
		).Scan(&resp.ID, &resp.CompetitionID, &resp.Name, &resp.JoinCode, &leaderID, &resp.IsFrozen, &resp.MaxSize, &resp.CreatedAt, &resp.UpdatedAt)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, ErrTeamNotFound
			}
			return nil, fmt.Errorf("get team by id: %w", err)
		}
	} else {
		err := r.db.QueryRow(ctx, `
			SELECT id, competition_id, name, join_code, leader_id, is_frozen, max_size, created_at, updated_at
			FROM teams WHERE id = $1`, id,
		).Scan(&resp.ID, &resp.CompetitionID, &resp.Name, &resp.JoinCode, &leaderID, &resp.IsFrozen, &resp.MaxSize, &resp.CreatedAt, &resp.UpdatedAt)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, ErrTeamNotFound
			}
			return nil, fmt.Errorf("get team by id: %w", err)
		}
	}

	settings, err := r.loadSettingsForComp(ctx, r.db, resp.CompetitionID)
	if err != nil {
		return nil, err
	}
	resp.EffectiveFrozen = resp.IsFrozen || settings.FrozenAll
	resp.EffectiveMaxSize = settings.MaxTeamSize
	if resp.MaxSize != nil {
		resp.EffectiveMaxSize = *resp.MaxSize
	}

	rows, err := r.db.Query(ctx, `
		SELECT u.id, u.email, u.name, u.phone_number, COALESCE(ue.role::text, 'participant')
		FROM team_members tm
		JOIN users u ON u.id = tm.user_id
		LEFT JOIN competitions c ON c.id = $2
		LEFT JOIN user_events ue ON ue.user_id = u.id AND ue.event_id = c.event_id
		WHERE tm.team_id = $1 AND u.deleted_at IS NULL
		ORDER BY tm.joined_at ASC, u.id ASC`, id, resp.CompetitionID)
	if err != nil {
		return nil, fmt.Errorf("fetch team members: %w", err)
	}
	defer rows.Close()

	resp.Members = make([]TeamMemberSummary, 0)
	for rows.Next() {
		var m TeamMemberSummary
		if err := rows.Scan(&m.ID, &m.Email, &m.Name, &m.PhoneNumber, &m.Role); err != nil {
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

func (r *Repository) GetTeamByUserID(ctx context.Context, compID, userID string) (*TeamDetailResponse, error) {
	if compID == "" {
		compID = DefaultCompetitionID
	}
	var teamID *string
	err := r.db.QueryRow(ctx, `
		SELECT tm.team_id
		FROM team_members tm
		JOIN users u ON u.id = tm.user_id
		WHERE tm.user_id = $1 AND tm.competition_id = $2 AND u.deleted_at IS NULL
	`, userID, compID).Scan(&teamID)
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
	if compID, hasComp := resolveCompetitionIDFromContext(ctx); hasComp {
		err := r.db.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM judging_slots s
				JOIN judging_panels p ON p.id = s.panel_id
				JOIN judging_rounds r ON r.id = p.round_id
				JOIN judging_panel_judges pj ON pj.panel_id = s.panel_id
				WHERE s.team_id = $1 AND pj.judge_id = $2 AND r.competition_id = $3
			)`, teamID, judgeID, compID).Scan(&ok)
		if err != nil {
			return false, fmt.Errorf("check judge team assignment: %w", err)
		}
		return ok, nil
	}
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
	compID, _ := resolveCompetitionIDFromContext(ctx)
	if filter.CompetitionID != nil && *filter.CompetitionID != "" {
		compID = *filter.CompetitionID
	}
	settings, err := r.loadSettingsForComp(ctx, r.db, compID)
	if err != nil {
		return nil, 0, err
	}

	var conditions []string
	var args []any
	argIndex := 1

	if filter.CompetitionID != nil && *filter.CompetitionID != "" {
		conditions = append(conditions, fmt.Sprintf("t.competition_id = $%d", argIndex))
		args = append(args, *filter.CompetitionID)
		argIndex++
	}

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
		SELECT t.id, t.competition_id, t.name, t.join_code, t.leader_id, u.name, u.email,
		       COUNT(m.id) AS member_count, t.is_frozen, t.max_size, t.created_at, t.updated_at
		FROM teams t
		LEFT JOIN users u ON t.leader_id = u.id AND u.deleted_at IS NULL
		LEFT JOIN team_members tm ON tm.team_id = t.id
		LEFT JOIN users m ON m.id = tm.user_id AND m.deleted_at IS NULL
		%s
		GROUP BY t.id, t.competition_id, t.name, t.join_code, t.leader_id, u.name, u.email, t.is_frozen, t.max_size, t.created_at, t.updated_at
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
			&t.ID, &t.CompetitionID, &t.Name, &t.JoinCode, &t.LeaderID, &t.LeaderName, &t.LeaderEmail,
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

	if compID, hasComp := resolveCompetitionIDFromContext(ctx); hasComp {
		if _, err := lockTeam(ctx, tx, "id = $1 AND competition_id = $2", id, compID); err != nil {
			return err
		}
	} else {
		if _, err := lockTeam(ctx, tx, "id = $1", id); err != nil {
			return err
		}
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
			`SELECT EXISTS(SELECT 1 FROM team_members tm JOIN users u ON u.id = tm.user_id WHERE tm.user_id = $1 AND tm.team_id = $2 AND u.deleted_at IS NULL)`,
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
			if isConstraint(err, "teams_name_key") || isConstraint(err, "teams_competition_id_name_key") {
				return ErrTeamAlreadyExists
			}
			return fmt.Errorf("update team: %w", err)
		}
	}
	return tx.Commit(ctx)
}

func (r *Repository) SetFrozen(ctx context.Context, id string, frozen bool) error {
	var (
		tag pgconn.CommandTag
		err error
	)
	if compID, hasComp := resolveCompetitionIDFromContext(ctx); hasComp {
		tag, err = r.db.Exec(ctx,
			`UPDATE teams SET is_frozen = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2 AND competition_id = $3`, frozen, id, compID)
	} else {
		tag, err = r.db.Exec(ctx,
			`UPDATE teams SET is_frozen = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`, frozen, id)
	}
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
	if compID, hasComp := resolveCompetitionIDFromContext(ctx); hasComp {
		if _, err := lockTeam(ctx, tx, "id = $1 AND competition_id = $2", id, compID); err != nil {
			return err
		}
	} else {
		if _, err := lockTeam(ctx, tx, "id = $1", id); err != nil {
			return err
		}
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

	if compID, hasComp := resolveCompetitionIDFromContext(ctx); hasComp {
		if _, err := lockTeam(ctx, tx, "id = $1 AND competition_id = $2", id, compID); err != nil {
			return err
		}
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

// AddMember lets staff place a participant who has no team in compID into teamID.
func (r *Repository) AddMember(ctx context.Context, compID, teamID, userID string) error {
	if compID == "" {
		compID = DefaultCompetitionID
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin add member tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Lock order is always user then team, matching join/leave.
	current, err := lockParticipant(ctx, tx, compID, userID)
	if err != nil {
		return err
	}
	team, err := lockTeam(ctx, tx, "competition_id = $1 AND id = $2", compID, teamID)
	if err != nil {
		return err
	}
	if current != nil {
		return ErrAlreadyInTeam
	}
	settings, err := r.loadSettingsForComp(ctx, tx, compID)
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
		`INSERT INTO team_members (team_id, user_id, competition_id) VALUES ($1, $2, $3)`, team.ID, userID, compID); err != nil {
		return fmt.Errorf("insert team member: %w", err)
	}
	return tx.Commit(ctx)
}

// RemoveMember lets staff remove a member; leadership transfer and empty-team cleanup still apply.
func (r *Repository) RemoveMember(ctx context.Context, compID, teamID, userID string) error {
	if compID == "" {
		compID = DefaultCompetitionID
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin remove member tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := lockUser(ctx, tx, userID); err != nil {
		return err
	}
	team, err := lockTeam(ctx, tx, "competition_id = $1 AND id = $2", compID, teamID)
	if err != nil {
		return err
	}
	if _, _, err := removeMemberTx(ctx, tx, team, userID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
