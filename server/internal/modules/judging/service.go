package judging

import (
	"context"
	"errors"
	"fmt"
	"time"

	"devclubxnst.com/cadence/pkg/audit"
)

var (
	ErrNotAllJudgesLocked = errors.New("cannot complete slot: not all judges on this panel have locked their scorecards")
	ErrInvalidCriteria    = errors.New("each criterion must have a positive max score and weight")
	ErrScoreOutOfRange    = errors.New("submitted score exceeds criterion maximum allowed score")
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
		if c.MaxScore <= 0 || c.Weight < 0 {
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
	if err := s.repo.UpdateRoundStatus(ctx, roundID, "active"); err != nil {
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
	if err := s.repo.ConfigureSlots(ctx, panelID, req.Slots); err != nil {
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
func (s *Service) UpdateSlotLinks(ctx context.Context, slotID string, links map[string]string) error {
	return s.repo.UpdateSlotLinks(ctx, slotID, links)
}

// GetJudgePanels returns panels and queues assigned to this judge.
func (s *Service) GetJudgePanels(ctx context.Context, judgeID string) ([]JudgePanelSummary, error) {
	return s.repo.GetJudgePanels(ctx, judgeID)
}

// GetJudgeSlotDetails retrieves full evaluation view for a single slot, with counter info.
func (s *Service) GetJudgeSlotDetails(ctx context.Context, slotID string, judgeID string) (*JudgeSlotDetails, error) {
	return s.repo.GetJudgeSlotDetails(ctx, slotID, judgeID)
}

// StartSlot starts a team's pitch session and triggers on-deck queue shifts.
func (s *Service) StartSlot(
	ctx context.Context,
	slotID string,
	actorID, actorEmail, actorRole string,
	ip, userAgent string,
) error {
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

// SubmitScores records or updates criteria scores for a judge.
func (s *Service) SubmitScores(
	ctx context.Context,
	slotID string,
	judgeID string,
	req SubmitScoresRequest,
) error {
	return s.repo.UpsertScores(ctx, slotID, judgeID, req.Scores)
}

// LockScores locks the judge's score for the team.
func (s *Service) LockScores(
	ctx context.Context,
	slotID string,
	judgeID, judgeEmail, judgeRole string,
	ip, userAgent string,
) error {
	if err := s.repo.LockJudgeScorecard(ctx, slotID, judgeID); err != nil {
		return err
	}

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
func (s *Service) GetVolunteerSchedule(ctx context.Context) ([]VolunteerPanelSchedule, error) {
	return s.repo.GetVolunteerSchedule(ctx)
}

// GetAdminLeaderboard calculates full round rankings with weighted criteria breakdown.
func (s *Service) GetAdminLeaderboard(ctx context.Context, roundID string) (*AdminLeaderboardResponse, error) {
	return s.repo.GetAdminLeaderboard(ctx, roundID)
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
