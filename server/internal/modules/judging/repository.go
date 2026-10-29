package judging

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrRoundNotFound    = errors.New("judging round not found")
	ErrPanelNotFound    = errors.New("judging panel not found")
	ErrSlotNotFound     = errors.New("judging slot not found")
	ErrUnauthorizedSlot = errors.New("unauthorized: you do not have permission to update links for this slot")
	ErrJudgeNotAssigned = errors.New("forbidden: judge is not assigned to evaluate this panel")
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
		WHERE round_id = $1
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

	var result []map[string]any
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

// ConfigureSlots assigns sequential slots for teams in a panel.
func (r *Repository) ConfigureSlots(ctx context.Context, panelID string, slots []SlotAssignment) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Fetch panel round duration
	var durationMins int
	err = tx.QueryRow(ctx, `
		SELECT r.slot_duration_mins 
		FROM judging_panels p 
		JOIN judging_rounds r ON r.id = p.round_id 
		WHERE p.id = $1;
	`, panelID).Scan(&durationMins)
	if err != nil {
		return fmt.Errorf("fetch panel round: %w", err)
	}

	// Clear existing slots for re-slotting
	if _, err := tx.Exec(ctx, `DELETE FROM judging_slots WHERE panel_id = $1;`, panelID); err != nil {
		return fmt.Errorf("clear slots: %w", err)
	}

	slotQuery := `
		INSERT INTO judging_slots (panel_id, team_id, sequence_order, status, estimated_start_time, submission_links)
		VALUES ($1, $2, $3, 'scheduled', $4, $5);
	`
	now := time.Now()
	for _, s := range slots {
		estTime := now.Add(time.Duration((s.SequenceOrder-1)*durationMins) * time.Minute)
		linksJSON, _ := json.Marshal(s.SubmissionLinks)
		if len(s.SubmissionLinks) == 0 {
			linksJSON = []byte("{}")
		}

		if _, err := tx.Exec(ctx, slotQuery, panelID, s.TeamID, s.SequenceOrder, estTime, linksJSON); err != nil {
			return fmt.Errorf("insert slot team %s: %w", s.TeamID, err)
		}
	}

	return tx.Commit(ctx)
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
		WHERE pj.judge_id = $1
		ORDER BY r.created_at DESC, p.name ASC;
	`
	rows, err := r.db.Query(ctx, query, judgeID)
	if err != nil {
		return nil, fmt.Errorf("query judge panels: %w", err)
	}
	defer rows.Close()

	var panels []JudgePanelSummary
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
		WHERE round_id = $1
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
			r.slot_duration_mins, s.actual_start_time, s.submission_links
		FROM judging_slots s
		JOIN judging_panels p ON p.id = s.panel_id
		JOIN judging_rounds r ON r.id = p.round_id
		JOIN teams tm ON tm.id = s.team_id
		WHERE s.id = $1;
	`
	var d JudgeSlotDetails
	var linksJSON []byte
	err := r.db.QueryRow(ctx, query, slotID).Scan(
		&d.SlotID, &d.PanelID, &d.PanelName, &d.Room, &d.TeamID, &d.TeamName, &d.SequenceOrder, &d.Status,
		&d.SlotDurationMins, &d.ActualStartTime, &linksJSON,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSlotNotFound
		}
		return nil, fmt.Errorf("query slot details: %w", err)
	}

	d.SlotDurationSeconds = d.SlotDurationMins * 60
	if d.ActualStartTime != nil {
		targetEnd := d.ActualStartTime.Add(time.Duration(d.SlotDurationMins) * time.Minute)
		d.TargetEndTime = &targetEnd
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
		WHERE s.id = $1
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

	// Check if all assigned panel judges have locked scores
	allLocked, err := r.CheckAllPanelJudgesLocked(ctx, slotID)
	if err == nil {
		d.AllPanelJudgesLocked = allLocked
	}

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
	var panelID string
	var seqOrder, bufferSize int
	query := `
		SELECT s.panel_id, s.sequence_order, r.buffer_size
		FROM judging_slots s
		JOIN judging_panels p ON p.id = s.panel_id
		JOIN judging_rounds r ON r.id = p.round_id
		WHERE s.id = $1;
	`
	if err := tx.QueryRow(ctx, query, slotID).Scan(&panelID, &seqOrder, &bufferSize); err != nil {
		return fmt.Errorf("fetch slot: %w", err)
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

// UpsertScores records criteria scores for a judge on a slot. Fails if already locked.
func (r *Repository) UpsertScores(ctx context.Context, slotID string, judgeID string, scores []CriterionScoreInput) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Check if already locked
	var lockedCount int
	checkQuery := `SELECT COUNT(*) FROM judging_scores WHERE slot_id = $1 AND judge_id = $2 AND is_locked = true;`
	if err := tx.QueryRow(ctx, checkQuery, slotID, judgeID).Scan(&lockedCount); err != nil {
		return fmt.Errorf("check locked: %w", err)
	}
	if lockedCount > 0 {
		return errors.New("scorecard is locked and cannot be edited")
	}

	// Fetch criteria bounds for this slot to validate score ranges
	critBoundsQuery := `
		SELECT c.id, c.name, c.min_score, c.max_score
		FROM judging_criteria c
		JOIN judging_panels p ON p.round_id = c.round_id
		JOIN judging_slots s ON s.panel_id = p.id
		WHERE s.id = $1;
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

	for _, sc := range scores {
		if b, ok := bounds[sc.CriterionID]; ok {
			if sc.Score < float64(b.minScore) || sc.Score > float64(b.maxScore) {
				return fmt.Errorf("%w: criterion '%s' expects score between %d and %d, got %.2f", ErrScoreOutOfRange, b.name, b.minScore, b.maxScore, sc.Score)
			}
		}
	}

	upsertQuery := `
		INSERT INTO judging_scores (slot_id, judge_id, criterion_id, score, notes, is_locked)
		VALUES ($1, $2, $3, $4, $5, false)
		ON CONFLICT (slot_id, judge_id, criterion_id)
		DO UPDATE SET score = EXCLUDED.score, notes = EXCLUDED.notes, updated_at = CURRENT_TIMESTAMP;
	`
	for _, sc := range scores {
		if _, err := tx.Exec(ctx, upsertQuery, slotID, judgeID, sc.CriterionID, sc.Score, sc.Notes); err != nil {
			return fmt.Errorf("upsert score %s: %w", sc.CriterionID, err)
		}
	}

	return tx.Commit(ctx)
}

// LockJudgeScorecard locks the judge's score for a slot.
func (r *Repository) LockJudgeScorecard(ctx context.Context, slotID string, judgeID string) error {
	query := `
		UPDATE judging_scores
		SET is_locked = true, locked_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		WHERE slot_id = $1 AND judge_id = $2;
	`
	res, err := r.db.Exec(ctx, query, slotID, judgeID)
	if err != nil {
		return fmt.Errorf("lock scorecard: %w", err)
	}
	if res.RowsAffected() == 0 {
		return errors.New("no scores found to lock; please submit criteria scores before locking")
	}
	return nil
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
		locked_judges AS (
			SELECT DISTINCT sc.judge_id
			FROM judging_scores sc
			WHERE sc.slot_id = $1 AND sc.is_locked = true
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

	var panelID string
	var seqOrder, bufferSize, durationMins int
	query := `
		SELECT s.panel_id, s.sequence_order, r.buffer_size, r.slot_duration_mins
		FROM judging_slots s
		JOIN judging_panels p ON p.id = s.panel_id
		JOIN judging_rounds r ON r.id = p.round_id
		WHERE s.id = $1;
	`
	if err := tx.QueryRow(ctx, query, slotID).Scan(&panelID, &seqOrder, &bufferSize, &durationMins); err != nil {
		return fmt.Errorf("fetch slot details: %w", err)
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
	now := time.Now()
	recalQuery := `
		SELECT id, sequence_order
		FROM judging_slots
		WHERE panel_id = $1 AND status IN ('scheduled', 'queued')
		ORDER BY sequence_order ASC;
	`
	rows, err := tx.Query(ctx, recalQuery, panelID)
	if err != nil {
		return fmt.Errorf("query remaining slots: %w", err)
	}
	defer rows.Close()

	type slotOrder struct {
		id string
	}
	var remaining []slotOrder
	for rows.Next() {
		var s slotOrder
		var so int
		if err := rows.Scan(&s.id, &so); err != nil {
			return err
		}
		remaining = append(remaining, s)
	}
	rows.Close()

	updateEstQuery := `UPDATE judging_slots SET estimated_start_time = $2, updated_at = CURRENT_TIMESTAMP WHERE id = $1;`
	for idx, s := range remaining {
		estTime := now.Add(time.Duration(idx*durationMins) * time.Minute)
		if _, err := tx.Exec(ctx, updateEstQuery, s.id, estTime); err != nil {
			return fmt.Errorf("update est time for slot %s: %w", s.id, err)
		}
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
			s.estimated_start_time, r.slot_duration_mins, s.submission_links,
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
		&resp.EstimatedStartTime, &resp.EstimatedDurationMins, &linksJSON, &resp.QueuePosition,
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
func (r *Repository) GetVolunteerSchedule(ctx context.Context) ([]VolunteerPanelSchedule, error) {
	query := `
		SELECT 
			p.id, p.name, p.room,
			r.id, r.name, r.slot_duration_mins, r.buffer_size
		FROM judging_panels p
		JOIN judging_rounds r ON r.id = p.round_id
		WHERE r.status = 'active'
		ORDER BY p.room ASC, p.name ASC;
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query volunteer schedule panels: %w", err)
	}
	defer rows.Close()

	var panels []VolunteerPanelSchedule
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
				s.estimated_start_time, s.actual_start_time, s.submission_links
			FROM judging_slots s
			JOIN teams tm ON tm.id = s.team_id
			WHERE s.panel_id = $1
			ORDER BY s.sequence_order ASC;
		`
		slotRows, err := r.db.Query(ctx, slotQuery, panels[i].PanelID)
		if err != nil {
			return nil, fmt.Errorf("query slots: %w", err)
		}

		for slotRows.Next() {
			var s TeamSlotSummary
			var linksJSON []byte
			if err := slotRows.Scan(&s.SlotID, &s.TeamID, &s.TeamName, &s.SequenceOrder, &s.Status, &s.EstimatedStartTime, &s.ActualStartTime, &linksJSON); err != nil {
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
			JOIN judging_scores sc ON sc.slot_id = s.id AND sc.is_locked = true
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
				), 0.00) as total_weighted_score
			FROM judging_slots s
			JOIN judging_panels p ON p.id = s.panel_id AND p.round_id = $1
			JOIN teams tm ON tm.id = s.team_id
			LEFT JOIN panel_judges_count pjc ON pjc.panel_id = p.id
			LEFT JOIN criterion_averages ca ON ca.slot_id = s.id
			LEFT JOIN judging_criteria c ON c.id = ca.criterion_id
			GROUP BY s.id, s.team_id, tm.name, p.name, p.room, s.status, pjc.total_judges
		)
		SELECT 
			slot_id, team_id, team_name, panel_name, room, slot_status,
			total_judges_on_panel, judges_evaluated_count, total_weighted_score
		FROM slot_totals
		ORDER BY total_weighted_score DESC, team_name ASC;
	`
	rows, err := r.db.Query(ctx, query, roundID)
	if err != nil {
		return nil, fmt.Errorf("calculate leaderboard: %w", err)
	}
	defer rows.Close()

	var entries []AdminLeaderboardEntry
	rank := 1
	gradedCount := 0
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
			JOIN judging_criteria c ON c.id = sc.criterion_id
			WHERE sc.slot_id = $1 AND sc.is_locked = true
			GROUP BY c.id, c.name, c.min_score, c.max_score, c.weight;
		`, re.slotID)
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

		var avgRawScore float64
		if critCount > 0 {
			avgRawScore = sumRawAvg / float64(critCount)
		}

		allLocked := re.totalJudgesOnPanel > 0 && re.judgesEvaluatedCount >= re.totalJudgesOnPanel
		if allLocked {
			gradedCount++
			totalGradedWeightedScore += re.totalWeightedScore
		}

		entries = append(entries, AdminLeaderboardEntry{
			Rank:                   rank,
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
		rank++
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
func (r *Repository) recalibratePanelEstimatesTx(ctx context.Context, tx pgx.Tx, panelID string, durationMins int) error {
	now := time.Now()
	recalQuery := `
		SELECT id, sequence_order
		FROM judging_slots
		WHERE panel_id = $1 AND status IN ('scheduled', 'queued')
		ORDER BY sequence_order ASC;
	`
	rows, err := tx.Query(ctx, recalQuery, panelID)
	if err != nil {
		return err
	}
	type slotSeq struct {
		id  string
		seq int
	}
	var pending []slotSeq
	for rows.Next() {
		var s slotSeq
		if err := rows.Scan(&s.id, &s.seq); err == nil {
			pending = append(pending, s)
		}
	}
	rows.Close()

	for idx, s := range pending {
		est := now.Add(time.Duration(idx*durationMins) * time.Minute)
		if _, err := tx.Exec(ctx, `UPDATE judging_slots SET estimated_start_time = $2, updated_at = CURRENT_TIMESTAMP WHERE id = $1;`, s.id, est); err != nil {
			return err
		}
	}
	return nil
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
		SELECT s.panel_id, s.team_id, s.sequence_order, s.status::text, p.round_id, r.slot_duration_mins
		FROM judging_slots s
		JOIN judging_panels p ON p.id = s.panel_id
		JOIN judging_rounds r ON r.id = p.round_id
		WHERE s.id = $1;
	`
	if err := tx.QueryRow(ctx, query, slotID).Scan(&sourcePanelID, &teamID, &currentSeqOrder, &currentStatus, &sourceRoundID, &durationMins); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrSlotNotFound
		}
		return fmt.Errorf("fetch current slot: %w", err)
	}

	if currentStatus == "completed" {
		return errors.New("cannot move an already completed slot")
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

	// Use temporary high sequence order to avoid UNIQUE (panel_id, sequence_order) violations
	if isCrossPanel {
		if _, err := tx.Exec(ctx, `UPDATE judging_slots SET sequence_order = sequence_order + 10000 WHERE panel_id = $1;`, sourcePanelID); err != nil {
			return fmt.Errorf("offset source panel slots: %w", err)
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE judging_slots SET sequence_order = sequence_order + 10000 WHERE panel_id = $1;`, targetPanelID); err != nil {
		return fmt.Errorf("offset target panel slots: %w", err)
	}

	// Update slot's panel and status
	if _, err := tx.Exec(ctx, `
		UPDATE judging_slots
		SET panel_id = $1, status = $2::judging_slot_status, updated_at = CURRENT_TIMESTAMP
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
