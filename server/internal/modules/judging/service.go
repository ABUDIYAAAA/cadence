package judging

import (
	"context"
	"errors"
	"fmt"
	"time"

	"devclubxnst.com/cadence/pkg/audit"
	"devclubxnst.com/cadence/pkg/metrics"
)

var (
	ErrNotAllJudgesLocked = errors.New("cannot complete slot: not all judges on this panel have locked their scorecards")
	ErrInvalidCriteria    = errors.New("each criterion must have valid min/max scores (max > min >= 0) and non-negative weight")
	ErrScoreOutOfRange    = errors.New("submitted score is outside criterion allowed score range")
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

// CreateRound validates criteria and registers a new judging round.
func (s *Service) CreateRound(
	ctx context.Context,
	req CreateRoundRequest,
	actorID, actorEmail, actorRole string,
	ip, userAgent string,
) (string, error) {
	for _, c := range req.Criteria {
		if c.MinScore < 0 || c.MaxScore <= c.MinScore || c.Weight < 0 {
			return "", ErrInvalidCriteria
		}
	}

	roundID, err := s.repo.CreateRound(ctx, req)
	if err != nil {
		return "", err
	}

	s.recordAudit(
		ctx,
		audit.ActionJudgingRoundCreate,
		audit.TargetJudgingRound,
		&roundID,
		fmt.Sprintf("Judging round '%s' created by %s", req.Name, actorEmail),
		&actorID, &actorEmail, &actorRole,
		map[string]any{
			"round_name":         req.Name,
			"buffer_size":        req.BufferSize,
			"slot_duration_mins": req.SlotDurationMins,
			"criteria_count":     len(req.Criteria),
		},
		&ip, &userAgent,
	)

	return roundID, nil
}

// ActivateRound sets round status to active.
func (s *Service) ActivateRound(
	ctx context.Context,
	roundID string,
	actorID, actorEmail, actorRole string,
	ip, userAgent string,
) error {
	if err := s.repo.ActivateRound(ctx, roundID); err != nil {
		return err
	}

	s.recordAudit(
		ctx,
		audit.ActionJudgingRoundActivate,
		audit.TargetJudgingRound,
		&roundID,
		fmt.Sprintf("Judging round activated by %s", actorEmail),
		&actorID, &actorEmail, &actorRole,
		map[string]any{"round_id": roundID},
		&ip, &userAgent,
	)

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
func (s *Service) EndRound(
	ctx context.Context,
	roundID string,
	actorID, actorEmail, actorRole string,
	ip, userAgent string,
) error {
	if err := s.repo.EndRound(ctx, roundID); err != nil {
		return err
	}
	s.recordAudit(ctx, audit.ActionJudgingRoundEnd, audit.TargetJudgingRound, &roundID,
		fmt.Sprintf("Judging round ended by %s", actorEmail),
		&actorID, &actorEmail, &actorRole, map[string]any{"round_id": roundID}, &ip, &userAgent)
	return nil
}

// DeleteRound permanently deletes a round that is not active.
func (s *Service) DeleteRound(
	ctx context.Context,
	roundID string,
	actorID, actorEmail, actorRole string,
	ip, userAgent string,
) error {
	if err := s.repo.DeleteRound(ctx, roundID); err != nil {
		return err
	}
	s.recordAudit(ctx, audit.ActionJudgingRoundDelete, audit.TargetJudgingRound, &roundID,
		fmt.Sprintf("Judging round deleted by %s", actorEmail),
		&actorID, &actorEmail, &actorRole, map[string]any{"round_id": roundID}, &ip, &userAgent)
	return nil
}

// GetRoundPanels lists a round's panels and the teams each holds.
func (s *Service) GetRoundPanels(ctx context.Context, roundID string) ([]RoundPanelOverview, error) {
	return s.repo.GetRoundPanels(ctx, roundID)
}

// GetAssignedJudges lists judges already assigned to a panel in the round.
func (s *Service) GetAssignedJudges(ctx context.Context, roundID string) ([]AssignedJudge, error) {
	return s.repo.GetAssignedJudges(ctx, roundID)
}

// CreatePanel creates a panel in a room and assigns judges.
func (s *Service) CreatePanel(
	ctx context.Context,
	req CreatePanelRequest,
	actorID, actorEmail, actorRole string,
	ip, userAgent string,
) (string, error) {
	panelID, err := s.repo.CreatePanel(ctx, req)
	if err != nil {
		return "", err
	}

	s.recordAudit(
		ctx,
		audit.ActionJudgingRoundCreate,
		audit.TargetJudgingPanel,
		&panelID,
		fmt.Sprintf("Judging panel '%s' created in Room '%s' with %d judges", req.Name, req.Room, len(req.JudgeIDs)),
		&actorID, &actorEmail, &actorRole,
		map[string]any{
			"round_id":    req.RoundID,
			"panel_name":  req.Name,
			"room":        req.Room,
			"judge_count": len(req.JudgeIDs),
		},
		&ip, &userAgent,
	)

	return panelID, nil
}

// ConfigureSlots sets sequential team schedule in a panel.
func (s *Service) ConfigureSlots(
	ctx context.Context,
	panelID string,
	req ConfigureSlotsRequest,
	actorID, actorEmail, actorRole string,
	ip, userAgent string,
) error {
	if err := s.repo.ConfigureSlots(ctx, panelID, req.Slots, req.Append); err != nil {
		return err
	}

	s.recordAudit(
		ctx,
		audit.ActionJudgingRoundCreate,
		audit.TargetJudgingPanel,
		&panelID,
		fmt.Sprintf("Configured %d team slots in panel by %s", len(req.Slots), actorEmail),
		&actorID, &actorEmail, &actorRole,
		map[string]any{
			"panel_id":   panelID,
			"slot_count": len(req.Slots),
		},
		&ip, &userAgent,
	)

	return nil
}

// UpdateSlotLinks modifies external project URLs for a slot.
// If the caller is not an admin, verifies that the user belongs to the team assigned to this slot.
func (s *Service) UpdateSlotLinks(
	ctx context.Context,
	slotID string,
	actorID, actorRole string,
	links map[string]string,
) error {
	if actorRole != "admin" {
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
	return s.repo.UpdateSlotLinks(ctx, slotID, links)
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
func (s *Service) StartSlot(
	ctx context.Context,
	slotID string,
	actorID, actorEmail, actorRole string,
	ip, userAgent string,
) error {
	if err := s.authorizeSlotActor(ctx, slotID, actorID); err != nil {
		return err
	}

	if err := s.repo.StartSlot(ctx, slotID); err != nil {
		return err
	}

	s.recordAudit(
		ctx,
		audit.ActionJudgingSlotStart,
		audit.TargetJudgingSlot,
		&slotID,
		fmt.Sprintf("Judging slot pitch started by %s", actorEmail),
		&actorID, &actorEmail, &actorRole,
		map[string]any{"slot_id": slotID},
		&ip, &userAgent,
	)

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
func (s *Service) PauseTimer(
	ctx context.Context,
	slotID string,
	actorID, actorEmail, actorRole string,
	ip, userAgent string,
) error {
	if err := s.authorizeSlotActor(ctx, slotID, actorID); err != nil {
		return err
	}
	if err := s.repo.PauseTimer(ctx, slotID); err != nil {
		return err
	}
	s.recordAudit(ctx, audit.ActionJudgingTimerPause, audit.TargetJudgingSlot, &slotID,
		fmt.Sprintf("Pitch timer paused by %s", actorEmail),
		&actorID, &actorEmail, &actorRole, map[string]any{"slot_id": slotID}, &ip, &userAgent)
	return nil
}

// ResumeTimer resumes a paused pitch clock.
func (s *Service) ResumeTimer(
	ctx context.Context,
	slotID string,
	actorID, actorEmail, actorRole string,
	ip, userAgent string,
) error {
	if err := s.authorizeSlotActor(ctx, slotID, actorID); err != nil {
		return err
	}
	if err := s.repo.ResumeTimer(ctx, slotID); err != nil {
		return err
	}
	s.recordAudit(ctx, audit.ActionJudgingTimerResume, audit.TargetJudgingSlot, &slotID,
		fmt.Sprintf("Pitch timer resumed by %s", actorEmail),
		&actorID, &actorEmail, &actorRole, map[string]any{"slot_id": slotID}, &ip, &userAgent)
	return nil
}

// ExtendTimer adds extra time to a pitch.
func (s *Service) ExtendTimer(
	ctx context.Context,
	slotID string,
	actorID, actorEmail, actorRole string,
	seconds int,
	ip, userAgent string,
) error {
	if err := s.authorizeSlotActor(ctx, slotID, actorID); err != nil {
		return err
	}
	if err := s.repo.ExtendTimer(ctx, slotID, seconds); err != nil {
		return err
	}
	s.recordAudit(ctx, audit.ActionJudgingTimerExtend, audit.TargetJudgingSlot, &slotID,
		fmt.Sprintf("Pitch timer extended by %ds by %s", seconds, actorEmail),
		&actorID, &actorEmail, &actorRole, map[string]any{"slot_id": slotID, "seconds": seconds}, &ip, &userAgent)
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
func (s *Service) LockScores(
	ctx context.Context,
	slotID string,
	judgeID, judgeEmail, judgeRole string,
	ip, userAgent string,
) error {
	if err := s.authorizeSlotActor(ctx, slotID, judgeID); err != nil {
		return err
	}

	if err := s.repo.LockJudgeScorecard(ctx, slotID, judgeID); err != nil {
		return err
	}

	metrics.InstrumentScoreSubmission("locked")

	s.recordAudit(
		ctx,
		audit.ActionJudgingScoreLock,
		audit.TargetJudgingSlot,
		&slotID,
		fmt.Sprintf("Judge %s locked their evaluation scorecard", judgeEmail),
		&judgeID, &judgeEmail, &judgeRole,
		map[string]any{"slot_id": slotID},
		&ip, &userAgent,
	)

	return nil
}

// CompleteSlot validates all judges locked scores, then completes the slot and advances the queue.
func (s *Service) CompleteSlot(
	ctx context.Context,
	slotID string,
	actorID, actorEmail, actorRole string,
	ip, userAgent string,
) error {
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

	s.recordAudit(
		ctx,
		audit.ActionJudgingSlotComplete,
		audit.TargetJudgingSlot,
		&slotID,
		fmt.Sprintf("Judging slot completed and queue advanced by %s", actorEmail),
		&actorID, &actorEmail, &actorRole,
		map[string]any{"slot_id": slotID},
		&ip, &userAgent,
	)

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
	return s.repo.GetTeamActiveStatus(ctx, *teamID)
}

// GetTeamStatus returns the live queue position and room coordinates for a team.
func (s *Service) GetTeamStatus(ctx context.Context, teamID string) (*TeamStatusResponse, error) {
	return s.repo.GetTeamActiveStatus(ctx, teamID)
}

// GetVolunteerSchedule returns room coordinates, on-deck teams, and upcoming teams for coordinators.
// A non-nil judgeID restricts the schedule to that judge's assigned panels.
func (s *Service) GetVolunteerSchedule(ctx context.Context, judgeID *string) ([]VolunteerPanelSchedule, error) {
	return s.repo.GetVolunteerSchedule(ctx, judgeID)
}

// GetAdminLeaderboard calculates full round rankings with weighted criteria breakdown.
func (s *Service) GetAdminLeaderboard(ctx context.Context, roundID string) (*AdminLeaderboardResponse, error) {
	return s.repo.GetAdminLeaderboard(ctx, roundID)
}

// MoveSlot moves a team slot across panels or reorders inside a panel.
func (s *Service) MoveSlot(
	ctx context.Context,
	slotID string,
	req MoveSlotRequest,
	actorID, actorEmail, actorRole string,
	ip, userAgent string,
) error {
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

	s.recordAudit(
		ctx,
		audit.ActionJudgingSlotMove,
		audit.TargetJudgingSlot,
		&slotID,
		fmt.Sprintf("Judging slot moved by %s", actorEmail),
		&actorID, &actorEmail, &actorRole,
		metadata,
		&ip, &userAgent,
	)

	return nil
}

// QueueSlot immediately puts a team slot on-deck into the room queue ('queued' status).
func (s *Service) QueueSlot(
	ctx context.Context,
	slotID string,
	actorID, actorEmail, actorRole string,
	ip, userAgent string,
) error {
	if err := s.repo.QueueSlot(ctx, slotID); err != nil {
		return err
	}

	s.recordAudit(
		ctx,
		audit.ActionJudgingSlotQueue,
		audit.TargetJudgingSlot,
		&slotID,
		fmt.Sprintf("Team slot thrown into room queue by %s", actorEmail),
		&actorID, &actorEmail, &actorRole,
		map[string]any{"slot_id": slotID, "status": "queued"},
		&ip, &userAgent,
	)

	return nil
}

// UnqueueSlot reverts a queued slot back to scheduled.
func (s *Service) UnqueueSlot(
	ctx context.Context,
	slotID string,
	actorID, actorEmail, actorRole string,
	ip, userAgent string,
) error {
	if err := s.repo.UnqueueSlot(ctx, slotID); err != nil {
		return err
	}

	s.recordAudit(
		ctx,
		audit.ActionJudgingSlotMove,
		audit.TargetJudgingSlot,
		&slotID,
		fmt.Sprintf("Team slot removed from queue by %s", actorEmail),
		&actorID, &actorEmail, &actorRole,
		map[string]any{"slot_id": slotID, "status": "scheduled"},
		&ip, &userAgent,
	)

	return nil
}

func (s *Service) recordAudit(
	ctx context.Context,
	action, targetType string,
	targetID *string,
	desc string,
	actorID, actorEmail, actorRole *string,
	metadata map[string]any,
	ip, ua *string,
) {
	if s.auditQueue == nil {
		return
	}
	s.auditQueue.Record(ctx, audit.Event{
		ActorID:     actorID,
		ActorEmail:  actorEmail,
		ActorRole:   actorRole,
		Action:      action,
		TargetType:  targetType,
		TargetID:    targetID,
		Description: desc,
		Metadata:    metadata,
		IPAddress:   ip,
		UserAgent:   ua,
		Timestamp:   time.Now(),
	})
}
