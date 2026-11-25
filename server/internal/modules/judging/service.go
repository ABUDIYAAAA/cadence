package judging

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"devclubxnst.com/cadence/internal/modules/auth"
	"devclubxnst.com/cadence/pkg/audit"
	"devclubxnst.com/cadence/pkg/metrics"
)

var (
	ErrNotAllJudgesLocked  = errors.New("cannot complete slot: not all judges on this panel have locked their scorecards")
	ErrInvalidCriteria     = errors.New("each criterion must have valid min/max scores (max > min >= 0) and non-negative weight")
	ErrScoreOutOfRange     = errors.New("submitted score is outside criterion allowed score range")
	ErrCriterionNotInRound = errors.New("submitted criterion does not belong to this slot's judging round")
	ErrProjectNameTooLong  = errors.New("project name must be at most 150 characters")
	ErrDuplicateCriterion  = errors.New("duplicate criterion_id in score submission")
)

type Service struct {
	repo       *Repository
	auditQueue audit.Recorder
}

func NewService(repo *Repository, auditQueue audit.Recorder) *Service {
	return &Service{
		repo:       repo,
		auditQueue: auditQueue,
	}
}

func actorEmail(ctx context.Context) string {
	if claims, ok := auth.GetUserClaims(ctx); ok && claims != nil && claims.Email != "" {
		return claims.Email
	}
	return "staff"
}

// CreateRound validates criteria and registers a new judging round.
func (s *Service) CreateRound(ctx context.Context, req CreateRoundRequest) (string, error) {
	for _, c := range req.Criteria {
		if c.MinScore < 0 || c.MaxScore <= c.MinScore || c.Weight < 0 {
			return "", ErrInvalidCriteria
		}
	}

	roundID, err := s.repo.CreateRound(ctx, req)
	if err != nil {
		return "", err
	}

	if s.auditQueue != nil {
		s.auditQueue.RecordAction(
			ctx,
			audit.ActionJudgingRoundCreate,
			audit.TargetJudgingRound,
			&roundID,
			fmt.Sprintf("Judging round '%s' created by %s", req.Name, actorEmail(ctx)),
			map[string]any{
				"round_name":         req.Name,
				"buffer_size":        req.BufferSize,
				"slot_duration_mins": req.SlotDurationMins,
				"criteria_count":     len(req.Criteria),
			},
		)
	}

	return roundID, nil
}

// ActivateRound sets round status to active.
func (s *Service) ActivateRound(ctx context.Context, roundID string) error {
	if err := s.repo.ActivateRound(ctx, roundID); err != nil {
		return err
	}

	if s.auditQueue != nil {
		s.auditQueue.RecordAction(
			ctx,
			audit.ActionJudgingRoundActivate,
			audit.TargetJudgingRound,
			&roundID,
			fmt.Sprintf("Judging round activated by %s", actorEmail(ctx)),
			map[string]any{"round_id": roundID},
		)
	}

	return nil
}

// ListRounds lists all judging rounds.
func (s *Service) ListRounds(ctx context.Context) ([]map[string]any, error) {
	return s.repo.ListRounds(ctx)
}

// GetRound fetches a single round with criteria.
func (s *Service) GetRound(ctx context.Context, roundID string) (*CreateRoundRequest, string, int, int, []CriterionOutput, error) {
	return s.repo.GetRound(ctx, roundID)
}

// EndRound closes an active round.
func (s *Service) EndRound(ctx context.Context, roundID string, force bool) error {
	if err := s.repo.EndRound(ctx, roundID, force); err != nil {
		return err
	}
	if s.auditQueue != nil {
		s.auditQueue.RecordAction(ctx, audit.ActionJudgingRoundEnd, audit.TargetJudgingRound, &roundID,
			fmt.Sprintf("Judging round ended by %s (force: %t)", actorEmail(ctx), force),
			map[string]any{"round_id": roundID, "force": force})
	}
	return nil
}

// DeleteRound permanently deletes a round that is not active.
func (s *Service) DeleteRound(ctx context.Context, roundID string, force bool) error {
	if err := s.repo.DeleteRound(ctx, roundID, force); err != nil {
		return err
	}
	if s.auditQueue != nil {
		s.auditQueue.RecordAction(ctx, audit.ActionJudgingRoundDelete, audit.TargetJudgingRound, &roundID,
			fmt.Sprintf("Judging round deleted by %s (force: %t)", actorEmail(ctx), force),
			map[string]any{"round_id": roundID, "force": force})
	}
	return nil
}

// GetRoundPanels lists a round's panels and the teams each holds.
func (s *Service) GetRoundPanels(ctx context.Context, roundID string) ([]RoundPanelOverview, error) {
	return s.repo.GetRoundPanels(ctx, roundID)
}

// ImportTeamCriteria seeds the round roster and each team's extra criterion.
func (s *Service) ImportTeamCriteria(ctx context.Context, roundID string, req ImportTeamCriteriaRequest) error {
	if err := s.repo.ImportTeamCriteria(ctx, roundID, req); err != nil {
		return err
	}
	if s.auditQueue != nil {
		s.auditQueue.RecordAction(
			ctx,
			audit.ActionJudgingTeamCriteriaImport,
			audit.TargetJudgingRound,
			&roundID,
			fmt.Sprintf("Imported %d team criteria for round by %s", len(req.Rows), actorEmail(ctx)),
			map[string]any{"round_id": roundID, "team_count": len(req.Rows), "weight": req.Weight},
		)
	}
	return nil
}

// SetTeamCriterion sets a team's custom criterion and the marks awarded for it.
func (s *Service) SetTeamCriterion(ctx context.Context, roundID, teamID string, req SetTeamCriterionRequest) error {
	if err := s.repo.SetTeamCriterion(ctx, roundID, teamID, req); err != nil {
		return err
	}
	if s.auditQueue != nil {
		s.auditQueue.RecordAction(
			ctx,
			audit.ActionJudgingTeamCriterionSet,
			audit.TargetJudgingRound,
			&roundID,
			fmt.Sprintf("Team marks set by %s", actorEmail(ctx)),
			map[string]any{"round_id": roundID, "team_id": teamID, "criterion": req.CriterionName, "max_score": req.MaxScore, "weight": req.Weight, "score": req.Score},
		)
	}
	return nil
}

// DeleteTeamCriterion removes a team's custom criterion.
func (s *Service) DeleteTeamCriterion(ctx context.Context, roundID, teamID string) error {
	if err := s.repo.DeleteTeamCriterion(ctx, roundID, teamID); err != nil {
		return err
	}
	if s.auditQueue != nil {
		s.auditQueue.RecordAction(
			ctx,
			audit.ActionJudgingTeamCriterionDel,
			audit.TargetJudgingRound,
			&roundID,
			fmt.Sprintf("Team criterion removed by %s", actorEmail(ctx)),
			map[string]any{"round_id": roundID, "team_id": teamID},
		)
	}
	return nil
}

// GetScoreSheet returns every mark in a round.
func (s *Service) GetScoreSheet(ctx context.Context, roundID string) (*ScoreSheetResponse, error) {
	return s.repo.GetScoreSheet(ctx, roundID)
}

// GetRoundTeams lists the round roster with each team's extra criterion.
func (s *Service) GetRoundTeams(ctx context.Context, roundID string) ([]RoundTeam, error) {
	return s.repo.GetRoundTeams(ctx, roundID)
}

// GetAssignedJudges lists judges already assigned to a panel in the round.
func (s *Service) GetAssignedJudges(ctx context.Context, roundID string) ([]AssignedJudge, error) {
	return s.repo.GetAssignedJudges(ctx, roundID)
}

// CreatePanel creates a panel in a room and assigns judges.
func (s *Service) CreatePanel(ctx context.Context, req CreatePanelRequest) (string, error) {
	panelID, err := s.repo.CreatePanel(ctx, req)
	if err != nil {
		return "", err
	}

	if s.auditQueue != nil {
		s.auditQueue.RecordAction(
			ctx,
			audit.ActionJudgingPanelCreate,
			audit.TargetJudgingPanel,
			&panelID,
			fmt.Sprintf("Judging panel '%s' created in Room '%s' with %d judges", req.Name, req.Room, len(req.JudgeIDs)),
			map[string]any{
				"round_id":    req.RoundID,
				"panel_name":  req.Name,
				"room":        req.Room,
				"judge_count": len(req.JudgeIDs),
			},
		)
	}

	return panelID, nil
}

// ConfigureSlots sets sequential team schedule in a panel.
func (s *Service) ConfigureSlots(ctx context.Context, panelID string, req ConfigureSlotsRequest) error {
	if err := s.repo.ConfigureSlots(ctx, panelID, req.Slots, req.Append); err != nil {
		return err
	}

	if s.auditQueue != nil {
		s.auditQueue.RecordAction(
			ctx,
			audit.ActionJudgingSlotsConfigure,
			audit.TargetJudgingPanel,
			&panelID,
			fmt.Sprintf("Configured %d team slots in panel by %s", len(req.Slots), actorEmail(ctx)),
			map[string]any{
				"panel_id":   panelID,
				"slot_count": len(req.Slots),
			},
		)
	}

	return nil
}

// sanitizeSubmissionArtifacts extracts a trimmed project_name (<=150 runes) and filters links to allowedSubmissionLinks.
func sanitizeSubmissionArtifacts(projectName string, rawLinks map[string]string) (string, map[string]string, error) {
	project := strings.TrimSpace(projectName)
	if project == "" && rawLinks != nil {
		project = strings.TrimSpace(rawLinks["project_name"])
	}
	if len([]rune(project)) > 150 {
		return "", nil, ErrProjectNameTooLong
	}
	links := make(map[string]string, len(allowedSubmissionLinks))
	for k, v := range rawLinks {
		v = strings.TrimSpace(v)
		if v != "" && allowedSubmissionLinks[k] {
			links[k] = v
		}
	}
	return project, links, nil
}

// UpdateSlotLinks modifies external project URLs for a slot's team.
// If the caller is not an admin, verifies that the user belongs to the team assigned to this slot.
func (s *Service) UpdateSlotLinks(
	ctx context.Context,
	slotID string,
	actorID, actorRole string,
	links map[string]string,
) error {
	if !auth.IsAdminRole(actorRole) {
		allowed, err := s.repo.CanUserUpdateSlotLinks(ctx, slotID, actorID)
		if err != nil {
			return fmt.Errorf("verify slot permission: %w", err)
		}
		if !allowed {
			exists, err := s.repo.SlotExists(ctx, slotID)
			if err != nil {
				return fmt.Errorf("verify slot exists: %w", err)
			}
			if !exists {
				return ErrSlotNotFound
			}
			return ErrUnauthorizedSlot
		}
	}
	teamID, err := s.repo.SlotTeamID(ctx, slotID)
	if err != nil {
		return err
	}
	if !auth.IsAdminRole(actorRole) {
		if ok, _, err := s.gateStatus(ctx, teamID); err != nil {
			return err
		} else if !ok {
			return ErrNotPresent
		}
	}
	project, cleanedLinks, err := sanitizeSubmissionArtifacts("", links)
	if err != nil {
		return err
	}
	if err := s.repo.SetTeamSubmission(ctx, teamID, project, cleanedLinks); err != nil {
		return err
	}

	// Record which links changed, not their URLs: they can carry private repo or deck addresses.
	keys := make([]string, 0, len(cleanedLinks))
	for k := range cleanedLinks {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if s.auditQueue != nil {
		s.auditQueue.RecordAction(
			ctx,
			audit.ActionJudgingSlotLinksUpdate,
			audit.TargetJudgingSlot,
			&slotID,
			fmt.Sprintf("Submission links updated by %s", actorEmail(ctx)),
			map[string]any{"slot_id": slotID, "team_id": teamID, "link_keys": keys, "has_project_name": project != ""},
		)
	}
	return nil
}

// GetJudgePanels returns panels and queues assigned to this judge.
func (s *Service) GetJudgePanels(ctx context.Context, judgeID string) ([]JudgePanelSummary, error) {
	return s.repo.GetJudgePanels(ctx, judgeID)
}

// GetJudgeSlotDetails retrieves full evaluation view for a single slot, with counter info.
// Verifies that the judge is assigned to this slot's panel.
func (s *Service) GetJudgeSlotDetails(ctx context.Context, slotID, judgeID string) (*JudgeSlotDetails, error) {
	if err := s.authorizeSlotActor(ctx, slotID, judgeID); err != nil {
		return nil, err
	}
	return s.repo.GetJudgeSlotDetails(ctx, slotID, judgeID)
}

// StartSlot starts a team's pitch session and triggers on-deck queue shifts.
func (s *Service) StartSlot(ctx context.Context, slotID, actorID string) error {
	if err := s.authorizeSlotActor(ctx, slotID, actorID); err != nil {
		return err
	}

	if err := s.repo.StartSlot(ctx, slotID); err != nil {
		return err
	}

	if s.auditQueue != nil {
		s.auditQueue.RecordAction(
			ctx,
			audit.ActionJudgingSlotStart,
			audit.TargetJudgingSlot,
			&slotID,
			fmt.Sprintf("Judging slot pitch started by %s", actorEmail(ctx)),
			map[string]any{"slot_id": slotID},
		)
	}

	return nil
}

// authorizeSlotActor allows only judges assigned to the slot's panel.
func (s *Service) authorizeSlotActor(ctx context.Context, slotID, actorID string) error {
	assigned, err := s.repo.IsJudgeAssignedToSlot(ctx, slotID, actorID)
	if err != nil {
		return fmt.Errorf("verify judge assignment: %w", err)
	}
	if assigned {
		return nil
	}
	exists, err := s.repo.SlotExists(ctx, slotID)
	if err != nil {
		return fmt.Errorf("verify slot exists: %w", err)
	}
	if !exists {
		return ErrSlotNotFound
	}
	return ErrJudgeNotAssigned
}

// PauseTimer pauses the pitch clock.
func (s *Service) PauseTimer(ctx context.Context, slotID, actorID string) error {
	if err := s.authorizeSlotActor(ctx, slotID, actorID); err != nil {
		return err
	}
	if err := s.repo.PauseTimer(ctx, slotID); err != nil {
		return err
	}
	if s.auditQueue != nil {
		s.auditQueue.RecordAction(ctx, audit.ActionJudgingTimerPause, audit.TargetJudgingSlot, &slotID,
			fmt.Sprintf("Pitch timer paused by %s", actorEmail(ctx)),
			map[string]any{"slot_id": slotID})
	}
	return nil
}

// ResumeTimer resumes a paused pitch clock.
func (s *Service) ResumeTimer(ctx context.Context, slotID, actorID string) error {
	if err := s.authorizeSlotActor(ctx, slotID, actorID); err != nil {
		return err
	}
	if err := s.repo.ResumeTimer(ctx, slotID); err != nil {
		return err
	}
	if s.auditQueue != nil {
		s.auditQueue.RecordAction(ctx, audit.ActionJudgingTimerResume, audit.TargetJudgingSlot, &slotID,
			fmt.Sprintf("Pitch timer resumed by %s", actorEmail(ctx)),
			map[string]any{"slot_id": slotID})
	}
	return nil
}

// ExtendTimer adds extra time to a pitch.
func (s *Service) ExtendTimer(ctx context.Context, slotID, actorID string, seconds int) error {
	if err := s.authorizeSlotActor(ctx, slotID, actorID); err != nil {
		return err
	}
	if err := s.repo.ExtendTimer(ctx, slotID, seconds); err != nil {
		return err
	}
	if s.auditQueue != nil {
		s.auditQueue.RecordAction(ctx, audit.ActionJudgingTimerExtend, audit.TargetJudgingSlot, &slotID,
			fmt.Sprintf("Pitch timer extended by %ds by %s", seconds, actorEmail(ctx)),
			map[string]any{"slot_id": slotID, "seconds": seconds})
	}
	return nil
}

// SubmitScores records or updates criteria scores for a judge.
func (s *Service) SubmitScores(
	ctx context.Context,
	slotID, judgeID string,
	req SubmitScoresRequest,
) error {
	if err := s.authorizeSlotActor(ctx, slotID, judgeID); err != nil {
		return err
	}
	err := s.repo.UpsertScores(ctx, slotID, judgeID, req.Scores)
	if err == nil {
		metrics.InstrumentScoreSubmission("draft")
	}
	return err
}

// LockScores locks the judge's score for the team.
func (s *Service) LockScores(ctx context.Context, slotID, judgeID string) error {
	if err := s.authorizeSlotActor(ctx, slotID, judgeID); err != nil {
		return err
	}

	if err := s.repo.LockJudgeScorecard(ctx, slotID, judgeID); err != nil {
		return err
	}

	metrics.InstrumentScoreSubmission("locked")

	if s.auditQueue != nil {
		s.auditQueue.RecordAction(
			ctx,
			audit.ActionJudgingScoreLock,
			audit.TargetJudgingSlot,
			&slotID,
			fmt.Sprintf("Judge %s locked their evaluation scorecard", actorEmail(ctx)),
			map[string]any{"slot_id": slotID},
		)
	}

	return nil
}

// CompleteSlot validates all judges locked scores, then completes the slot and advances the queue.
func (s *Service) CompleteSlot(ctx context.Context, slotID, actorID string) error {
	if err := s.authorizeSlotActor(ctx, slotID, actorID); err != nil {
		return err
	}

	allLocked, err := s.repo.CheckAllPanelJudgesLocked(ctx, slotID)
	if err != nil {
		return err
	}
	if !allLocked {
		return ErrNotAllJudgesLocked
	}

	if err := s.repo.CompleteSlot(ctx, slotID); err != nil {
		return err
	}

	if s.auditQueue != nil {
		s.auditQueue.RecordAction(
			ctx,
			audit.ActionJudgingSlotComplete,
			audit.TargetJudgingSlot,
			&slotID,
			fmt.Sprintf("Judging slot completed and queue advanced by %s", actorEmail(ctx)),
			map[string]any{"slot_id": slotID},
		)
	}

	return nil
}

// GetTeamStatusForUser looks up the user's team and fetches their live queue coordinates.
func (s *Service) GetTeamStatusForUser(ctx context.Context, userID string) (*TeamStatusResponse, error) {
	teamID, err := s.repo.GetUserTeamID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if teamID == nil {
		return &TeamStatusResponse{
			HasActiveRound: false,
			Instructions:   "You are not assigned to any team.",
		}, nil
	}
	return s.withSubmitGate(ctx, *teamID)
}

// withSubmitGate loads a team's status and marks whether the attendance gate lets it submit.
func (s *Service) withSubmitGate(ctx context.Context, teamID string) (*TeamStatusResponse, error) {
	resp, err := s.repo.GetTeamActiveStatus(ctx, teamID)
	if err != nil {
		return nil, err
	}
	can, reason, err := s.gateStatus(ctx, teamID)
	if err != nil {
		return nil, err
	}
	resp.CanSubmit = can
	resp.SubmitBlockedReason = reason
	return resp, nil
}

// GetSubmissionGate returns the optional attendance/check-in session that gates submissions for a round.
func (s *Service) GetSubmissionGate(ctx context.Context, roundID string) (SubmissionGate, error) {
	return s.repo.GetSubmissionGate(ctx, roundID)
}

// SetSubmissionGate sets or clears the optional gating session for a round.
func (s *Service) SetSubmissionGate(ctx context.Context, roundID string, sessionID *string, actorID string) (SubmissionGate, error) {
	if sessionID != nil && *sessionID == "" {
		sessionID = nil
	}
	gate, err := s.repo.SetSubmissionGate(ctx, roundID, sessionID, actorID)
	if err != nil {
		return gate, err
	}
	if s.auditQueue != nil {
		s.auditQueue.RecordAction(
			ctx,
			audit.ActionJudgingSlotLinksUpdate,
			audit.TargetJudgingSlot,
			nil,
			fmt.Sprintf("Submission gate changed by %s", actorEmail(ctx)),
			map[string]any{"round_id": roundID, "submission_gate_session": sessionID},
		)
	}
	return gate, nil
}

// ListSubmissions lists each team's project name and links for the CSV export.
func (s *Service) ListSubmissions(ctx context.Context) (*SubmissionsResponse, error) {
	rows, err := s.repo.ListSubmissions(ctx)
	if err != nil {
		return nil, err
	}
	return &SubmissionsResponse{Submissions: rows}, nil
}

// gateStatus reports whether the team may submit under the active (published) round's optional gate.
// Submissions stay closed until a judging round is published (status = 'active').
// Once active, if no attendance gate session is configured on that round, submissions are open to all teams.
func (s *Service) gateStatus(ctx context.Context, teamID string) (bool, string, error) {
	const (
		closed       = "Submissions are closed for your team."
		notPublished = "Submissions open once a judging round is published."
	)
	roundID, err := s.repo.ResolveActiveGateRoundID(ctx)
	if err != nil {
		return false, "", err
	}
	if roundID == "" {
		return false, notPublished, nil
	}
	gate, err := s.repo.GetSubmissionGate(ctx, roundID)
	if err != nil {
		return false, "", err
	}
	// Optional gate: once the round is published, no session means submissions are open to all teams.
	if gate.SessionID == nil {
		return true, "", nil
	}
	present, err := s.repo.TeamPresentInSession(ctx, teamID, *gate.SessionID)
	if err != nil {
		return false, "", fmt.Errorf("check attendance: %w", err)
	}
	if present {
		return true, "", nil
	}
	if gate.SessionName != nil && *gate.SessionName != "" {
		return false, fmt.Sprintf("No member of your team was present at %q, so submissions are closed for your team.", *gate.SessionName), nil
	}
	return false, closed, nil
}

// GetMySubmission returns the caller's team submission with its gate status.
func (s *Service) GetMySubmission(ctx context.Context, userID string) (*MySubmission, error) {
	teamID, err := s.repo.GetUserTeamID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if teamID == nil {
		return &MySubmission{Links: map[string]string{}}, nil
	}
	name, project, links, err := s.repo.GetTeamSubmission(ctx, *teamID)
	if err != nil {
		return nil, err
	}
	can, reason, err := s.gateStatus(ctx, *teamID)
	if err != nil {
		return nil, err
	}
	return &MySubmission{HasTeam: true, TeamName: name, ProjectName: project, Links: links, CanSubmit: can, SubmitBlockedReason: reason}, nil
}

var allowedSubmissionLinks = map[string]bool{"github": true, "demo": true, "slides": true}

// SetMySubmission saves the caller's team submission, subject to the attendance gate.
func (s *Service) SetMySubmission(ctx context.Context, actorID string, req UpdateMySubmissionRequest) error {
	teamID, err := s.repo.GetUserTeamID(ctx, actorID)
	if err != nil {
		return err
	}
	if teamID == nil {
		return ErrUnauthorizedSlot
	}
	if can, _, err := s.gateStatus(ctx, *teamID); err != nil {
		return err
	} else if !can {
		return ErrNotPresent
	}
	project, links, err := sanitizeSubmissionArtifacts(req.ProjectName, req.Links)
	if err != nil {
		return err
	}
	if err := s.repo.SetTeamSubmission(ctx, *teamID, project, links); err != nil {
		return err
	}
	keys := make([]string, 0, len(links))
	for k := range links {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if s.auditQueue != nil {
		s.auditQueue.RecordAction(ctx, audit.ActionJudgingSlotLinksUpdate, audit.TargetJudgingSlot, teamID,
			fmt.Sprintf("Team submission updated by %s", actorEmail(ctx)),
			map[string]any{"team_id": *teamID, "link_keys": keys, "has_project_name": project != ""})
	}
	return nil
}

// GetTeamStatus returns the live queue position and room coordinates for a team.
func (s *Service) GetTeamStatus(ctx context.Context, teamID string) (*TeamStatusResponse, error) {
	return s.withSubmitGate(ctx, teamID)
}

// GetVolunteerSchedule returns room coordinates, on-deck teams, and upcoming teams for coordinators.
// A non-nil judgeID restricts the schedule to that judge's assigned panels.
func (s *Service) GetVolunteerSchedule(ctx context.Context, judgeID *string) ([]VolunteerPanelSchedule, error) {
	return s.repo.GetVolunteerSchedule(ctx, judgeID)
}

// GetRoundAnalytics returns standings, panel, criterion and judge statistics for a round.
func (s *Service) GetRoundAnalytics(ctx context.Context, roundID string) (*RoundAnalyticsResponse, error) {
	return s.repo.GetRoundAnalytics(ctx, roundID)
}

// GetAdminLeaderboard calculates full round rankings with weighted criteria breakdown.
func (s *Service) GetAdminLeaderboard(ctx context.Context, roundID string) (*AdminLeaderboardResponse, error) {
	return s.repo.GetAdminLeaderboard(ctx, roundID)
}

// UnlockScorecard reopens locked scorecards on a team so the judges can edit and lock again.
func (s *Service) UnlockScorecard(
	ctx context.Context,
	slotID string,
	req UnlockScorecardRequest,
	actorID, actorRole string,
) ([]string, bool, error) {
	// A judge can only reopen their own scorecard, on a panel they sit on. Admins can pick any judge.
	if actorRole == "judge" {
		if err := s.authorizeSlotActor(ctx, slotID, actorID); err != nil {
			return nil, false, err
		}
		req.JudgeID = actorID
	}
	judges, reopened, err := s.repo.UnlockScorecards(ctx, slotID, req.JudgeID)
	if err != nil {
		return nil, false, err
	}
	if s.auditQueue != nil {
		s.auditQueue.RecordAction(
			ctx,
			audit.ActionJudgingScoreUnlock,
			audit.TargetJudgingSlot,
			&slotID,
			fmt.Sprintf("Scorecard unlocked by %s", actorEmail(ctx)),
			map[string]any{"slot_id": slotID, "judge_ids": judges, "slot_reopened": reopened, "reason": req.Reason},
		)
	}
	return judges, reopened, nil
}

// UpdateRoundDuration changes a round's default time per team.
func (s *Service) UpdateRoundDuration(ctx context.Context, roundID string, req UpdateRoundDurationRequest) (int, error) {
	reset, err := s.repo.UpdateRoundSlotDuration(ctx, roundID, req.SlotDurationMins, req.ResetCustom)
	if err != nil {
		return 0, err
	}

	if s.auditQueue != nil {
		s.auditQueue.RecordAction(
			ctx,
			audit.ActionJudgingRoundDuration,
			audit.TargetJudgingRound,
			&roundID,
			fmt.Sprintf("Round time per team changed by %s", actorEmail(ctx)),
			map[string]any{"round_id": roundID, "slot_duration_mins": req.SlotDurationMins, "reset_custom": req.ResetCustom, "teams_reset": reset},
		)
	}
	return reset, nil
}

// SetPanelJudges replaces the judges seated in a room.
func (s *Service) SetPanelJudges(ctx context.Context, panelID string, req SetPanelJudgesRequest) error {
	added, removed, err := s.repo.SetPanelJudges(ctx, panelID, req.JudgeIDs)
	if err != nil {
		return err
	}
	if len(added) == 0 && len(removed) == 0 {
		return nil
	}

	if s.auditQueue != nil {
		s.auditQueue.RecordAction(
			ctx,
			audit.ActionJudgingPanelJudges,
			audit.TargetJudgingPanel,
			&panelID,
			fmt.Sprintf("Room judges changed by %s", actorEmail(ctx)),
			map[string]any{"panel_id": panelID, "added": added, "removed": removed},
		)
	}
	return nil
}

// UpdateSlotTiming sets a team's own pitch length and pinned start time.
func (s *Service) UpdateSlotTiming(ctx context.Context, slotID string, req UpdateSlotTimingRequest) error {
	if err := s.repo.UpdateSlotTiming(ctx, slotID, req); err != nil {
		return err
	}

	metadata := map[string]any{"slot_id": slotID, "duration_mins": nil, "start_time": nil}
	if req.DurationMins != nil {
		metadata["duration_mins"] = *req.DurationMins
	}
	if req.StartTime != nil {
		metadata["start_time"] = req.StartTime.UTC().Format(time.RFC3339)
	}

	if s.auditQueue != nil {
		s.auditQueue.RecordAction(
			ctx,
			audit.ActionJudgingSlotTiming,
			audit.TargetJudgingSlot,
			&slotID,
			fmt.Sprintf("Judging slot time changed by %s", actorEmail(ctx)),
			metadata,
		)
	}
	return nil
}

// MoveSlot moves a team slot across panels or reorders inside a panel.
func (s *Service) MoveSlot(ctx context.Context, slotID string, req MoveSlotRequest) error {
	if err := s.repo.MoveSlot(ctx, slotID, req); err != nil {
		return err
	}

	metadata := map[string]any{"slot_id": slotID}
	if req.TargetPanelID != nil {
		metadata["target_panel_id"] = *req.TargetPanelID
	}
	if req.TargetSequenceOrder != nil {
		metadata["target_sequence_order"] = *req.TargetSequenceOrder
	}
	if req.Status != nil {
		metadata["status"] = *req.Status
	}

	if s.auditQueue != nil {
		s.auditQueue.RecordAction(
			ctx,
			audit.ActionJudgingSlotMove,
			audit.TargetJudgingSlot,
			&slotID,
			fmt.Sprintf("Judging slot moved by %s", actorEmail(ctx)),
			metadata,
		)
	}

	return nil
}

// QueueSlot immediately puts a team slot on-deck into the room queue ('queued' status).
func (s *Service) QueueSlot(ctx context.Context, slotID string) error {
	if err := s.repo.QueueSlot(ctx, slotID); err != nil {
		return err
	}

	if s.auditQueue != nil {
		s.auditQueue.RecordAction(
			ctx,
			audit.ActionJudgingSlotQueue,
			audit.TargetJudgingSlot,
			&slotID,
			fmt.Sprintf("Team slot thrown into room queue by %s", actorEmail(ctx)),
			map[string]any{"slot_id": slotID, "status": "queued"},
		)
	}

	return nil
}

// UnqueueSlot reverts a queued slot back to scheduled.
func (s *Service) UnqueueSlot(ctx context.Context, slotID string) error {
	if err := s.repo.UnqueueSlot(ctx, slotID); err != nil {
		return err
	}

	if s.auditQueue != nil {
		s.auditQueue.RecordAction(
			ctx,
			audit.ActionJudgingSlotUnqueue,
			audit.TargetJudgingSlot,
			&slotID,
			fmt.Sprintf("Team slot removed from queue by %s", actorEmail(ctx)),
			map[string]any{"slot_id": slotID, "status": "scheduled"},
		)
	}

	return nil
}
