package judging

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrRoundNotFound      = errors.New("judging round not found")
	ErrPanelNotFound      = errors.New("judging panel not found")
	ErrSlotNotFound       = errors.New("judging slot not found")
	ErrNotPresent         = errors.New("submissions are closed: no member of your team was present at the required attendance session")
	ErrGateSessionInvalid = errors.New("submission gate must be an existing attendance session")
	ErrUnauthorizedSlot   = errors.New("unauthorized: you do not have permission to update links for this slot")
	ErrJudgeNotAssigned   = errors.New("forbidden: judge is not assigned to evaluate this panel")
	ErrSlotNotStarted     = errors.New("pitch has not started; start the pitch before scoring")
	ErrJudgeHasScores     = errors.New("a judge who has already scored in this room can't be removed from it")
	ErrNoJudges           = errors.New("a room needs at least one judge")
	ErrNotJudges          = errors.New("only users with the judge role can be assigned to a panel")
	ErrJudgeAssigned      = errors.New("one or more judges are already assigned to another panel in this round")
	ErrTeamSlotted        = errors.New("one or more teams are already scheduled in this round")
	ErrRoundNotActive     = errors.New("this round is not active; pitches can only start once the round has been activated")
	ErrRoundCompleted     = errors.New("this round has ended, so its time per team can no longer be changed")
	ErrRoundNotRunning    = errors.New("only an active round can be ended")
	ErrPitchesRunning     = errors.New("a team is still presenting; complete that pitch before ending the round")
	ErrRoundIsActive      = errors.New("end the round before deleting it")
	ErrRoundFinished      = errors.New("this round has already finished and cannot be activated")
	ErrPanelBusy          = errors.New("another team is already presenting in this room; complete that pitch first")
	ErrPanelStarted       = errors.New("panel has started or finished pitches; add teams instead of replacing the sequence")
	ErrSlotTimingLocked   = errors.New("finished teams can no longer have their time changed")
	ErrSlotNotMovable     = errors.New("only teams waiting in the queue can be moved; presenting and finished teams stay where they are")
	ErrPitchEnded         = errors.New("pitch has ended; the timer can no longer be changed")
	ErrIncompleteScore    = errors.New("scorecard is incomplete: every criterion must be scored before locking")
	ErrTimerPaused        = errors.New("timer is already paused")
	ErrTimerNotPaused     = errors.New("timer is not paused")
	ErrSlotNotPending     = errors.New("pitch has already started or finished")
	ErrCriterionHasScores = errors.New("judges have already scored this criterion, so it can't be removed")
	ErrRoundHasScores     = errors.New("scores already exist for this round's team criteria; they cannot be re-imported")
	ErrDuplicateTeamRow   = errors.New("each team may appear only once in the import")
	ErrUnknownTeam        = errors.New("one or more teams do not exist")
	ErrAwardedScoreRange  = errors.New("a team's score must be between 0 and its max score")
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// CreateRound creates a new judging round with criteria in a transaction.
func (r *Repository) CreateRound(ctx context.Context, req CreateRoundRequest) (string, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	bufSize := req.BufferSize
	if bufSize <= 0 {
		bufSize = 2
	}
	duration := req.SlotDurationMins
	if duration <= 0 {
		duration = 15
	}

	var roundID string
	roundQuery := `
		INSERT INTO judging_rounds (name, status, buffer_size, slot_duration_mins)
		VALUES ($1, 'draft', $2, $3)
		RETURNING id;
	`
	if err := tx.QueryRow(ctx, roundQuery, req.Name, bufSize, duration).Scan(&roundID); err != nil {
		return "", fmt.Errorf("insert round: %w", err)
	}

	critQuery := `
		INSERT INTO judging_criteria (round_id, name, description, min_score, max_score, weight, order_index)
		VALUES ($1, $2, $3, $4, $5, $6, $7);
	`
	for i, c := range req.Criteria {
		minScore := c.MinScore
		if minScore < 0 {
			minScore = 0
		}
		maxScore := c.MaxScore
		if maxScore <= 0 {
			maxScore = 10
		}
		weight := c.Weight
		if weight <= 0 {
			weight = 1.00
		}
		orderIdx := c.OrderIndex
		if orderIdx == 0 {
			orderIdx = i + 1
		}
		if _, err := tx.Exec(ctx, critQuery, roundID, c.Name, c.Description, minScore, maxScore, weight, orderIdx); err != nil {
			return "", fmt.Errorf("insert criterion %s: %w", c.Name, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit tx: %w", err)
	}

	return roundID, nil
}

// GetRound retrieves round metadata and criteria.
func (r *Repository) GetRound(ctx context.Context, id string) (*CreateRoundRequest, string, int, int, []CriterionOutput, error) {
	roundQuery := `SELECT id, name, status, buffer_size, slot_duration_mins FROM judging_rounds WHERE id = $1;`
	var roundID, name, status string
	var bufferSize, duration int
	if err := r.db.QueryRow(ctx, roundQuery, id).Scan(&roundID, &name, &status, &bufferSize, &duration); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, "", 0, 0, nil, ErrRoundNotFound
		}
		return nil, "", 0, 0, nil, fmt.Errorf("query round: %w", err)
	}

	critQuery := `
		SELECT id, name, COALESCE(description, ''), min_score, max_score, weight, order_index
		FROM judging_criteria
		WHERE round_id = $1 AND team_id IS NULL
		ORDER BY order_index ASC, id ASC;
	`
	rows, err := r.db.Query(ctx, critQuery, id)
	if err != nil {
		return nil, "", 0, 0, nil, fmt.Errorf("query criteria: %w", err)
	}
	defer rows.Close()

	var criteria []CriterionOutput
	for rows.Next() {
		var c CriterionOutput
		if err := rows.Scan(&c.ID, &c.Name, &c.Description, &c.MinScore, &c.MaxScore, &c.Weight, &c.OrderIndex); err != nil {
			return nil, "", 0, 0, nil, fmt.Errorf("scan criterion: %w", err)
		}
		criteria = append(criteria, c)
	}

	req := &CreateRoundRequest{
		Name:             name,
		BufferSize:       bufferSize,
		SlotDurationMins: duration,
	}

	return req, status, bufferSize, duration, criteria, nil
}

// ListRounds returns all judging rounds.
func (r *Repository) ListRounds(ctx context.Context) ([]map[string]any, error) {
	query := `
		SELECT r.id, r.name, r.status, r.buffer_size, r.slot_duration_mins, r.created_at,
		       COUNT(DISTINCT p.id) as panel_count,
		       COUNT(DISTINCT s.id) as slot_count
		FROM judging_rounds r
		LEFT JOIN judging_panels p ON p.round_id = r.id
		LEFT JOIN judging_slots s ON s.panel_id = p.id
		GROUP BY r.id
		ORDER BY r.created_at DESC;
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list rounds: %w", err)
	}
	defer rows.Close()

	result := make([]map[string]any, 0)
	for rows.Next() {
		var id, name, status string
		var bufferSize, duration, panelCount, slotCount int
		var createdAt time.Time
		if err := rows.Scan(&id, &name, &status, &bufferSize, &duration, &createdAt, &panelCount, &slotCount); err != nil {
			return nil, fmt.Errorf("scan round: %w", err)
		}
		result = append(result, map[string]any{
			"id":                 id,
			"name":               name,
			"status":             status,
			"buffer_size":        bufferSize,
			"slot_duration_mins": duration,
			"created_at":         createdAt,
			"panel_count":        panelCount,
			"slot_count":         slotCount,
		})
	}
	return result, nil
}

// ActivateRound opens a draft round and stages the first buffer_size teams of each room
// as on-deck, so volunteers and participants see who to call up immediately.
func (r *Repository) ActivateRound(ctx context.Context, roundID string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var status string
	var bufferSize int
	if err := tx.QueryRow(ctx,
		`SELECT status::text, buffer_size FROM judging_rounds WHERE id = $1 FOR UPDATE`, roundID,
	).Scan(&status, &bufferSize); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrRoundNotFound
		}
		return fmt.Errorf("fetch round: %w", err)
	}
	switch status {
	case "active":
		return nil
	case "draft":
	default:
		return ErrRoundFinished
	}

	if _, err := tx.Exec(ctx,
		`UPDATE judging_rounds SET status = 'active', updated_at = CURRENT_TIMESTAMP WHERE id = $1`, roundID,
	); err != nil {
		return fmt.Errorf("activate round: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE judging_slots s
		SET status = 'queued', updated_at = CURRENT_TIMESTAMP
		FROM judging_panels p
		WHERE s.panel_id = p.id AND p.round_id = $1
		  AND s.status = 'scheduled' AND s.sequence_order <= $2`, roundID, bufferSize,
	); err != nil {
		return fmt.Errorf("stage first teams: %w", err)
	}
	return tx.Commit(ctx)
}

// EndRound closes an active round. Rooms, teams and scores are kept; the leaderboard stays available.
// With force, pitches still running are closed (completed now, scores not yet locked don't count);
// without it the end is refused while a team is presenting.
func (r *Repository) EndRound(ctx context.Context, roundID string, force bool) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var status string
	if err := tx.QueryRow(ctx,
		`SELECT status::text FROM judging_rounds WHERE id = $1 FOR UPDATE`, roundID,
	).Scan(&status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrRoundNotFound
		}
		return fmt.Errorf("fetch round: %w", err)
	}
	if status != "active" {
		return ErrRoundNotRunning
	}

	var presenting int
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(*) FROM judging_slots s
		JOIN judging_panels p ON p.id = s.panel_id
		WHERE p.round_id = $1 AND s.status = 'in_progress'`, roundID).Scan(&presenting); err != nil {
		return fmt.Errorf("check running pitches: %w", err)
	}
	if presenting > 0 {
		if !force {
			return ErrPitchesRunning
		}
		if _, err := tx.Exec(ctx, `
			UPDATE judging_slots s
			SET status = 'completed',
			    actual_end_time = CURRENT_TIMESTAMP,
			    pitch_ended_at = COALESCE(s.pitch_ended_at, s.paused_at, CURRENT_TIMESTAMP),
			    updated_at = CURRENT_TIMESTAMP
			FROM judging_panels p
			WHERE s.panel_id = p.id AND p.round_id = $1 AND s.status = 'in_progress'`, roundID); err != nil {
			return fmt.Errorf("close running pitches: %w", err)
		}
	}

	if _, err := tx.Exec(ctx,
		`UPDATE judging_rounds SET status = 'completed', updated_at = CURRENT_TIMESTAMP WHERE id = $1`, roundID,
	); err != nil {
		return fmt.Errorf("end round: %w", err)
	}
	return tx.Commit(ctx)
}

// DeleteRound permanently removes a round with its criteria, rooms, slots and scores.
// An active round is only deleted when force is set.
func (r *Repository) DeleteRound(ctx context.Context, roundID string, force bool) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var status string
	if err := tx.QueryRow(ctx,
		`SELECT status::text FROM judging_rounds WHERE id = $1 FOR UPDATE`, roundID,
	).Scan(&status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrRoundNotFound
		}
		return fmt.Errorf("fetch round: %w", err)
	}
	if status == "active" && !force {
		return ErrRoundIsActive
	}

	if _, err := tx.Exec(ctx, `DELETE FROM judging_rounds WHERE id = $1`, roundID); err != nil {
		return fmt.Errorf("delete round: %w", err)
	}
	return tx.Commit(ctx)
}

// UpdateRoundStatus updates the status of a judging round.
func (r *Repository) UpdateRoundStatus(ctx context.Context, roundID, status string) error {
	query := `UPDATE judging_rounds SET status = $2::judging_round_status, updated_at = CURRENT_TIMESTAMP WHERE id = $1;`
	res, err := r.db.Exec(ctx, query, roundID, status)
	if err != nil {
		return fmt.Errorf("update round status: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrRoundNotFound
	}
	return nil
}

// CreatePanel creates a panel in a room and assigns judges.
func (r *Repository) CreatePanel(ctx context.Context, req CreatePanelRequest) (string, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var panelID string
	query := `
		INSERT INTO judging_panels (round_id, name, room)
		VALUES ($1, $2, $3)
		RETURNING id;
	`
	if err := tx.QueryRow(ctx, query, req.RoundID, req.Name, req.Room).Scan(&panelID); err != nil {
		return "", fmt.Errorf("insert panel: %w", err)
	}

	// Panels are staffed by judges only.
	var judges int
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(DISTINCT id) FROM users
		WHERE id = ANY($1::uuid[]) AND role = 'judge' AND deleted_at IS NULL`,
		req.JudgeIDs).Scan(&judges); err != nil {
		return "", fmt.Errorf("verify judges: %w", err)
	}
	distinct := make(map[string]struct{}, len(req.JudgeIDs))
	for _, id := range req.JudgeIDs {
		distinct[id] = struct{}{}
	}
	if judges != len(distinct) {
		return "", ErrNotJudges
	}

	// A judge can only sit on one panel per round.
	var conflicts int
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(*) FROM judging_panel_judges pj
		JOIN judging_panels p ON p.id = pj.panel_id
		WHERE p.round_id = $1 AND p.id <> $2 AND pj.judge_id = ANY($3::uuid[])`,
		req.RoundID, panelID, req.JudgeIDs).Scan(&conflicts); err != nil {
		return "", fmt.Errorf("check judge assignments: %w", err)
	}
	if conflicts > 0 {
		return "", ErrJudgeAssigned
	}

	judgeQuery := `INSERT INTO judging_panel_judges (panel_id, judge_id) VALUES ($1, $2) ON CONFLICT DO NOTHING;`
	for _, jID := range req.JudgeIDs {
		if _, err := tx.Exec(ctx, judgeQuery, panelID, jID); err != nil {
			return "", fmt.Errorf("assign judge %s: %w", jID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit tx: %w", err)
	}
	return panelID, nil
}

// SetPanelJudges replaces the judges seated in a room. Judges must have the judge role, can sit on only
// one room per round, and can't be removed once they've scored a team in this room.
func (r *Repository) SetPanelJudges(ctx context.Context, panelID string, judgeIDs []string) (added, removed []string, err error) {
	distinct := make(map[string]struct{}, len(judgeIDs))
	unique := make([]string, 0, len(judgeIDs))
	for _, id := range judgeIDs {
		if _, dup := distinct[id]; !dup {
			distinct[id] = struct{}{}
			unique = append(unique, id)
		}
	}
	if len(unique) == 0 {
		return nil, nil, ErrNoJudges
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var roundID string
	if err := tx.QueryRow(ctx, `SELECT round_id FROM judging_panels WHERE id = $1 FOR UPDATE;`, panelID).Scan(&roundID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, ErrPanelNotFound
		}
		return nil, nil, fmt.Errorf("fetch panel: %w", err)
	}

	var judges int
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(DISTINCT id) FROM users
		WHERE id = ANY($1::uuid[]) AND role = 'judge' AND deleted_at IS NULL`, unique).Scan(&judges); err != nil {
		return nil, nil, fmt.Errorf("verify judges: %w", err)
	}
	if judges != len(unique) {
		return nil, nil, ErrNotJudges
	}

	var conflicts int
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(*) FROM judging_panel_judges pj
		JOIN judging_panels p ON p.id = pj.panel_id
		WHERE p.round_id = $1 AND p.id <> $2 AND pj.judge_id = ANY($3::uuid[])`,
		roundID, panelID, unique).Scan(&conflicts); err != nil {
		return nil, nil, fmt.Errorf("check judge assignments: %w", err)
	}
	if conflicts > 0 {
		return nil, nil, ErrJudgeAssigned
	}

	rows, err := tx.Query(ctx, `SELECT judge_id::text FROM judging_panel_judges WHERE panel_id = $1;`, panelID)
	if err != nil {
		return nil, nil, fmt.Errorf("query current judges: %w", err)
	}
	current := map[string]struct{}{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, nil, err
		}
		current[id] = struct{}{}
	}
	rows.Close()

	for id := range current {
		if _, keep := distinct[id]; !keep {
			removed = append(removed, id)
		}
	}
	for _, id := range unique {
		if _, had := current[id]; !had {
			added = append(added, id)
		}
	}

	if len(removed) > 0 {
		var scored int
		if err := tx.QueryRow(ctx, `
			SELECT COUNT(*) FROM judging_scores sc
			JOIN judging_slots s ON s.id = sc.slot_id
			WHERE s.panel_id = $1 AND sc.judge_id = ANY($2::uuid[])`, panelID, removed).Scan(&scored); err != nil {
			return nil, nil, fmt.Errorf("check judge scores: %w", err)
		}
		if scored > 0 {
			return nil, nil, ErrJudgeHasScores
		}
		if _, err := tx.Exec(ctx, `DELETE FROM judging_panel_judges WHERE panel_id = $1 AND judge_id = ANY($2::uuid[]);`, panelID, removed); err != nil {
			return nil, nil, fmt.Errorf("remove judges: %w", err)
		}
	}
	for _, id := range added {
		if _, err := tx.Exec(ctx, `INSERT INTO judging_panel_judges (panel_id, judge_id) VALUES ($1, $2) ON CONFLICT DO NOTHING;`, panelID, id); err != nil {
			return nil, nil, fmt.Errorf("assign judge %s: %w", id, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, fmt.Errorf("commit tx: %w", err)
	}
	return added, removed, nil
}

// GetRoundPanels lists a round's panels with judge counts and the teams each already holds.
func (r *Repository) GetRoundPanels(ctx context.Context, roundID string) ([]RoundPanelOverview, error) {
	rows, err := r.db.Query(ctx, `
		SELECT p.id, p.name, p.room,
			COALESCE((SELECT array_agg(pj.judge_id::text ORDER BY pj.judge_id) FROM judging_panel_judges pj WHERE pj.panel_id = p.id), '{}'),
			COALESCE((SELECT array_agg(s.team_id::text ORDER BY s.sequence_order)
			          FROM judging_slots s WHERE s.panel_id = p.id), '{}')
		FROM judging_panels p
		WHERE p.round_id = $1
		ORDER BY p.room ASC, p.name ASC`, roundID)
	if err != nil {
		return nil, fmt.Errorf("query round panels: %w", err)
	}
	defer rows.Close()

	out := make([]RoundPanelOverview, 0)
	for rows.Next() {
		var o RoundPanelOverview
		if err := rows.Scan(&o.PanelID, &o.Name, &o.Room, &o.JudgeIDs, &o.TeamIDs); err != nil {
			return nil, fmt.Errorf("scan round panel: %w", err)
		}
		o.JudgeCount = len(o.JudgeIDs)
		o.SlotCount = len(o.TeamIDs)
		o.Slots = []RoundPanelSlot{}
		out = append(out, o)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	slotRows, err := r.db.Query(ctx, `
		SELECT s.panel_id, s.id, s.team_id, t.name, s.sequence_order, s.status::text,
			   COALESCE(s.duration_mins, r.slot_duration_mins), s.duration_mins IS NOT NULL, s.scheduled_start_time, s.estimated_start_time
		FROM judging_slots s
		JOIN judging_panels p ON p.id = s.panel_id
		JOIN judging_rounds r ON r.id = p.round_id
		JOIN teams t ON t.id = s.team_id
		WHERE p.round_id = $1
		ORDER BY s.sequence_order ASC`, roundID)
	if err != nil {
		return nil, fmt.Errorf("query round panel slots: %w", err)
	}
	defer slotRows.Close()

	byPanel := make(map[string]int, len(out))
	for i, o := range out {
		byPanel[o.PanelID] = i
	}
	for slotRows.Next() {
		var panelID string
		var sl RoundPanelSlot
		if err := slotRows.Scan(&panelID, &sl.SlotID, &sl.TeamID, &sl.TeamName, &sl.SequenceOrder, &sl.Status,
			&sl.DurationMins, &sl.CustomDuration, &sl.ScheduledStartTime, &sl.EstimatedStartTime); err != nil {
			return nil, fmt.Errorf("scan round panel slot: %w", err)
		}
		if i, ok := byPanel[panelID]; ok {
			out[i].Slots = append(out[i].Slots, sl)
		}
	}
	return out, slotRows.Err()
}

// GetAssignedJudges lists judges already seated on a panel in the round, with that panel.
func (r *Repository) GetAssignedJudges(ctx context.Context, roundID string) ([]AssignedJudge, error) {
	rows, err := r.db.Query(ctx, `
		SELECT pj.judge_id, p.id, p.name
		FROM judging_panel_judges pj
		JOIN judging_panels p ON p.id = pj.panel_id
		WHERE p.round_id = $1
		ORDER BY p.name ASC`, roundID)
	if err != nil {
		return nil, fmt.Errorf("query assigned judges: %w", err)
	}
	defer rows.Close()

	out := make([]AssignedJudge, 0)
	for rows.Next() {
		var a AssignedJudge
		if err := rows.Scan(&a.JudgeID, &a.PanelID, &a.PanelName); err != nil {
			return nil, fmt.Errorf("scan assigned judge: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// ConfigureSlots assigns sequential slots for teams in a panel.
// With appendMode the slots are added after the existing ones; otherwise the sequence is replaced.
func (r *Repository) ConfigureSlots(ctx context.Context, panelID string, slots []SlotAssignment, appendMode bool) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Fetch panel round duration
	var durationMins int
	var roundID string
	err = tx.QueryRow(ctx, `
		SELECT r.slot_duration_mins, r.id
		FROM judging_panels p 
		JOIN judging_rounds r ON r.id = p.round_id 
		WHERE p.id = $1
		FOR UPDATE OF p;
	`, panelID).Scan(&durationMins, &roundID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrPanelNotFound
		}
		return fmt.Errorf("fetch panel round: %w", err)
	}

	teamIDs := make([]string, len(slots))
	for i, s := range slots {
		teamIDs[i] = s.TeamID
	}

	// Any team can be scheduled. One that wasn't in the CSV import joins the round's roster, so it
	// shows up alongside the imported teams and can be given custom-criterion marks.
	if _, err := tx.Exec(ctx, `
		INSERT INTO judging_round_teams (round_id, team_id)
		SELECT $1, t.id FROM teams t WHERE t.id = ANY($2::uuid[])
		ON CONFLICT DO NOTHING`, roundID, teamIDs); err != nil {
		return fmt.Errorf("add teams to roster: %w", err)
	}

	offset := 0
	if appendMode {
		// A team plays once per round: reject any already scheduled in any panel of it.
		var taken int
		if err := tx.QueryRow(ctx, `
			SELECT COUNT(*) FROM judging_slots s
			JOIN judging_panels p ON p.id = s.panel_id
			WHERE p.round_id = $1 AND s.team_id = ANY($2::uuid[])`, roundID, teamIDs).Scan(&taken); err != nil {
			return fmt.Errorf("check scheduled teams: %w", err)
		}
		if taken > 0 {
			return ErrTeamSlotted
		}
		if err := tx.QueryRow(ctx,
			`SELECT COALESCE(MAX(sequence_order), 0) FROM judging_slots WHERE panel_id = $1`, panelID,
		).Scan(&offset); err != nil {
			return fmt.Errorf("next sequence: %w", err)
		}
	} else {
		// Replacing would wipe live or finished pitches (and their scores).
		var started int
		if err := tx.QueryRow(ctx, `
			SELECT COUNT(*) FROM judging_slots
			WHERE panel_id = $1 AND status IN ('in_progress', 'completed')`, panelID).Scan(&started); err != nil {
			return fmt.Errorf("check started slots: %w", err)
		}
		if started > 0 {
			return ErrPanelStarted
		}
		// Clear existing slots for re-slotting
		if _, err := tx.Exec(ctx, `DELETE FROM judging_slots WHERE panel_id = $1;`, panelID); err != nil {
			return fmt.Errorf("clear slots: %w", err)
		}
	}

	slotQuery := `
		INSERT INTO judging_slots (panel_id, team_id, sequence_order, status, estimated_start_time, submission_links)
		VALUES ($1, $2, $3, 'scheduled', $4, $5);
	`
	now := time.Now()
	for i, s := range slots {
		if appendMode {
			s.SequenceOrder = offset + i + 1
		}
		estTime := now.Add(time.Duration((s.SequenceOrder-1)*durationMins) * time.Minute)
		linksJSON, _ := json.Marshal(s.SubmissionLinks)
		if len(s.SubmissionLinks) == 0 {
			linksJSON = []byte("{}")
		}

		if _, err := tx.Exec(ctx, slotQuery, panelID, s.TeamID, s.SequenceOrder, estTime, linksJSON); err != nil {
			return fmt.Errorf("insert slot team %s: %w", s.TeamID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	r.seedSlotsFromTeams(ctx, panelID)
	return nil
}

// seedSlotsFromTeams copies each team's saved submission onto its new empty slots so judges see
// links submitted before the slot existed. Best effort: the slots are already committed, and a
// team re-saving its submission mirrors it again.
func (r *Repository) seedSlotsFromTeams(ctx context.Context, panelID string) {
	_, _ = r.db.Exec(ctx, `
		UPDATE judging_slots s
		SET submission_links = t.submission_links ||
			CASE WHEN t.project_name IS NULL THEN '{}'::jsonb ELSE jsonb_build_object('project_name', t.project_name) END
		FROM teams t
		WHERE s.panel_id = $1 AND t.id = s.team_id AND s.submission_links = '{}'::jsonb`, panelID)
}

// UpdateSlotLinks modifies submission artifacts for a slot.
func (r *Repository) UpdateSlotLinks(ctx context.Context, slotID string, links map[string]string) error {
	linksJSON, err := json.Marshal(links)
	if err != nil {
		return fmt.Errorf("marshal links: %w", err)
	}

	query := `UPDATE judging_slots SET submission_links = $2, updated_at = CURRENT_TIMESTAMP WHERE id = $1;`
	res, err := r.db.Exec(ctx, query, slotID, linksJSON)
	if err != nil {
		return fmt.Errorf("update slot links: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrSlotNotFound
	}
	return nil
}

// SlotExists verifies if a judging slot exists.
func (r *Repository) SlotExists(ctx context.Context, slotID string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM judging_slots WHERE id = $1);`
	err := r.db.QueryRow(ctx, query, slotID).Scan(&exists)
	return exists, err
}

// CanUserUpdateSlotLinks verifies if the user belongs to the team assigned to slotID.
func (r *Repository) CanUserUpdateSlotLinks(ctx context.Context, slotID, userID string) (bool, error) {
	var allowed bool
	query := `
		SELECT EXISTS (
			SELECT 1 FROM judging_slots s
			JOIN users u ON u.team_id = s.team_id
			WHERE s.id = $1 AND u.id = $2
		);
	`
	err := r.db.QueryRow(ctx, query, slotID, userID).Scan(&allowed)
	return allowed, err
}

// IsJudgeAssignedToSlot checks if judgeID is mapped to the panel evaluating slotID.
func (r *Repository) IsJudgeAssignedToSlot(ctx context.Context, slotID, judgeID string) (bool, error) {
	var assigned bool
	query := `
		SELECT EXISTS (
			SELECT 1 FROM judging_slots s
			JOIN judging_panel_judges pj ON pj.panel_id = s.panel_id
			WHERE s.id = $1 AND pj.judge_id = $2
		);
	`
	err := r.db.QueryRow(ctx, query, slotID, judgeID).Scan(&assigned)
	return assigned, err
}

// GetJudgePanels returns panels where judgeID is an assigned evaluator.
func (r *Repository) GetJudgePanels(ctx context.Context, judgeID string) ([]JudgePanelSummary, error) {
	query := `
		SELECT 
			p.id, p.name, p.room,
			r.id, r.name, r.status::text, r.slot_duration_mins, r.buffer_size
		FROM judging_panels p
		JOIN judging_rounds r ON r.id = p.round_id
		JOIN judging_panel_judges pj ON pj.panel_id = p.id
		WHERE pj.judge_id = $1 AND r.status IN ('active', 'completed')
		ORDER BY r.created_at DESC, p.name ASC;
	`
	rows, err := r.db.Query(ctx, query, judgeID)
	if err != nil {
		return nil, fmt.Errorf("query judge panels: %w", err)
	}
	defer rows.Close()

	panels := make([]JudgePanelSummary, 0)
	for rows.Next() {
		var p JudgePanelSummary
		if err := rows.Scan(&p.PanelID, &p.PanelName, &p.Room, &p.RoundID, &p.RoundName, &p.RoundStatus, &p.SlotDurationMins, &p.BufferSize); err != nil {
			return nil, fmt.Errorf("scan judge panel: %w", err)
		}
		panels = append(panels, p)
	}

	// For each panel, fetch criteria and slots
	for i := range panels {
		crit, err := r.GetRoundCriteria(ctx, panels[i].RoundID)
		if err != nil {
			return nil, err
		}
		panels[i].Criteria = crit

		slots, err := r.GetPanelSlotsForJudge(ctx, panels[i].PanelID, judgeID)
		if err != nil {
			return nil, err
		}
		panels[i].Slots = slots
		panels[i].TotalSlots = len(slots)

		completed := 0
		for _, s := range slots {
			if s.Status == "completed" {
				completed++
			}
			if s.Status == "in_progress" && panels[i].CurrentSlot == nil {
				copyS := s
				panels[i].CurrentSlot = &copyS
			}
		}
		panels[i].CompletedSlots = completed
	}

	return panels, nil
}

// GetRoundCriteria fetches criteria for a round.
func (r *Repository) GetRoundCriteria(ctx context.Context, roundID string) ([]CriterionOutput, error) {
	query := `
		SELECT id, name, COALESCE(description, ''), min_score, max_score, weight, order_index
		FROM judging_criteria
		WHERE round_id = $1 AND team_id IS NULL
		ORDER BY order_index ASC;
	`
	rows, err := r.db.Query(ctx, query, roundID)
	if err != nil {
		return nil, fmt.Errorf("query criteria: %w", err)
	}
	defer rows.Close()

	var list []CriterionOutput
	for rows.Next() {
		var c CriterionOutput
		if err := rows.Scan(&c.ID, &c.Name, &c.Description, &c.MinScore, &c.MaxScore, &c.Weight, &c.OrderIndex); err != nil {
			return nil, fmt.Errorf("scan criterion: %w", err)
		}
		list = append(list, c)
	}
	return list, nil
}

// GetPanelSlotsForJudge gets slot list with the judge's personal lock status.
func (r *Repository) GetPanelSlotsForJudge(ctx context.Context, panelID string, judgeID string) ([]JudgeSlotSummary, error) {
	query := `
		SELECT 
			s.id, s.team_id, tm.name, s.sequence_order, s.status::text,
			s.estimated_start_time, s.actual_start_time, s.actual_end_time,
			COALESCE(bool_and(sc.is_locked), false) as my_locked
		FROM judging_slots s
		JOIN teams tm ON tm.id = s.team_id
		LEFT JOIN judging_scores sc ON sc.slot_id = s.id AND sc.judge_id = $2
		WHERE s.panel_id = $1
		GROUP BY s.id, tm.name
		ORDER BY s.sequence_order ASC;
	`
	rows, err := r.db.Query(ctx, query, panelID, judgeID)
	if err != nil {
		return nil, fmt.Errorf("query panel slots for judge: %w", err)
	}
	defer rows.Close()

	var slots []JudgeSlotSummary
	for rows.Next() {
		var s JudgeSlotSummary
		if err := rows.Scan(&s.SlotID, &s.TeamID, &s.TeamName, &s.SequenceOrder, &s.Status, &s.EstimatedStartTime, &s.ActualStartTime, &s.ActualEndTime, &s.IsMyScorecardLocked); err != nil {
			return nil, fmt.Errorf("scan slot for judge: %w", err)
		}
		slots = append(slots, s)
	}

	return slots, nil
}

// GetJudgeSlotDetails retrieves full evaluation view for a single slot, including timer info.
func (r *Repository) GetJudgeSlotDetails(ctx context.Context, slotID string, judgeID string) (*JudgeSlotDetails, error) {
	query := `
		SELECT 
			s.id, s.panel_id, p.name, p.room, s.team_id, tm.name, s.sequence_order, s.status::text,
			COALESCE(s.duration_mins, r.slot_duration_mins), s.actual_start_time, s.submission_links,
			s.paused_at, s.paused_total_ms, s.extra_secs, s.pitch_ended_at, CURRENT_TIMESTAMP
		FROM judging_slots s
		JOIN judging_panels p ON p.id = s.panel_id
		JOIN judging_rounds r ON r.id = p.round_id
		JOIN teams tm ON tm.id = s.team_id
		WHERE s.id = $1;
	`
	var d JudgeSlotDetails
	var linksJSON []byte
	var pausedTotalMs int64
	err := r.db.QueryRow(ctx, query, slotID).Scan(
		&d.SlotID, &d.PanelID, &d.PanelName, &d.Room, &d.TeamID, &d.TeamName, &d.SequenceOrder, &d.Status,
		&d.SlotDurationMins, &d.ActualStartTime, &linksJSON,
		&d.PausedAt, &pausedTotalMs, &d.ExtraSeconds, &d.PitchEndedAt, &d.ServerTime,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSlotNotFound
		}
		return nil, fmt.Errorf("query slot details: %w", err)
	}

	d.SlotDurationSeconds = d.SlotDurationMins * 60
	d.PitchEnded = d.PitchEndedAt != nil
	d.IsPaused = d.PausedAt != nil && !d.PitchEnded
	if d.ActualStartTime != nil && d.Status == "in_progress" {
		allowed := time.Duration(d.SlotDurationSeconds+d.ExtraSeconds) * time.Second
		// Time spent paused so far doesn't count against the pitch.
		paused := time.Duration(pausedTotalMs) * time.Millisecond
		// The clock stops at whichever came first: the pitch ending or the current pause.
		clockNow := d.ServerTime
		if d.PitchEndedAt != nil {
			clockNow = *d.PitchEndedAt
		} else if d.PausedAt != nil {
			clockNow = *d.PausedAt
		}
		remaining := int((allowed - (clockNow.Sub(*d.ActualStartTime) - paused)).Seconds())
		d.RemainingSeconds = &remaining
		if !d.IsPaused && !d.PitchEnded {
			targetEnd := d.ServerTime.Add(time.Duration(remaining) * time.Second)
			d.TargetEndTime = &targetEnd
		}
	}

	d.SubmissionLinks = make(map[string]string)
	if len(linksJSON) > 0 {
		_ = json.Unmarshal(linksJSON, &d.SubmissionLinks)
	}

	// Fetch criteria scores for this judge
	scoresQuery := `
		SELECT 
			c.id, c.name, c.min_score, c.max_score, c.weight, sc.score, sc.notes, COALESCE(sc.is_locked, false)
		FROM judging_criteria c
		JOIN judging_panels p ON p.round_id = c.round_id
		JOIN judging_slots s ON s.panel_id = p.id
		LEFT JOIN judging_scores sc ON sc.slot_id = s.id AND sc.criterion_id = c.id AND sc.judge_id = $2
		WHERE s.id = $1 AND c.awarded_score IS NULL AND (c.team_id IS NULL OR c.team_id = s.team_id)
		ORDER BY c.order_index ASC;
	`
	rows, err := r.db.Query(ctx, scoresQuery, slotID, judgeID)
	if err != nil {
		return nil, fmt.Errorf("query criteria scores: %w", err)
	}
	defer rows.Close()

	allMyLocked := true
	hasAnyScore := false
	for rows.Next() {
		var c CriterionScoreOutput
		var locked bool
		if err := rows.Scan(&c.CriterionID, &c.CriterionName, &c.MinScore, &c.MaxScore, &c.Weight, &c.Score, &c.Notes, &locked); err != nil {
			return nil, fmt.Errorf("scan criteria score: %w", err)
		}
		if c.Score != nil {
			hasAnyScore = true
			if c.MaxScore > c.MinScore {
				norm := ((*c.Score - float64(c.MinScore)) / float64(c.MaxScore-c.MinScore)) * c.Weight
				c.NormalizedScore = &norm
			} else {
				norm := c.Weight
				c.NormalizedScore = &norm
			}
		}
		if !locked {
			allMyLocked = false
		}
		d.CriteriaScores = append(d.CriteriaScores, c)
	}
	d.IsMyScorecardLocked = hasAnyScore && allMyLocked

	// Per-judge lock progress for the panel
	judges, err := r.GetPanelLockProgress(ctx, slotID)
	if err != nil {
		return nil, err
	}
	d.PanelJudges = judges
	d.TotalJudgesCount = len(judges)
	for _, j := range judges {
		if j.IsLocked {
			d.LockedJudgesCount++
		}
	}
	d.AllPanelJudgesLocked = d.TotalJudgesCount > 0 && d.LockedJudgesCount == d.TotalJudgesCount

	return &d, nil
}

// StartSlot starts a slot pitch, sets actual_start_time, and marks the next buffer_size slots as 'queued'.
func (r *Repository) StartSlot(ctx context.Context, slotID string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Fetch slot panel and sequence order
	var panelID, status string
	var seqOrder, bufferSize int
	query := `
		SELECT s.panel_id, s.sequence_order, r.buffer_size, s.status::text, r.status::text
		FROM judging_slots s
		JOIN judging_panels p ON p.id = s.panel_id
		JOIN judging_rounds r ON r.id = p.round_id
		WHERE s.id = $1
		FOR UPDATE OF s;
	`
	var roundStatus string
	if err := tx.QueryRow(ctx, query, slotID).Scan(&panelID, &seqOrder, &bufferSize, &status, &roundStatus); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrSlotNotFound
		}
		return fmt.Errorf("fetch slot: %w", err)
	}

	// Only a pending slot can start; restarting would reset the running timer.
	if status != "scheduled" && status != "queued" {
		return ErrSlotNotPending
	}
	if roundStatus != "active" {
		return ErrRoundNotActive
	}

	// A room runs one pitch at a time.
	var presenting int
	if err := tx.QueryRow(ctx,
		`SELECT COUNT(*) FROM judging_slots WHERE panel_id = $1 AND status = 'in_progress'`, panelID,
	).Scan(&presenting); err != nil {
		return fmt.Errorf("check room is free: %w", err)
	}
	if presenting > 0 {
		return ErrPanelBusy
	}

	// Set slot to in_progress
	updateCurrent := `
		UPDATE judging_slots
		SET status = 'in_progress', actual_start_time = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		WHERE id = $1;
	`
	if _, err := tx.Exec(ctx, updateCurrent, slotID); err != nil {
		return fmt.Errorf("set in_progress: %w", err)
	}

	// Promote next buffer_size slots to 'queued'
	promoteQuery := `
		UPDATE judging_slots
		SET status = 'queued', updated_at = CURRENT_TIMESTAMP
		WHERE panel_id = $1 AND sequence_order > $2 AND sequence_order <= ($2 + $3) AND status = 'scheduled';
	`
	if _, err := tx.Exec(ctx, promoteQuery, panelID, seqOrder, bufferSize); err != nil {
		return fmt.Errorf("promote to queued: %w", err)
	}

	return tx.Commit(ctx)
}

// lockInProgressSlot row-locks the slot and returns its pause timestamp, or ErrSlotNotStarted
// unless the pitch is running.
// Timer changes are also rejected once the pitch has ended (a judge locked their scores).
func lockInProgressSlot(ctx context.Context, tx pgx.Tx, slotID string) (pausedAt *time.Time, err error) {
	pausedAt, endedAt, err := lockRunningSlot(ctx, tx, slotID)
	if err != nil {
		return nil, err
	}
	if endedAt != nil {
		return nil, ErrPitchEnded
	}
	return pausedAt, nil
}

// lockRunningSlot row-locks the slot and returns its pause and pitch-end timestamps,
// or ErrSlotNotStarted unless the slot is in progress.
func lockRunningSlot(ctx context.Context, tx pgx.Tx, slotID string) (pausedAt, endedAt *time.Time, err error) {
	var status string
	err = tx.QueryRow(ctx,
		`SELECT status::text, paused_at, pitch_ended_at FROM judging_slots WHERE id = $1 FOR UPDATE`, slotID,
	).Scan(&status, &pausedAt, &endedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, ErrSlotNotFound
		}
		return nil, nil, fmt.Errorf("lock slot: %w", err)
	}
	if status != "in_progress" {
		return nil, nil, ErrSlotNotStarted
	}
	return pausedAt, endedAt, nil
}

// PauseTimer freezes the pitch clock. Fails if the pitch isn't running or is already paused.
func (r *Repository) PauseTimer(ctx context.Context, slotID string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	pausedAt, err := lockInProgressSlot(ctx, tx, slotID)
	if err != nil {
		return err
	}
	if pausedAt != nil {
		return ErrTimerPaused
	}
	if _, err := tx.Exec(ctx,
		`UPDATE judging_slots SET paused_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE id = $1`, slotID,
	); err != nil {
		return fmt.Errorf("pause timer: %w", err)
	}
	return tx.Commit(ctx)
}

// ResumeTimer restarts a paused clock, banking the paused interval so it isn't counted as pitch time.
func (r *Repository) ResumeTimer(ctx context.Context, slotID string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	pausedAt, err := lockInProgressSlot(ctx, tx, slotID)
	if err != nil {
		return err
	}
	if pausedAt == nil {
		return ErrTimerNotPaused
	}
	if _, err := tx.Exec(ctx, `
		UPDATE judging_slots
		SET paused_total_ms = paused_total_ms + (EXTRACT(EPOCH FROM (CURRENT_TIMESTAMP - paused_at)) * 1000)::bigint,
		    paused_at = NULL, updated_at = CURRENT_TIMESTAMP
		WHERE id = $1`, slotID,
	); err != nil {
		return fmt.Errorf("resume timer: %w", err)
	}
	return tx.Commit(ctx)
}

// ExtendTimer adds seconds to a running (or paused) pitch.
func (r *Repository) ExtendTimer(ctx context.Context, slotID string, seconds int) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := lockInProgressSlot(ctx, tx, slotID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE judging_slots SET extra_secs = extra_secs + $2, updated_at = CURRENT_TIMESTAMP WHERE id = $1`, slotID, seconds,
	); err != nil {
		return fmt.Errorf("extend timer: %w", err)
	}
	return tx.Commit(ctx)
}

// requireSlotInProgress returns ErrSlotNotStarted unless the slot's pitch is currently running.
func requireSlotInProgress(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, slotID string) error {
	var status string
	if err := q.QueryRow(ctx, `SELECT status::text FROM judging_slots WHERE id = $1`, slotID).Scan(&status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrSlotNotFound
		}
		return fmt.Errorf("check slot status: %w", err)
	}
	if status != "in_progress" {
		return ErrSlotNotStarted
	}
	return nil
}

// UpsertScores records criteria scores for a judge on a slot. Fails if already locked.
func (r *Repository) UpsertScores(ctx context.Context, slotID string, judgeID string, scores []CriterionScoreInput) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, _, err := lockRunningSlot(ctx, tx, slotID); err != nil {
		return err
	}

	// Check if already locked
	var lockedCount int
	checkQuery := `SELECT COUNT(*) FROM judging_scores WHERE slot_id = $1 AND judge_id = $2 AND is_locked = true;`
	if err := tx.QueryRow(ctx, checkQuery, slotID, judgeID).Scan(&lockedCount); err != nil {
		return fmt.Errorf("check locked: %w", err)
	}
	if lockedCount > 0 {
		return errors.New("scorecard is locked and cannot be edited")
	}

	// Fetch criteria bounds for this slot to validate criterion ownership and score ranges
	critBoundsQuery := `
		SELECT c.id, c.name, c.min_score, c.max_score
		FROM judging_criteria c
		JOIN judging_panels p ON p.round_id = c.round_id
		JOIN judging_slots s ON s.panel_id = p.id
		WHERE s.id = $1 AND c.awarded_score IS NULL AND (c.team_id IS NULL OR c.team_id = s.team_id);
	`
	bRows, err := tx.Query(ctx, critBoundsQuery, slotID)
	if err != nil {
		return fmt.Errorf("fetch criteria bounds: %w", err)
	}
	defer bRows.Close()

	type bound struct {
		name     string
		minScore int
		maxScore int
	}
	bounds := make(map[string]bound)
	for bRows.Next() {
		var id string
		var b bound
		if err := bRows.Scan(&id, &b.name, &b.minScore, &b.maxScore); err != nil {
			return fmt.Errorf("scan criteria bound: %w", err)
		}
		bounds[id] = b
	}
	bRows.Close()
	if err := bRows.Err(); err != nil {
		return fmt.Errorf("iterate criteria bounds: %w", err)
	}

	seenCriteria := make(map[string]struct{}, len(scores))
	for _, sc := range scores {
		if _, dup := seenCriteria[sc.CriterionID]; dup {
			return fmt.Errorf("%w: %s", ErrDuplicateCriterion, sc.CriterionID)
		}
		seenCriteria[sc.CriterionID] = struct{}{}

		b, ok := bounds[sc.CriterionID]
		if !ok {
			return fmt.Errorf("%w: %s", ErrCriterionNotInRound, sc.CriterionID)
		}
		if math.IsNaN(sc.Score) || math.IsInf(sc.Score, 0) || sc.Score < float64(b.minScore) || sc.Score > float64(b.maxScore) {
			return fmt.Errorf("%w: criterion '%s' expects score between %d and %d, got %.2f", ErrScoreOutOfRange, b.name, b.minScore, b.maxScore, sc.Score)
		}
	}

	if err := recordScoreChangesTx(ctx, tx, slotID, judgeID, scores); err != nil {
		return err
	}

	upsertQuery := `
		INSERT INTO judging_scores (slot_id, judge_id, criterion_id, score, notes, is_locked)
		VALUES ($1, $2, $3, $4, $5, false)
		ON CONFLICT (slot_id, judge_id, criterion_id)
		DO UPDATE SET score = EXCLUDED.score, notes = EXCLUDED.notes, updated_at = CURRENT_TIMESTAMP
		WHERE judging_scores.is_locked = false;
	`
	for _, sc := range scores {
		cmdTag, err := tx.Exec(ctx, upsertQuery, slotID, judgeID, sc.CriterionID, sc.Score, sc.Notes)
		if err != nil {
			return fmt.Errorf("upsert score %s: %w", sc.CriterionID, err)
		}
		if cmdTag.RowsAffected() == 0 {
			return errors.New("scorecard is locked and cannot be edited")
		}
	}

	return tx.Commit(ctx)
}

// LockJudgeScorecard locks the judge's score for a slot.
func (r *Repository) LockJudgeScorecard(ctx context.Context, slotID string, judgeID string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, _, err := lockRunningSlot(ctx, tx, slotID); err != nil {
		return err
	}

	// Remove any stray foreign-round score rows before evaluating completeness or locking.
	if _, err := tx.Exec(ctx, `
		DELETE FROM judging_scores sc
		WHERE sc.slot_id = $1 AND sc.judge_id = $2
		  AND NOT EXISTS (
			SELECT 1
			FROM judging_slots s
			JOIN judging_panels p ON p.id = s.panel_id
			JOIN judging_criteria c ON c.round_id = p.round_id
			WHERE s.id = sc.slot_id AND c.id = sc.criterion_id
			  AND c.awarded_score IS NULL AND (c.team_id IS NULL OR c.team_id = s.team_id)
		  )`, slotID, judgeID); err != nil {
		return fmt.Errorf("clean foreign score rows: %w", err)
	}

	// Every criterion in the slot's round must have a valid score within [min_score, max_score] before locking.
	var required, scored int
	if err := tx.QueryRow(ctx, `
		SELECT
			COUNT(c.id) AS required,
			COUNT(sc.id) FILTER (
				WHERE sc.score IS NOT NULL
				  AND sc.score >= c.min_score
				  AND sc.score <= c.max_score
			) AS scored
		FROM judging_slots s
		JOIN judging_panels p ON p.id = s.panel_id
		JOIN judging_criteria c ON c.round_id = p.round_id AND c.awarded_score IS NULL AND (c.team_id IS NULL OR c.team_id = s.team_id)
		LEFT JOIN judging_scores sc
			ON sc.slot_id = s.id
		   AND sc.judge_id = $2
		   AND sc.criterion_id = c.id
		WHERE s.id = $1`,
		slotID, judgeID).Scan(&required, &scored); err != nil {
		return fmt.Errorf("count scored criteria: %w", err)
	}
	if scored == 0 {
		return errors.New("no scores found to lock; please submit criteria scores before locking")
	}
	if scored < required {
		return fmt.Errorf("%w (%d of %d scored)", ErrIncompleteScore, scored, required)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE judging_scores sc
		SET is_locked = true, locked_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		FROM judging_slots s
		JOIN judging_panels p ON p.id = s.panel_id
		JOIN judging_criteria c ON c.round_id = p.round_id
		WHERE sc.slot_id = $1
		  AND sc.judge_id = $2
		  AND s.id = sc.slot_id
		  AND c.id = sc.criterion_id
		  AND c.awarded_score IS NULL AND (c.team_id IS NULL OR c.team_id = s.team_id)`, slotID, judgeID); err != nil {
		return fmt.Errorf("lock scorecard: %w", err)
	}

	// The first lock ends the pitch for the whole panel: the clock stops where it was
	// (frozen at the pause point if currently paused).
	if _, err := tx.Exec(ctx, `
		UPDATE judging_slots
		SET pitch_ended_at = COALESCE(paused_at, CURRENT_TIMESTAMP), updated_at = CURRENT_TIMESTAMP
		WHERE id = $1 AND pitch_ended_at IS NULL`, slotID); err != nil {
		return fmt.Errorf("end pitch: %w", err)
	}

	if err := recordLockMetricsTx(ctx, tx, slotID, judgeID); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// GetPanelLockProgress lists each judge on the slot's panel and whether they have locked
// every criterion. Only lock state is returned, never scores.
func (r *Repository) GetPanelLockProgress(ctx context.Context, slotID string) ([]PanelJudgeProgress, error) {
	rows, err := r.db.Query(ctx, `
		SELECT u.id, u.name,
			(SELECT COUNT(*)
			 FROM judging_scores sc
			 JOIN judging_criteria c ON c.id = sc.criterion_id AND c.round_id = p.round_id
			      AND c.awarded_score IS NULL AND (c.team_id IS NULL OR c.team_id = s.team_id)
			 WHERE sc.slot_id = s.id AND sc.judge_id = u.id AND sc.is_locked = true)
			>= (SELECT COUNT(*)
			    FROM judging_criteria c
			    WHERE c.round_id = p.round_id AND c.awarded_score IS NULL AND (c.team_id IS NULL OR c.team_id = s.team_id))
		FROM judging_slots s
		JOIN judging_panels p ON p.id = s.panel_id
		JOIN judging_panel_judges pj ON pj.panel_id = s.panel_id
		JOIN users u ON u.id = pj.judge_id
		WHERE s.id = $1
		ORDER BY u.name ASC`, slotID)
	if err != nil {
		return nil, fmt.Errorf("query panel lock progress: %w", err)
	}
	defer rows.Close()

	judges := make([]PanelJudgeProgress, 0)
	for rows.Next() {
		var j PanelJudgeProgress
		if err := rows.Scan(&j.JudgeID, &j.Name, &j.IsLocked); err != nil {
			return nil, fmt.Errorf("scan panel judge: %w", err)
		}
		judges = append(judges, j)
	}
	return judges, rows.Err()
}

// CheckAllPanelJudgesLocked checks if every judge assigned to the panel has locked scores for the slot.
func (r *Repository) CheckAllPanelJudgesLocked(ctx context.Context, slotID string) (bool, error) {
	query := `
		WITH panel_judges AS (
			SELECT pj.judge_id
			FROM judging_slots s
			JOIN judging_panel_judges pj ON pj.panel_id = s.panel_id
			WHERE s.id = $1
		),
		criteria_count AS (
			SELECT COUNT(*) AS n
			FROM judging_criteria c
			JOIN judging_panels p ON p.round_id = c.round_id
			JOIN judging_slots s ON s.panel_id = p.id
			WHERE s.id = $1 AND c.awarded_score IS NULL AND (c.team_id IS NULL OR c.team_id = s.team_id)
		),
		locked_judges AS (
			SELECT sc.judge_id
			FROM judging_scores sc
			JOIN judging_slots s ON s.id = sc.slot_id
			JOIN judging_panels p ON p.id = s.panel_id
			JOIN judging_criteria c ON c.id = sc.criterion_id AND c.round_id = p.round_id
			     AND c.awarded_score IS NULL AND (c.team_id IS NULL OR c.team_id = s.team_id)
			WHERE sc.slot_id = $1 AND sc.is_locked = true
			GROUP BY sc.judge_id
			HAVING COUNT(DISTINCT c.id) >= (SELECT n FROM criteria_count)
		)
		SELECT 
			(SELECT COUNT(*) FROM panel_judges) AS total_judges,
			(SELECT COUNT(*) FROM panel_judges pj WHERE pj.judge_id IN (SELECT judge_id FROM locked_judges)) AS locked_judges;
	`
	var totalJudges, lockedJudges int
	if err := r.db.QueryRow(ctx, query, slotID).Scan(&totalJudges, &lockedJudges); err != nil {
		return false, fmt.Errorf("query locked status: %w", err)
	}
	if totalJudges == 0 {
		return false, errors.New("no judges assigned to this panel")
	}
	return totalJudges == lockedJudges, nil
}

// CompleteSlot marks slot completed, shifts queue, and recalibrates remaining slot timings.
func (r *Repository) CompleteSlot(ctx context.Context, slotID string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var panelID, status string
	var seqOrder, bufferSize, durationMins int
	query := `
		SELECT s.panel_id, s.sequence_order, r.buffer_size, r.slot_duration_mins, s.status::text
		FROM judging_slots s
		JOIN judging_panels p ON p.id = s.panel_id
		JOIN judging_rounds r ON r.id = p.round_id
		WHERE s.id = $1
		FOR UPDATE OF s;
	`
	if err := tx.QueryRow(ctx, query, slotID).Scan(&panelID, &seqOrder, &bufferSize, &durationMins, &status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrSlotNotFound
		}
		return fmt.Errorf("fetch slot details: %w", err)
	}
	// Only a running pitch can be completed; this also makes a double-complete a no-op error.
	if status != "in_progress" {
		return ErrSlotNotStarted
	}

	// Complete this slot
	completeQuery := `
		UPDATE judging_slots
		SET status = 'completed', actual_end_time = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		WHERE id = $1;
	`
	if _, err := tx.Exec(ctx, completeQuery, slotID); err != nil {
		return fmt.Errorf("mark complete: %w", err)
	}

	// Promote next buffer_size slots to queued
	promoteQuery := `
		UPDATE judging_slots
		SET status = 'queued', updated_at = CURRENT_TIMESTAMP
		WHERE panel_id = $1 AND sequence_order > $2 AND sequence_order <= ($2 + $3) AND status = 'scheduled';
	`
	if _, err := tx.Exec(ctx, promoteQuery, panelID, seqOrder, bufferSize); err != nil {
		return fmt.Errorf("promote next slots: %w", err)
	}

	// Recalibrate estimated start times for remaining slots
	if err := r.recalibratePanelEstimatesTx(ctx, tx, panelID, durationMins); err != nil {
		return fmt.Errorf("recalibrate estimates: %w", err)
	}

	return tx.Commit(ctx)
}

// GetUserTeamID retrieves the active team_id for a user.
func (r *Repository) GetUserTeamID(ctx context.Context, userID string) (*string, error) {
	var teamID *string
	err := r.db.QueryRow(ctx, "SELECT team_id FROM users WHERE id = $1 AND deleted_at IS NULL;", userID).Scan(&teamID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get user team id: %w", err)
	}
	return teamID, nil
}

// GetTeamActiveStatus fetches the team's room, queue index, and estimated start time.
func (r *Repository) GetTeamActiveStatus(ctx context.Context, teamID string) (*TeamStatusResponse, error) {
	query := `
		SELECT 
			r.name, p.name, p.room, s.status::text, s.sequence_order,
			s.estimated_start_time, COALESCE(s.duration_mins, r.slot_duration_mins), s.submission_links, s.id::text,
			(
				SELECT COUNT(*) + 1 
				FROM judging_slots prev 
				WHERE prev.panel_id = s.panel_id 
				  AND prev.status IN ('queued', 'in_progress', 'scheduled') 
				  AND prev.sequence_order < s.sequence_order
			) as queue_pos
		FROM judging_slots s
		JOIN judging_panels p ON p.id = s.panel_id
		JOIN judging_rounds r ON r.id = p.round_id
		WHERE s.team_id = $1 AND r.status = 'active'
		ORDER BY s.created_at DESC
		LIMIT 1;
	`
	var resp TeamStatusResponse
	var linksJSON []byte
	err := r.db.QueryRow(ctx, query, teamID).Scan(
		&resp.RoundName, &resp.PanelName, &resp.Room, &resp.Status, &resp.QueuePosition,
		&resp.EstimatedStartTime, &resp.EstimatedDurationMins, &linksJSON, &resp.SlotID, &resp.QueuePosition,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &TeamStatusResponse{HasActiveRound: false}, nil
		}
		return nil, fmt.Errorf("query team status: %w", err)
	}

	resp.HasActiveRound = true
	resp.SubmissionLinks = make(map[string]string)
	if len(linksJSON) > 0 {
		_ = json.Unmarshal(linksJSON, &resp.SubmissionLinks)
	}

	switch resp.Status {
	case "in_progress":
		resp.Instructions = fmt.Sprintf("Your team is currently pitching in Room %s! Good luck!", resp.Room)
	case "queued":
		resp.Instructions = fmt.Sprintf("You are on deck! Please proceed immediately to Room %s waiting area.", resp.Room)
	case "scheduled":
		resp.Instructions = fmt.Sprintf("You are scheduled in Room %s. Please keep an eye on your queue position.", resp.Room)
	case "completed":
		resp.Instructions = "Your evaluation has concluded. Great job!"
	}

	return &resp, nil
}

// GetVolunteerSchedule returns room coordinates, on-deck teams, and upcoming teams for coordinators.
// When judgeID is non-nil, only panels that judge is assigned to are returned.
func (r *Repository) GetVolunteerSchedule(ctx context.Context, judgeID *string) ([]VolunteerPanelSchedule, error) {
	query := `
		SELECT 
			p.id, p.name, p.room,
			r.id, r.name, r.slot_duration_mins, r.buffer_size
		FROM judging_panels p
		JOIN judging_rounds r ON r.id = p.round_id
		WHERE r.status = 'active'
		  AND ($1::uuid IS NULL OR EXISTS (
			SELECT 1 FROM judging_panel_judges pj
			WHERE pj.panel_id = p.id AND pj.judge_id = $1::uuid
		  ))
		ORDER BY p.room ASC, p.name ASC;
	`
	rows, err := r.db.Query(ctx, query, judgeID)
	if err != nil {
		return nil, fmt.Errorf("query volunteer schedule panels: %w", err)
	}
	defer rows.Close()

	panels := make([]VolunteerPanelSchedule, 0)
	for rows.Next() {
		var p VolunteerPanelSchedule
		if err := rows.Scan(&p.PanelID, &p.PanelName, &p.Room, &p.RoundID, &p.RoundName, &p.SlotDurationMins, &p.BufferSize); err != nil {
			return nil, fmt.Errorf("scan panel: %w", err)
		}
		p.QueuedTeams = []TeamSlotSummary{}
		p.UpcomingTeams = []TeamSlotSummary{}
		panels = append(panels, p)
	}
	rows.Close()

	for i := range panels {
		slotQuery := `
			SELECT 
				s.id, s.team_id, tm.name, s.sequence_order, s.status::text,
				s.estimated_start_time, s.actual_start_time, s.submission_links,
				COALESCE(s.duration_mins, $2), s.duration_mins IS NOT NULL, s.scheduled_start_time
			FROM judging_slots s
			JOIN teams tm ON tm.id = s.team_id
			WHERE s.panel_id = $1
			ORDER BY s.sequence_order ASC;
		`
		slotRows, err := r.db.Query(ctx, slotQuery, panels[i].PanelID, panels[i].SlotDurationMins)
		if err != nil {
			return nil, fmt.Errorf("query slots: %w", err)
		}

		for slotRows.Next() {
			var s TeamSlotSummary
			var linksJSON []byte
			if err := slotRows.Scan(&s.SlotID, &s.TeamID, &s.TeamName, &s.SequenceOrder, &s.Status, &s.EstimatedStartTime, &s.ActualStartTime, &linksJSON,
				&s.DurationMins, &s.CustomDuration, &s.ScheduledStartTime); err != nil {
				slotRows.Close()
				return nil, err
			}
			s.SubmissionLinks = make(map[string]string)
			if len(linksJSON) > 0 {
				_ = json.Unmarshal(linksJSON, &s.SubmissionLinks)
			}

			panels[i].TotalCount++
			switch s.Status {
			case "completed":
				panels[i].CompletedCount++
			case "in_progress":
				copyS := s
				panels[i].ActiveTeam = &copyS
			case "queued":
				panels[i].QueuedTeams = append(panels[i].QueuedTeams, s)
			case "scheduled":
				panels[i].UpcomingTeams = append(panels[i].UpcomingTeams, s)
			}
		}
		slotRows.Close()
	}

	return panels, nil
}

// GetAdminLeaderboard calculates full round rankings with weighted criteria breakdown.
func (r *Repository) GetAdminLeaderboard(ctx context.Context, roundID string) (*AdminLeaderboardResponse, error) {
	// Round info
	var roundName, roundStatus string
	if err := r.db.QueryRow(ctx, `SELECT name, status::text FROM judging_rounds WHERE id = $1;`, roundID).Scan(&roundName, &roundStatus); err != nil {
		return nil, ErrRoundNotFound
	}

	criteria, err := r.GetRoundCriteria(ctx, roundID)
	if err != nil {
		return nil, err
	}

	var maxPossibleScore float64
	for _, c := range criteria {
		maxPossibleScore += c.Weight
	}
	// Per-team criteria share one weight across teams, so each team's ceiling is the same.
	var teamCritWeight float64
	_ = r.db.QueryRow(ctx, `SELECT COALESCE(MAX(weight), 0)::float8 FROM judging_criteria WHERE round_id = $1 AND team_id IS NOT NULL`, roundID).Scan(&teamCritWeight)
	maxPossibleScore += teamCritWeight

	// Leaderboard calculation
	query := `
		WITH panel_judges_count AS (
			SELECT panel_id, COUNT(*) as total_judges
			FROM judging_panel_judges
			GROUP BY panel_id
		),
		criterion_averages AS (
			SELECT 
				s.id as slot_id,
				sc.criterion_id,
				AVG(sc.score) as avg_score,
				COUNT(DISTINCT sc.judge_id) as judges_evaluated
			FROM judging_slots s
			JOIN judging_panels p ON p.id = s.panel_id AND p.round_id = $1
			JOIN judging_scores sc ON sc.slot_id = s.id AND sc.is_locked = true
			JOIN judging_criteria c ON c.id = sc.criterion_id AND c.round_id = p.round_id
			     AND c.awarded_score IS NULL AND (c.team_id IS NULL OR c.team_id = s.team_id)
			GROUP BY s.id, sc.criterion_id
		),
		slot_totals AS (
			SELECT 
				s.id as slot_id,
				s.team_id,
				tm.name as team_name,
				p.name as panel_name,
				p.room,
				s.status::text as slot_status,
				COALESCE(pjc.total_judges, 0) as total_judges_on_panel,
				COALESCE(MAX(ca.judges_evaluated), 0) as judges_evaluated_count,
				COALESCE(SUM(
					CASE 
						WHEN c.max_score > c.min_score 
						THEN ((ca.avg_score - c.min_score)::numeric / (c.max_score - c.min_score)::numeric) * c.weight
						ELSE c.weight
					END
				), 0.00) + COALESCE((
					SELECT ((ac.awarded_score - ac.min_score) / NULLIF(ac.max_score - ac.min_score, 0)::numeric) * ac.weight
					FROM judging_criteria ac
					WHERE ac.round_id = $1 AND ac.team_id = s.team_id AND ac.awarded_score IS NOT NULL
				), 0.00) as total_weighted_score
			FROM judging_slots s
			JOIN judging_panels p ON p.id = s.panel_id AND p.round_id = $1
			JOIN teams tm ON tm.id = s.team_id
			LEFT JOIN panel_judges_count pjc ON pjc.panel_id = p.id
			LEFT JOIN criterion_averages ca ON ca.slot_id = s.id
			LEFT JOIN judging_criteria c ON c.id = ca.criterion_id AND c.round_id = p.round_id
			GROUP BY s.id, s.team_id, tm.name, p.name, p.room, s.status, pjc.total_judges
		)
		SELECT 
			slot_id, team_id, team_name, panel_name, room, slot_status,
			total_judges_on_panel, judges_evaluated_count, total_weighted_score
		FROM slot_totals
		ORDER BY (judges_evaluated_count > 0) DESC, total_weighted_score DESC, team_name ASC;
	`
	rows, err := r.db.Query(ctx, query, roundID)
	if err != nil {
		return nil, fmt.Errorf("calculate leaderboard: %w", err)
	}
	defer rows.Close()

	entries := make([]AdminLeaderboardEntry, 0)
	gradedCount := 0
	// Ranks count only teams with at least one locked scorecard; equal scores share a rank.
	scoredSoFar := 0
	rank := 0
	var prevScore float64
	var totalGradedWeightedScore float64

	type rawEntry struct {
		slotID               string
		teamID               string
		teamName             string
		panelName            string
		room                 string
		slotStatus           string
		totalJudgesOnPanel   int
		judgesEvaluatedCount int
		totalWeightedScore   float64
	}
	var rawList []rawEntry

	for rows.Next() {
		var re rawEntry
		if err := rows.Scan(&re.slotID, &re.teamID, &re.teamName, &re.panelName, &re.room, &re.slotStatus, &re.totalJudgesOnPanel, &re.judgesEvaluatedCount, &re.totalWeightedScore); err != nil {
			return nil, fmt.Errorf("scan leaderboard row: %w", err)
		}
		rawList = append(rawList, re)
	}
	rows.Close()

	for _, re := range rawList {
		// Fetch criteria averages and weighted scores for this slot
		critAverages := make(map[string]float64)
		critWeighted := make(map[string]float64)
		var sumRawAvg float64
		var critCount int

		critRows, err := r.db.Query(ctx, `
			SELECT c.name, c.min_score, c.max_score, c.weight, AVG(sc.score)
			FROM judging_scores sc
			JOIN judging_slots sl ON sl.id = sc.slot_id
			JOIN judging_criteria c ON c.id = sc.criterion_id AND c.round_id = $2
			     AND c.awarded_score IS NULL AND (c.team_id IS NULL OR c.team_id = sl.team_id)
			WHERE sc.slot_id = $1 AND sc.is_locked = true
			GROUP BY c.id, c.name, c.min_score, c.max_score, c.weight;
		`, re.slotID, roundID)
		if err == nil {
			for critRows.Next() {
				var cName string
				var minScore, maxScore int
				var weight, avgSc float64
				if err := critRows.Scan(&cName, &minScore, &maxScore, &weight, &avgSc); err == nil {
					critAverages[cName] = avgSc
					sumRawAvg += avgSc
					critCount++
					var wScore float64
					if maxScore > minScore {
						wScore = ((avgSc - float64(minScore)) / float64(maxScore-minScore)) * weight
					} else {
						wScore = weight
					}
					critWeighted[cName] = wScore
				}
			}
			critRows.Close()
		}

		// Pre-awarded team criterion: shown alongside the judged ones.
		var awName string
		var awMin, awMax int
		var awWeight, awScore float64
		if err := r.db.QueryRow(ctx, `
			SELECT name, min_score, max_score, weight::float8, awarded_score::float8
			FROM judging_criteria
			WHERE round_id = $1 AND team_id = $2 AND awarded_score IS NOT NULL`, roundID, re.teamID,
		).Scan(&awName, &awMin, &awMax, &awWeight, &awScore); err == nil {
			critAverages[awName] = awScore
			if awMax > awMin {
				critWeighted[awName] = ((awScore - float64(awMin)) / float64(awMax-awMin)) * awWeight
			} else {
				critWeighted[awName] = awWeight
			}
		}

		var avgRawScore float64
		if critCount > 0 {
			avgRawScore = sumRawAvg / float64(critCount)
		}

		entryRank := 0
		if re.judgesEvaluatedCount > 0 {
			scoredSoFar++
			if scoredSoFar == 1 || math.Abs(re.totalWeightedScore-prevScore) > 1e-9 {
				rank = scoredSoFar
			}
			prevScore = re.totalWeightedScore
			entryRank = rank
		}

		allLocked := re.totalJudgesOnPanel > 0 && re.judgesEvaluatedCount >= re.totalJudgesOnPanel
		if allLocked {
			gradedCount++
			totalGradedWeightedScore += re.totalWeightedScore
		}

		entries = append(entries, AdminLeaderboardEntry{
			Rank:                   entryRank,
			TeamID:                 re.teamID,
			TeamName:               re.teamName,
			PanelName:              re.panelName,
			Room:                   re.room,
			SlotStatus:             re.slotStatus,
			CriteriaAverages:       critAverages,
			CriteriaWeightedScores: critWeighted,
			TotalWeightedScore:     re.totalWeightedScore,
			MaxPossibleScore:       maxPossibleScore,
			AverageRawScore:        avgRawScore,
			AverageScore:           avgRawScore,
			JudgesEvaluatedCount:   re.judgesEvaluatedCount,
			TotalJudgesOnPanel:     re.totalJudgesOnPanel,
			AllPanelJudgesLocked:   allLocked,
		})
	}

	var roundAverageScore float64
	if gradedCount > 0 {
		roundAverageScore = totalGradedWeightedScore / float64(gradedCount)
	}

	return &AdminLeaderboardResponse{
		RoundID:          roundID,
		RoundName:        roundName,
		Status:           roundStatus,
		Criteria:         criteria,
		Leaderboard:      entries,
		TotalTeams:       len(entries),
		GradedTeams:      gradedCount,
		MaxPossibleScore: maxPossibleScore,
		AverageScore:     roundAverageScore,
	}, nil
}

// recalibratePanelEstimatesTx recalculates estimated start times for scheduled and queued slots.
// Each slot advances the clock by its own duration (falling back to defaultDurationMins), and a slot
// with a pinned scheduled_start_time starts exactly then, with the following teams chaining after it.
func (r *Repository) recalibratePanelEstimatesTx(ctx context.Context, tx pgx.Tx, panelID string, defaultDurationMins int) error {
	rows, err := tx.Query(ctx, `
		SELECT id, COALESCE(duration_mins, $2), scheduled_start_time
		FROM judging_slots
		WHERE panel_id = $1 AND status IN ('scheduled', 'queued')
		ORDER BY sequence_order ASC;
	`, panelID, defaultDurationMins)
	if err != nil {
		return err
	}
	type pendingSlot struct {
		id     string
		mins   int
		pinned *time.Time
	}
	var pending []pendingSlot
	for rows.Next() {
		var s pendingSlot
		if err := rows.Scan(&s.id, &s.mins, &s.pinned); err != nil {
			rows.Close()
			return err
		}
		pending = append(pending, s)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	cursor := time.Now()
	for _, s := range pending {
		est := cursor
		if s.pinned != nil {
			est = *s.pinned
		}
		if _, err := tx.Exec(ctx, `UPDATE judging_slots SET estimated_start_time = $2, updated_at = CURRENT_TIMESTAMP WHERE id = $1;`, s.id, est); err != nil {
			return err
		}
		cursor = est.Add(time.Duration(s.mins) * time.Minute)
	}
	return nil
}

// UpdateRoundSlotDuration changes the round's default time per team and reschedules every room.
// With resetOverrides, waiting teams that had their own custom length go back to the new default.
// It returns how many teams were reset.
func (r *Repository) UpdateRoundSlotDuration(ctx context.Context, roundID string, mins int, resetOverrides bool) (int, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var status string
	if err := tx.QueryRow(ctx, `SELECT status::text FROM judging_rounds WHERE id = $1 FOR UPDATE;`, roundID).Scan(&status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, ErrRoundNotFound
		}
		return 0, fmt.Errorf("fetch round: %w", err)
	}
	if status == "completed" {
		return 0, ErrRoundCompleted
	}

	if _, err := tx.Exec(ctx, `UPDATE judging_rounds SET slot_duration_mins = $2, updated_at = CURRENT_TIMESTAMP WHERE id = $1;`, roundID, mins); err != nil {
		return 0, fmt.Errorf("update round duration: %w", err)
	}

	reset := 0
	if resetOverrides {
		tag, err := tx.Exec(ctx, `
			UPDATE judging_slots s SET duration_mins = NULL, updated_at = CURRENT_TIMESTAMP
			FROM judging_panels p
			WHERE p.id = s.panel_id AND p.round_id = $1
			  AND s.duration_mins IS NOT NULL AND s.status IN ('scheduled', 'queued');`, roundID)
		if err != nil {
			return 0, fmt.Errorf("reset team durations: %w", err)
		}
		reset = int(tag.RowsAffected())
	}

	rows, err := tx.Query(ctx, `SELECT id FROM judging_panels WHERE round_id = $1;`, roundID)
	if err != nil {
		return 0, fmt.Errorf("query panels: %w", err)
	}
	var panelIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		panelIDs = append(panelIDs, id)
	}
	rows.Close()
	for _, id := range panelIDs {
		if err := r.recalibratePanelEstimatesTx(ctx, tx, id, mins); err != nil {
			return 0, fmt.Errorf("recalibrate estimates: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit tx: %w", err)
	}
	return reset, nil
}

// UpdateSlotTiming sets (or clears, when nil) a team's own pitch length and pinned start time,
// then recalculates the panel's estimated start times. Finished teams can no longer be changed.
func (r *Repository) UpdateSlotTiming(ctx context.Context, slotID string, req UpdateSlotTimingRequest) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var panelID, status string
	var defaultMins int
	err = tx.QueryRow(ctx, `
		SELECT s.panel_id, s.status::text, r.slot_duration_mins
		FROM judging_slots s
		JOIN judging_panels p ON p.id = s.panel_id
		JOIN judging_rounds r ON r.id = p.round_id
		WHERE s.id = $1
		FOR UPDATE OF s;
	`, slotID).Scan(&panelID, &status, &defaultMins)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrSlotNotFound
		}
		return fmt.Errorf("fetch slot: %w", err)
	}
	if status == "completed" || status == "skipped" || status == "no_show" {
		return ErrSlotTimingLocked
	}

	if _, err := tx.Exec(ctx, `
		UPDATE judging_slots
		SET duration_mins = $2, scheduled_start_time = $3, updated_at = CURRENT_TIMESTAMP
		WHERE id = $1;
	`, slotID, req.DurationMins, req.StartTime); err != nil {
		return fmt.Errorf("update slot timing: %w", err)
	}

	if err := r.recalibratePanelEstimatesTx(ctx, tx, panelID, defaultMins); err != nil {
		return fmt.Errorf("recalibrate estimates: %w", err)
	}
	return tx.Commit(ctx)
}

// MoveSlot moves a team slot within its current panel or transfers it to a new panel/room,
// optionally overriding status (e.g. to 'queued' to throw into the room queue) and updating sequence order.
// NOTE: Admins can throw teams into the room queue even if it exceeds the round buffer limit.
func (r *Repository) MoveSlot(ctx context.Context, slotID string, req MoveSlotRequest) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// 1. Fetch current slot details
	var sourcePanelID, teamID, currentStatus string
	var currentSeqOrder, durationMins int
	var sourceRoundID string
	query := `
		SELECT s.panel_id, s.team_id, s.sequence_order, s.status::text, p.round_id, r.slot_duration_mins, r.status::text
		FROM judging_slots s
		JOIN judging_panels p ON p.id = s.panel_id
		JOIN judging_rounds r ON r.id = p.round_id
		WHERE s.id = $1
		FOR UPDATE OF s;
	`
	var roundStatus string
	if err := tx.QueryRow(ctx, query, slotID).Scan(&sourcePanelID, &teamID, &currentSeqOrder, &currentStatus, &sourceRoundID, &durationMins, &roundStatus); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrSlotNotFound
		}
		return fmt.Errorf("fetch current slot: %w", err)
	}

	// Only teams still waiting (scheduled or on deck) can be moved; presenting and finished teams are fixed.
	if currentStatus != "scheduled" && currentStatus != "queued" {
		return ErrSlotNotMovable
	}

	targetPanelID := sourcePanelID
	isCrossPanel := false
	if req.TargetPanelID != nil && *req.TargetPanelID != "" && *req.TargetPanelID != sourcePanelID {
		targetPanelID = *req.TargetPanelID
		isCrossPanel = true

		// Verify target panel exists and belongs to same round
		var targetRoundID string
		var targetDuration int
		err := tx.QueryRow(ctx, `
			SELECT p.round_id, r.slot_duration_mins
			FROM judging_panels p
			JOIN judging_rounds r ON r.id = p.round_id
			WHERE p.id = $1;
		`, targetPanelID).Scan(&targetRoundID, &targetDuration)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return errors.New("target panel not found")
			}
			return fmt.Errorf("verify target panel: %w", err)
		}
		if targetRoundID != sourceRoundID {
			return errors.New("cannot move team across different judging rounds")
		}

		durationMins = targetDuration
	}

	// 2. Fetch all slots in target panel (excluding moving slot if already in target panel)
	type slotEntry struct {
		id  string
		seq int
	}
	var targetSlots []slotEntry
	rows, err := tx.Query(ctx, `
		SELECT id, sequence_order
		FROM judging_slots
		WHERE panel_id = $1 AND id != $2
		ORDER BY sequence_order ASC;
	`, targetPanelID, slotID)
	if err != nil {
		return fmt.Errorf("query target panel slots: %w", err)
	}
	for rows.Next() {
		var s slotEntry
		if err := rows.Scan(&s.id, &s.seq); err != nil {
			rows.Close()
			return err
		}
		targetSlots = append(targetSlots, s)
	}
	rows.Close()

	// 3. Determine target sequence order
	targetPos := len(targetSlots) + 1
	if req.TargetSequenceOrder != nil && *req.TargetSequenceOrder >= 1 {
		targetPos = *req.TargetSequenceOrder
		if targetPos > len(targetSlots)+1 {
			targetPos = len(targetSlots) + 1
		}
	}

	// Build new ordered list of slot IDs for target panel
	var newTargetSlotIDs []string
	inserted := false
	for i, s := range targetSlots {
		if (i + 1) == targetPos {
			newTargetSlotIDs = append(newTargetSlotIDs, slotID)
			inserted = true
		}
		newTargetSlotIDs = append(newTargetSlotIDs, s.id)
	}
	if !inserted {
		newTargetSlotIDs = append(newTargetSlotIDs, slotID)
	}

	// 4. Update the moving slot's status if specified
	var newStatus = currentStatus
	if req.Status != nil && *req.Status != "" {
		newStatus = *req.Status
	}
	// Before the round starts nothing is staged: every team stays scheduled wherever it lands.
	if roundStatus == "draft" {
		newStatus = "scheduled"
	}

	// Use temporary high sequence order to avoid UNIQUE (panel_id, sequence_order) violations
	if isCrossPanel {
		if _, err := tx.Exec(ctx, `UPDATE judging_slots SET sequence_order = sequence_order + 10000 WHERE panel_id = $1;`, sourcePanelID); err != nil {
			return fmt.Errorf("offset source panel slots: %w", err)
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE judging_slots SET sequence_order = sequence_order + 10000 WHERE panel_id = $1;`, targetPanelID); err != nil {
		return fmt.Errorf("offset target panel slots: %w", err)
	}

	// Update slot's panel and status. Its order moves to a value no other slot holds (the offsets above
	// can otherwise collide with the target panel's offset slots); renumbering below sets the final order.
	if _, err := tx.Exec(ctx, `
		UPDATE judging_slots
		SET panel_id = $1, status = $2::judging_slot_status, sequence_order = 20000, updated_at = CURRENT_TIMESTAMP
		WHERE id = $3;
	`, targetPanelID, newStatus, slotID); err != nil {
		return fmt.Errorf("update moving slot: %w", err)
	}

	// Renumber target panel slots sequentially
	for idx, sID := range newTargetSlotIDs {
		newSeq := idx + 1
		if _, err := tx.Exec(ctx, `
			UPDATE judging_slots
			SET sequence_order = $1, updated_at = CURRENT_TIMESTAMP
			WHERE id = $2;
		`, newSeq, sID); err != nil {
			return fmt.Errorf("renumber target slot: %w", err)
		}
	}

	// If cross-panel, renumber remaining slots in source panel
	if isCrossPanel {
		sourceRows, err := tx.Query(ctx, `
			SELECT id
			FROM judging_slots
			WHERE panel_id = $1
			ORDER BY sequence_order ASC;
		`, sourcePanelID)
		if err != nil {
			return fmt.Errorf("query remaining source slots: %w", err)
		}
		var sourceSlotIDs []string
		for sourceRows.Next() {
			var id string
			if err := sourceRows.Scan(&id); err == nil {
				sourceSlotIDs = append(sourceSlotIDs, id)
			}
		}
		sourceRows.Close()

		for idx, sID := range sourceSlotIDs {
			newSeq := idx + 1
			if _, err := tx.Exec(ctx, `
				UPDATE judging_slots
				SET sequence_order = $1, updated_at = CURRENT_TIMESTAMP
				WHERE id = $2;
			`, newSeq, sID); err != nil {
				return fmt.Errorf("renumber source slot: %w", err)
			}
		}

		// Recalibrate source panel estimates
		var sourceDuration int
		_ = tx.QueryRow(ctx, `
			SELECT r.slot_duration_mins
			FROM judging_panels p
			JOIN judging_rounds r ON r.id = p.round_id
			WHERE p.id = $1;
		`, sourcePanelID).Scan(&sourceDuration)
		if sourceDuration > 0 {
			if err := r.recalibratePanelEstimatesTx(ctx, tx, sourcePanelID, sourceDuration); err != nil {
				return fmt.Errorf("recalibrate source estimates: %w", err)
			}
		}
	}

	// Recalibrate target panel estimates
	if err := r.recalibratePanelEstimatesTx(ctx, tx, targetPanelID, durationMins); err != nil {
		return fmt.Errorf("recalibrate target estimates: %w", err)
	}

	return tx.Commit(ctx)
}

// QueueSlot immediately throws a team slot into the room queue ('queued' status).
func (r *Repository) QueueSlot(ctx context.Context, slotID string) error {
	status := "queued"
	return r.MoveSlot(ctx, slotID, MoveSlotRequest{Status: &status})
}

// UnqueueSlot reverts a queued slot back to 'scheduled'.
func (r *Repository) UnqueueSlot(ctx context.Context, slotID string) error {
	status := "scheduled"
	return r.MoveSlot(ctx, slotID, MoveSlotRequest{Status: &status})
}

const submissionGateKey = "submission_gate_session"

// GetSubmissionGate returns the gating attendance session, or an empty gate when none is set.
func (r *Repository) GetSubmissionGate(ctx context.Context) (SubmissionGate, error) {
	var gate SubmissionGate
	var raw []byte
	err := r.db.QueryRow(ctx, `SELECT value FROM app_settings WHERE key = $1`, submissionGateKey).Scan(&raw)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return gate, nil
		}
		return gate, fmt.Errorf("get submission gate: %w", err)
	}
	var id string
	if json.Unmarshal(raw, &id) != nil || id == "" {
		return gate, nil
	}
	var name string
	err = r.db.QueryRow(ctx, `SELECT name FROM scan_sessions WHERE id = $1`, id).Scan(&name)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return gate, nil // session was removed; treat as no gate
		}
		return gate, fmt.Errorf("get gate session: %w", err)
	}
	gate.SessionID, gate.SessionName = &id, &name
	return gate, nil
}

// SetSubmissionGate stores the gating attendance session (nil clears it).
func (r *Repository) SetSubmissionGate(ctx context.Context, sessionID *string, updatedBy string) (SubmissionGate, error) {
	id := ""
	if sessionID != nil {
		id = *sessionID
		var kind string
		err := r.db.QueryRow(ctx, `SELECT kind FROM scan_sessions WHERE id = $1`, id).Scan(&kind)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return SubmissionGate{}, ErrGateSessionInvalid
			}
			return SubmissionGate{}, fmt.Errorf("check gate session: %w", err)
		}
		if kind != "attendance" {
			return SubmissionGate{}, ErrGateSessionInvalid
		}
	}
	raw, _ := json.Marshal(id)
	_, err := r.db.Exec(ctx, `
		INSERT INTO app_settings (key, value, updated_by) VALUES ($1, $2, $3)
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_by = EXCLUDED.updated_by, updated_at = CURRENT_TIMESTAMP`,
		submissionGateKey, raw, updatedBy)
	if err != nil {
		return SubmissionGate{}, fmt.Errorf("set submission gate: %w", err)
	}
	return r.GetSubmissionGate(ctx)
}

// TeamPresentInSession reports whether at least one member of the team was scanned in the session.
func (r *Repository) TeamPresentInSession(ctx context.Context, teamID, sessionID string) (bool, error) {
	var present bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM users u
			JOIN tickets t ON t.user_id = u.id
			JOIN scan_events e ON e.ticket_id = t.id AND e.session_id = $2
			WHERE u.team_id = $1 AND u.deleted_at IS NULL
		)`, teamID, sessionID).Scan(&present)
	return present, err
}

// SlotTeamID returns the team that owns a slot.
func (r *Repository) SlotTeamID(ctx context.Context, slotID string) (string, error) {
	var teamID string
	err := r.db.QueryRow(ctx, `SELECT team_id::text FROM judging_slots WHERE id = $1`, slotID).Scan(&teamID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrSlotNotFound
	}
	return teamID, err
}

// GetTeamSubmission returns the team's name, project name and links.
func (r *Repository) GetTeamSubmission(ctx context.Context, teamID string) (name string, project string, links map[string]string, err error) {
	var proj *string
	var raw []byte
	err = r.db.QueryRow(ctx, `SELECT name, project_name, submission_links FROM teams WHERE id = $1`, teamID).Scan(&name, &proj, &raw)
	if err != nil {
		return "", "", nil, fmt.Errorf("get team submission: %w", err)
	}
	if proj != nil {
		project = *proj
	}
	links = map[string]string{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &links)
	}
	return name, project, links, nil
}

// SetTeamSubmission saves the team's submission and mirrors it onto any judging slots it holds,
// so judges and volunteers keep reading links from the slot.
func (r *Repository) SetTeamSubmission(ctx context.Context, teamID, project string, links map[string]string) error {
	linksJSON, err := json.Marshal(links)
	if err != nil {
		return fmt.Errorf("marshal links: %w", err)
	}
	slotLinks := map[string]string{}
	for k, v := range links {
		slotLinks[k] = v
	}
	if project != "" {
		slotLinks["project_name"] = project
	}
	slotJSON, _ := json.Marshal(slotLinks)

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx)
	var projArg *string
	if project != "" {
		projArg = &project
	}
	if _, err := tx.Exec(ctx, `UPDATE teams SET project_name = $2, submission_links = $3 WHERE id = $1`, teamID, projArg, linksJSON); err != nil {
		return fmt.Errorf("update team submission: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE judging_slots SET submission_links = $2, updated_at = CURRENT_TIMESTAMP WHERE team_id = $1`, teamID, slotJSON); err != nil {
		return fmt.Errorf("mirror slot links: %w", err)
	}
	return tx.Commit(ctx)
}

// ListSubmissions lists every team's project name and links for the CSV export.
func (r *Repository) ListSubmissions(ctx context.Context) ([]SubmissionRow, error) {
	rows, err := r.db.Query(ctx, `SELECT name, COALESCE(project_name, ''), submission_links FROM teams ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("query submissions: %w", err)
	}
	defer rows.Close()
	out := []SubmissionRow{}
	for rows.Next() {
		var row SubmissionRow
		var raw []byte
		if err := rows.Scan(&row.TeamName, &row.ProjectName, &raw); err != nil {
			return nil, fmt.Errorf("scan submission: %w", err)
		}
		links := map[string]string{}
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &links)
		}
		row.Repository, row.Demo, row.Slides = links["github"], links["demo"], links["slides"]
		out = append(out, row)
	}
	return out, rows.Err()
}

// ImportTeamCriteria replaces the round's roster and per-team criteria in one transaction.
func (r *Repository) ImportTeamCriteria(ctx context.Context, roundID string, req ImportTeamCriteriaRequest) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var status string
	if err := tx.QueryRow(ctx, `SELECT status::text FROM judging_rounds WHERE id = $1 FOR UPDATE`, roundID).Scan(&status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrRoundNotFound
		}
		return fmt.Errorf("lock round: %w", err)
	}
	if status == "completed" {
		return ErrRoundFinished
	}

	for _, row := range req.Rows {
		if row.Score != nil && (math.IsNaN(*row.Score) || math.IsInf(*row.Score, 0) || *row.Score > float64(row.MaxScore)) {
			return ErrAwardedScoreRange
		}
	}

	seen := make(map[string]struct{}, len(req.Rows))
	teamIDs := make([]string, 0, len(req.Rows))
	for _, row := range req.Rows {
		if _, dup := seen[row.TeamID]; dup {
			return ErrDuplicateTeamRow
		}
		seen[row.TeamID] = struct{}{}
		teamIDs = append(teamIDs, row.TeamID)
	}
	var found int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM teams WHERE id = ANY($1::uuid[])`, teamIDs).Scan(&found); err != nil {
		return fmt.Errorf("verify teams: %w", err)
	}
	if found != len(teamIDs) {
		return ErrUnknownTeam
	}

	var scored int
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(*) FROM judging_scores sc
		JOIN judging_criteria c ON c.id = sc.criterion_id
		WHERE c.round_id = $1 AND c.team_id IS NOT NULL`, roundID).Scan(&scored); err != nil {
		return fmt.Errorf("check scores: %w", err)
	}
	if scored > 0 {
		return ErrRoundHasScores
	}

	if _, err := tx.Exec(ctx, `DELETE FROM judging_criteria WHERE round_id = $1 AND team_id IS NOT NULL`, roundID); err != nil {
		return fmt.Errorf("clear team criteria: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM judging_round_teams WHERE round_id = $1`, roundID); err != nil {
		return fmt.Errorf("clear roster: %w", err)
	}

	var maxOrder int
	if err := tx.QueryRow(ctx, `SELECT COALESCE(MAX(order_index), 0) FROM judging_criteria WHERE round_id = $1`, roundID).Scan(&maxOrder); err != nil {
		return fmt.Errorf("next order: %w", err)
	}

	for _, row := range req.Rows {
		if _, err := tx.Exec(ctx, `INSERT INTO judging_round_teams (round_id, team_id) VALUES ($1, $2)`, roundID, row.TeamID); err != nil {
			return fmt.Errorf("insert roster team: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO judging_criteria (round_id, team_id, name, min_score, max_score, weight, order_index, awarded_score)
			VALUES ($1, $2, $3, 0, $4, $5, $6, $7)`,
			roundID, row.TeamID, row.CriterionName, row.MaxScore, req.Weight, maxOrder+1, row.Score); err != nil {
			return fmt.Errorf("insert team criterion: %w", err)
		}
	}
	return tx.Commit(ctx)
}

// SetTeamCriterion creates or updates one team's custom criterion with marks entered by an admin.
// The criterion is not judged: its mark counts straight into the team's total. The team joins the
// round's roster if it wasn't already on it.
func (r *Repository) SetTeamCriterion(ctx context.Context, roundID, teamID string, req SetTeamCriterionRequest) error {
	if math.IsNaN(req.Score) || math.IsInf(req.Score, 0) || req.Score < 0 || req.Score > float64(req.MaxScore) {
		return ErrAwardedScoreRange
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var status string
	if err := tx.QueryRow(ctx, `SELECT status::text FROM judging_rounds WHERE id = $1 FOR UPDATE`, roundID).Scan(&status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrRoundNotFound
		}
		return fmt.Errorf("lock round: %w", err)
	}
	if status == "completed" {
		return ErrRoundFinished
	}

	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM teams WHERE id = $1)`, teamID).Scan(&exists); err != nil {
		return fmt.Errorf("verify team: %w", err)
	}
	if !exists {
		return ErrUnknownTeam
	}

	var maxOrder int
	if err := tx.QueryRow(ctx, `SELECT COALESCE(MAX(order_index), 0) FROM judging_criteria WHERE round_id = $1`, roundID).Scan(&maxOrder); err != nil {
		return fmt.Errorf("next order: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO judging_round_teams (round_id, team_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, roundID, teamID); err != nil {
		return fmt.Errorf("add team to roster: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO judging_criteria (round_id, team_id, name, min_score, max_score, weight, order_index, awarded_score)
		VALUES ($1, $2, $3, 0, $4, $5, $6, $7)
		ON CONFLICT (round_id, team_id) WHERE team_id IS NOT NULL
		DO UPDATE SET name = EXCLUDED.name, max_score = EXCLUDED.max_score, weight = EXCLUDED.weight,
		              awarded_score = EXCLUDED.awarded_score`,
		roundID, teamID, req.CriterionName, req.MaxScore, req.Weight, maxOrder+1, req.Score); err != nil {
		return fmt.Errorf("upsert team criterion: %w", err)
	}
	return tx.Commit(ctx)
}

// DeleteTeamCriterion removes a team's custom criterion (and its marks). The team stays in the round.
func (r *Repository) DeleteTeamCriterion(ctx context.Context, roundID, teamID string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var status string
	if err := tx.QueryRow(ctx, `SELECT status::text FROM judging_rounds WHERE id = $1 FOR UPDATE`, roundID).Scan(&status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrRoundNotFound
		}
		return fmt.Errorf("lock round: %w", err)
	}
	if status == "completed" {
		return ErrRoundFinished
	}

	var scored int
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(*) FROM judging_scores sc
		JOIN judging_criteria c ON c.id = sc.criterion_id
		WHERE c.round_id = $1 AND c.team_id = $2`, roundID, teamID).Scan(&scored); err != nil {
		return fmt.Errorf("check scores: %w", err)
	}
	if scored > 0 {
		return ErrCriterionHasScores
	}
	if _, err := tx.Exec(ctx, `DELETE FROM judging_criteria WHERE round_id = $1 AND team_id = $2`, roundID, teamID); err != nil {
		return fmt.Errorf("delete team criterion: %w", err)
	}
	return tx.Commit(ctx)
}

// GetRoundTeams lists the round's roster with each team's extra criterion.
func (r *Repository) GetRoundTeams(ctx context.Context, roundID string) ([]RoundTeam, error) {
	rows, err := r.db.Query(ctx, `
		SELECT t.id, t.name, COALESCE(c.name, ''), COALESCE(c.max_score, 0), COALESCE(c.weight, 0)::float8, c.awarded_score::float8
		FROM judging_round_teams rt
		JOIN teams t ON t.id = rt.team_id
		LEFT JOIN judging_criteria c ON c.round_id = rt.round_id AND c.team_id = rt.team_id
		WHERE rt.round_id = $1
		ORDER BY t.name ASC`, roundID)
	if err != nil {
		return nil, fmt.Errorf("query roster: %w", err)
	}
	defer rows.Close()
	out := make([]RoundTeam, 0)
	for rows.Next() {
		var t RoundTeam
		if err := rows.Scan(&t.TeamID, &t.TeamName, &t.CriterionName, &t.MaxScore, &t.Weight, &t.AwardedScore); err != nil {
			return nil, fmt.Errorf("scan roster: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
