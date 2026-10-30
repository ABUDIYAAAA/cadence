package judging

import (
	"context"
	"fmt"
	"sort"
)

// RoundAnalyticsResponse is the admin analytics view of a single round.
type RoundAnalyticsResponse struct {
	RoundID           string                  `json:"round_id"`
	RoundName         string                  `json:"round_name"`
	Status            string                  `json:"status"`
	Summary           AnalyticsSummary        `json:"summary"`
	TopTeams          []AdminLeaderboardEntry `json:"top_teams"`
	BottomTeams       []AdminLeaderboardEntry `json:"bottom_teams"`
	Panels            []PanelAnalytics        `json:"panels"`
	Criteria          []CriterionAnalytics    `json:"criteria"`
	Judges            []JudgeAnalytics        `json:"judges"`
	ScoreDistribution []DistributionBucket    `json:"score_distribution"`
}

type AnalyticsSummary struct {
	TotalTeams       int     `json:"total_teams"`
	GradedTeams      int     `json:"graded_teams"`
	PartialTeams     int     `json:"partial_teams"`
	TotalJudges      int     `json:"total_judges"`
	ScorecardsLocked int     `json:"scorecards_locked"`
	MaxPossibleScore float64 `json:"max_possible_score"`
	TotalScore       float64 `json:"total_score"`
	AverageScore     float64 `json:"average_score"`
	MedianScore      float64 `json:"median_score"`
	HighestScore     float64 `json:"highest_score"`
	LowestScore      float64 `json:"lowest_score"`
	AvgGradingSecs   float64 `json:"avg_grading_secs"`
}

type PanelAnalytics struct {
	PanelID        string  `json:"panel_id"`
	PanelName      string  `json:"panel_name"`
	Room           string  `json:"room"`
	Judges         int     `json:"judges"`
	TotalTeams     int     `json:"total_teams"`
	GradedTeams    int     `json:"graded_teams"`
	TotalScore     float64 `json:"total_score"`
	AverageScore   float64 `json:"average_score"`
	HighestScore   float64 `json:"highest_score"`
	LowestScore    float64 `json:"lowest_score"`
	TopTeam        string  `json:"top_team,omitempty"`
	AvgGradingSecs float64 `json:"avg_grading_secs"`
}

type CriterionAnalytics struct {
	CriterionID  string  `json:"criterion_id"`
	Name         string  `json:"name"`
	MinScore     int     `json:"min_score"`
	MaxScore     int     `json:"max_score"`
	Weight       float64 `json:"weight"`
	Scores       int     `json:"scores"`
	AvgScore     float64 `json:"avg_score"`
	LowestScore  float64 `json:"lowest_score"`
	HighestScore float64 `json:"highest_score"`
	AvgPct       float64 `json:"avg_pct"`
}

type JudgeCriterionAvg struct {
	CriterionID string  `json:"criterion_id"`
	Name        string  `json:"name"`
	AvgScore    float64 `json:"avg_score"`
	AvgPct      float64 `json:"avg_pct"`
	Scores      int     `json:"scores"`
}

type JudgeAnalytics struct {
	JudgeID            string              `json:"judge_id"`
	Name               string              `json:"name"`
	Panels             []string            `json:"panels"`
	ScorecardsLocked   int                 `json:"scorecards_locked"`
	ScorecardsAssigned int                 `json:"scorecards_assigned"`
	AvgGradingSecs     float64             `json:"avg_grading_secs"`
	MinGradingSecs     float64             `json:"min_grading_secs"`
	MaxGradingSecs     float64             `json:"max_grading_secs"`
	AvgSlotSecs        float64             `json:"avg_slot_secs"`
	AvgEdits           float64             `json:"avg_edits"`
	TotalScoreGiven    float64             `json:"total_score_given"`
	AvgTotalPerTeam    float64             `json:"avg_total_per_team"`
	AvgScore           float64             `json:"avg_score"`
	AvgPct             float64             `json:"avg_pct"`
	Criteria           []JudgeCriterionAvg `json:"criteria"`
}

type DistributionBucket struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}

// GetRoundAnalytics assembles standings, panel, criterion and judge statistics for a round.
func (r *Repository) GetRoundAnalytics(ctx context.Context, roundID string) (*RoundAnalyticsResponse, error) {
	lb, err := r.GetAdminLeaderboard(ctx, roundID)
	if err != nil {
		return nil, err
	}

	out := &RoundAnalyticsResponse{
		RoundID: lb.RoundID, RoundName: lb.RoundName, Status: lb.Status,
		TopTeams: []AdminLeaderboardEntry{}, BottomTeams: []AdminLeaderboardEntry{},
		Panels: []PanelAnalytics{}, Criteria: []CriterionAnalytics{}, Judges: []JudgeAnalytics{},
		ScoreDistribution: []DistributionBucket{},
	}

	// Teams that every panel judge has locked are fully graded; those with some locks are partial.
	var graded []AdminLeaderboardEntry
	partial := 0
	for _, e := range lb.Leaderboard {
		if e.AllPanelJudgesLocked {
			graded = append(graded, e)
		} else if e.JudgesEvaluatedCount > 0 {
			partial++
		}
	}
	sort.SliceStable(graded, func(i, j int) bool { return graded[i].TotalWeightedScore > graded[j].TotalWeightedScore })

	sum := &out.Summary
	sum.TotalTeams, sum.GradedTeams, sum.PartialTeams = lb.TotalTeams, len(graded), partial
	sum.MaxPossibleScore = lb.MaxPossibleScore
	scores := make([]float64, 0, len(graded))
	for _, e := range graded {
		scores = append(scores, e.TotalWeightedScore)
		sum.TotalScore += e.TotalWeightedScore
	}
	if n := len(scores); n > 0 {
		sum.AverageScore = sum.TotalScore / float64(n)
		sum.HighestScore, sum.LowestScore = scores[0], scores[n-1]
		if n%2 == 1 {
			sum.MedianScore = scores[n/2]
		} else {
			sum.MedianScore = (scores[n/2-1] + scores[n/2]) / 2
		}
	}
	for i := 0; i < len(graded) && i < 10; i++ {
		out.TopTeams = append(out.TopTeams, graded[i])
	}
	for i := len(graded) - 1; i >= 0 && len(out.BottomTeams) < 5 && len(graded) > 10; i-- {
		out.BottomTeams = append(out.BottomTeams, graded[i])
	}
	out.ScoreDistribution = distribution(scores, lb.MaxPossibleScore)

	if err := r.fillPanelAnalytics(ctx, roundID, graded, lb.Leaderboard, out); err != nil {
		return nil, err
	}
	if err := r.fillCriteriaAnalytics(ctx, roundID, out); err != nil {
		return nil, err
	}
	if err := r.fillJudgeAnalytics(ctx, roundID, out); err != nil {
		return nil, err
	}
	return out, nil
}

func distribution(scores []float64, max float64) []DistributionBucket {
	buckets := make([]DistributionBucket, 5)
	for i := range buckets {
		buckets[i].Label = fmt.Sprintf("%d–%d%%", i*20, (i+1)*20)
	}
	if max <= 0 {
		return buckets
	}
	for _, s := range scores {
		idx := int(s / max * 5)
		if idx > 4 {
			idx = 4
		}
		if idx < 0 {
			idx = 0
		}
		buckets[idx].Count++
	}
	return buckets
}

func (r *Repository) fillPanelAnalytics(ctx context.Context, roundID string, graded, all []AdminLeaderboardEntry, out *RoundAnalyticsResponse) error {
	rows, err := r.db.Query(ctx, `
		SELECT p.id, p.name, p.room,
		       (SELECT COUNT(*) FROM judging_panel_judges pj WHERE pj.panel_id = p.id),
		       COALESCE((SELECT AVG(m.slot_ms) FROM judging_judge_metrics m
		                 WHERE m.panel_id = p.id AND m.locked_at IS NOT NULL), 0) / 1000.0
		FROM judging_panels p WHERE p.round_id = $1 ORDER BY p.name, p.room`, roundID)
	if err != nil {
		return fmt.Errorf("query panel analytics: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var p PanelAnalytics
		if err := rows.Scan(&p.PanelID, &p.PanelName, &p.Room, &p.Judges, &p.AvgGradingSecs); err != nil {
			return fmt.Errorf("scan panel analytics: %w", err)
		}
		out.Panels = append(out.Panels, p)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	// Leaderboard entries carry names, not panel ids, so match on the panel/room pair.
	for i := range out.Panels {
		p := &out.Panels[i]
		for _, e := range all {
			if e.PanelName == p.PanelName && e.Room == p.Room {
				p.TotalTeams++
			}
		}
		first := true
		for _, e := range graded { // graded is sorted best-first
			if e.PanelName != p.PanelName || e.Room != p.Room {
				continue
			}
			p.GradedTeams++
			p.TotalScore += e.TotalWeightedScore
			if first {
				p.TopTeam, p.HighestScore, p.LowestScore, first = e.TeamName, e.TotalWeightedScore, e.TotalWeightedScore, false
			}
			p.LowestScore = e.TotalWeightedScore
		}
		if p.GradedTeams > 0 {
			p.AverageScore = p.TotalScore / float64(p.GradedTeams)
		}
	}
	return nil
}

func (r *Repository) fillCriteriaAnalytics(ctx context.Context, roundID string, out *RoundAnalyticsResponse) error {
	rows, err := r.db.Query(ctx, `
		SELECT c.id, c.name, c.min_score, c.max_score, c.weight::float8,
		       COUNT(sc.id), COALESCE(AVG(sc.score), 0)::float8,
		       COALESCE(MIN(sc.score), 0)::float8, COALESCE(MAX(sc.score), 0)::float8,
		       COALESCE(AVG(CASE WHEN c.max_score > c.min_score
		            THEN (sc.score - c.min_score) / (c.max_score - c.min_score)::numeric END), 0)::float8
		FROM judging_criteria c
		LEFT JOIN (
			judging_scores sc
			JOIN judging_slots s ON s.id = sc.slot_id
			JOIN judging_panels p ON p.id = s.panel_id
		) ON sc.criterion_id = c.id AND sc.is_locked = true AND p.round_id = c.round_id
		WHERE c.round_id = $1
		GROUP BY c.id, c.name, c.min_score, c.max_score, c.weight, c.order_index
		ORDER BY c.order_index, c.name`, roundID)
	if err != nil {
		return fmt.Errorf("query criteria analytics: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var c CriterionAnalytics
		if err := rows.Scan(&c.CriterionID, &c.Name, &c.MinScore, &c.MaxScore, &c.Weight,
			&c.Scores, &c.AvgScore, &c.LowestScore, &c.HighestScore, &c.AvgPct); err != nil {
			return fmt.Errorf("scan criteria analytics: %w", err)
		}
		out.Criteria = append(out.Criteria, c)
	}
	return rows.Err()
}

func (r *Repository) fillJudgeAnalytics(ctx context.Context, roundID string, out *RoundAnalyticsResponse) error {
	rows, err := r.db.Query(ctx, `
		SELECT u.id, u.name,
		       ARRAY_AGG(DISTINCT p.name || ' · ' || p.room ORDER BY p.name || ' · ' || p.room),
		       (SELECT COUNT(*) FROM judging_slots s WHERE s.panel_id = ANY(ARRAY_AGG(DISTINCT p.id))),
		       COALESCE((SELECT COUNT(*) FROM judging_judge_metrics m
		                 WHERE m.judge_id = u.id AND m.round_id = $1 AND m.locked_at IS NOT NULL), 0),
		       COALESCE((SELECT AVG(m.slot_ms) FROM judging_judge_metrics m WHERE m.judge_id = u.id AND m.round_id = $1 AND m.locked_at IS NOT NULL), 0)::float8 / 1000,
		       COALESCE((SELECT MIN(m.slot_ms) FROM judging_judge_metrics m WHERE m.judge_id = u.id AND m.round_id = $1 AND m.locked_at IS NOT NULL), 0)::float8 / 1000,
		       COALESCE((SELECT MAX(m.slot_ms) FROM judging_judge_metrics m WHERE m.judge_id = u.id AND m.round_id = $1 AND m.locked_at IS NOT NULL), 0)::float8 / 1000,
		       COALESCE((SELECT AVG(m.slot_ms) FROM judging_judge_metrics m WHERE m.judge_id = u.id AND m.round_id = $1 AND m.locked_at IS NOT NULL), 0)::float8 / 1000,
		       COALESCE((SELECT AVG(m.edit_count) FROM judging_judge_metrics m WHERE m.judge_id = u.id AND m.round_id = $1 AND m.locked_at IS NOT NULL), 0)::float8,
		       COALESCE((SELECT SUM(m.score_total) FROM judging_judge_metrics m WHERE m.judge_id = u.id AND m.round_id = $1 AND m.locked_at IS NOT NULL), 0)::float8,
		       COALESCE((SELECT AVG(m.score_total) FROM judging_judge_metrics m WHERE m.judge_id = u.id AND m.round_id = $1 AND m.locked_at IS NOT NULL), 0)::float8,
		       COALESCE((SELECT AVG(m.score_avg) FROM judging_judge_metrics m WHERE m.judge_id = u.id AND m.round_id = $1 AND m.locked_at IS NOT NULL), 0)::float8,
		       COALESCE((SELECT AVG(m.score_pct) FROM judging_judge_metrics m WHERE m.judge_id = u.id AND m.round_id = $1 AND m.locked_at IS NOT NULL), 0)::float8
		FROM judging_panels p
		JOIN judging_panel_judges pj ON pj.panel_id = p.id
		JOIN users u ON u.id = pj.judge_id
		WHERE p.round_id = $1
		GROUP BY u.id, u.name
		ORDER BY u.name`, roundID)
	if err != nil {
		return fmt.Errorf("query judge analytics: %w", err)
	}
	defer rows.Close()

	index := map[string]int{}
	for rows.Next() {
		var j JudgeAnalytics
		if err := rows.Scan(&j.JudgeID, &j.Name, &j.Panels, &j.ScorecardsAssigned, &j.ScorecardsLocked,
			&j.AvgGradingSecs, &j.MinGradingSecs, &j.MaxGradingSecs, &j.AvgSlotSecs, &j.AvgEdits,
			&j.TotalScoreGiven, &j.AvgTotalPerTeam, &j.AvgScore, &j.AvgPct); err != nil {
			return fmt.Errorf("scan judge analytics: %w", err)
		}
		j.Criteria = []JudgeCriterionAvg{}
		index[j.JudgeID] = len(out.Judges)
		out.Judges = append(out.Judges, j)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	out.Summary.TotalJudges = len(out.Judges)

	var gradingSum float64
	var gradingN int
	for _, j := range out.Judges {
		out.Summary.ScorecardsLocked += j.ScorecardsLocked
		gradingSum += j.AvgGradingSecs * float64(j.ScorecardsLocked)
		gradingN += j.ScorecardsLocked
	}
	if gradingN > 0 {
		out.Summary.AvgGradingSecs = gradingSum / float64(gradingN)
	}

	crit, err := r.db.Query(ctx, `
		SELECT sc.judge_id, c.id, c.name, COUNT(*), AVG(sc.score)::float8,
		       COALESCE(AVG(CASE WHEN c.max_score > c.min_score
		            THEN (sc.score - c.min_score) / (c.max_score - c.min_score)::numeric END), 0)::float8
		FROM judging_scores sc
		JOIN judging_slots s ON s.id = sc.slot_id
		JOIN judging_panels p ON p.id = s.panel_id
		JOIN judging_criteria c ON c.id = sc.criterion_id AND c.round_id = p.round_id
		WHERE c.round_id = $1 AND sc.is_locked = true
		GROUP BY sc.judge_id, c.id, c.name, c.order_index
		ORDER BY c.order_index, c.name`, roundID)
	if err != nil {
		return fmt.Errorf("query judge criteria analytics: %w", err)
	}
	defer crit.Close()
	for crit.Next() {
		var judgeID string
		var a JudgeCriterionAvg
		if err := crit.Scan(&judgeID, &a.CriterionID, &a.Name, &a.Scores, &a.AvgScore, &a.AvgPct); err != nil {
			return fmt.Errorf("scan judge criteria analytics: %w", err)
		}
		if i, ok := index[judgeID]; ok {
			out.Judges[i].Criteria = append(out.Judges[i].Criteria, a)
		}
	}
	return crit.Err()
}
