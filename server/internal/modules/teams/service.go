package teams

import (
	"context"
	"fmt"
	"math"

	"devclubxnst.com/cadence/internal/modules/auth"
	"devclubxnst.com/cadence/pkg/audit"
)

type Service struct {
	repo    *Repository
	auditor audit.Recorder
}

func NewService(repo *Repository, auditor audit.Recorder) *Service {
	return &Service{
		repo:    repo,
		auditor: auditor,
	}
}

// record writes an audit event attributed to the caller in ctx.
func (s *Service) record(ctx context.Context, action, teamID, description string, metadata map[string]any) {
	if s.auditor == nil {
		return
	}
	var actorID, actorEmail, actorRole *string
	if claims, _ := auth.GetUserClaims(ctx); claims != nil {
		actorID = &claims.UserID
		actorEmail = &claims.Email
		actorRole = &claims.Role
	}
	event := audit.Event{
		ActorID:     actorID,
		ActorEmail:  actorEmail,
		ActorRole:   actorRole,
		Action:      action,
		TargetType:  audit.TargetTeam,
		Description: description,
		Metadata:    metadata,
	}
	if teamID != "" {
		event.TargetID = &teamID
	}
	s.auditor.Record(ctx, event)
}

// ---- participant flows ----

func (s *Service) CreateTeam(ctx context.Context, userID string, req CreateTeamRequest) (*TeamDetailResponse, error) {
	teamID, err := s.repo.CreateTeam(ctx, userID, req.Name)
	if err != nil {
		return nil, err
	}
	s.record(ctx, audit.ActionTeamCreate, teamID, fmt.Sprintf("Created team %s", req.Name),
		map[string]any{"name": req.Name, "leader_id": userID})
	return s.repo.GetTeamByID(ctx, teamID)
}

func (s *Service) JoinTeam(ctx context.Context, userID string, req JoinTeamRequest) (*TeamDetailResponse, error) {
	teamID, err := s.repo.JoinByCode(ctx, userID, req.Code)
	if err != nil {
		return nil, err
	}
	s.record(ctx, audit.ActionTeamJoin, teamID, fmt.Sprintf("User %s joined team %s", userID, teamID),
		map[string]any{"user_id": userID})
	return s.repo.GetTeamByID(ctx, teamID)
}

func (s *Service) LeaveTeam(ctx context.Context, userID string) error {
	teamID, err := s.repo.LeaveTeam(ctx, userID)
	if err != nil {
		return err
	}
	s.record(ctx, audit.ActionTeamLeave, teamID, fmt.Sprintf("User %s left team %s", userID, teamID),
		map[string]any{"user_id": userID})
	return nil
}

func (s *Service) RenameOwnTeam(ctx context.Context, userID string, req RenameTeamRequest) (*TeamDetailResponse, error) {
	teamID, err := s.repo.RenameOwnTeam(ctx, userID, req.Name)
	if err != nil {
		return nil, err
	}
	s.record(ctx, audit.ActionTeamUpdate, teamID, fmt.Sprintf("Renamed team to %s", req.Name),
		map[string]any{"name": req.Name})
	return s.repo.GetTeamByID(ctx, teamID)
}

func (s *Service) RegenerateOwnCode(ctx context.Context, userID string) (*TeamDetailResponse, error) {
	teamID, err := s.repo.RegenerateOwnCode(ctx, userID)
	if err != nil {
		return nil, err
	}
	s.record(ctx, audit.ActionTeamRegenCode, teamID, "Regenerated team join code", nil)
	return s.repo.GetTeamByID(ctx, teamID)
}

func (s *Service) KickMember(ctx context.Context, leaderID, targetID string) (*TeamDetailResponse, error) {
	teamID, err := s.repo.KickMember(ctx, leaderID, targetID)
	if err != nil {
		return nil, err
	}
	s.record(ctx, audit.ActionTeamKick, teamID, fmt.Sprintf("Removed member %s from team %s", targetID, teamID),
		map[string]any{"user_id": targetID})
	return s.repo.GetTeamByID(ctx, teamID)
}

func (s *Service) GetMyTeam(ctx context.Context, userID string) (*TeamDetailResponse, error) {
	return s.repo.GetTeamByUserID(ctx, userID)
}

// ---- reads ----

func (s *Service) GetTeamByID(ctx context.Context, id string) (*TeamDetailResponse, error) {
	return s.repo.GetTeamByID(ctx, id)
}

// GetTeamForJudge returns a team only if the judge is assigned to a panel evaluating it.
// Unassigned teams are reported as not found so their existence isn't disclosed.
// The join code is withheld from judges.
func (s *Service) GetTeamForJudge(ctx context.Context, id, judgeID string) (*TeamDetailResponse, error) {
	ok, err := s.repo.IsTeamAssignedToJudge(ctx, id, judgeID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrTeamNotFound
	}
	team, err := s.repo.GetTeamByID(ctx, id)
	if err != nil {
		return nil, err
	}
	team.JoinCode = ""
	return team, nil
}

func (s *Service) ListTeams(ctx context.Context, filter TeamFilter) (*PaginatedTeamsResponse, error) {
	data, totalCount, err := s.repo.ListTeams(ctx, filter)
	if err != nil {
		return nil, err
	}
	if filter.JudgeID != nil {
		for i := range data {
			data[i].JoinCode = ""
		}
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

	return &PaginatedTeamsResponse{
		Data:       data,
		TotalCount: totalCount,
		Page:       page,
		Limit:      limit,
		TotalPages: int(math.Ceil(float64(totalCount) / float64(limit))),
	}, nil
}

// ---- staff flows ----

func (s *Service) UpdateTeam(ctx context.Context, id string, req UpdateTeamRequest) (*TeamDetailResponse, error) {
	if err := s.repo.UpdateTeam(ctx, id, req); err != nil {
		return nil, err
	}
	team, err := s.repo.GetTeamByID(ctx, id)
	if err != nil {
		return nil, err
	}
	s.record(ctx, audit.ActionTeamUpdate, id, fmt.Sprintf("Updated team %s", team.Name), map[string]any{
		"name": req.Name, "leader_id": req.LeaderID, "max_size": req.MaxSize,
	})
	return team, nil
}

func (s *Service) DeleteTeam(ctx context.Context, id string) error {
	team, _ := s.repo.GetTeamByID(ctx, id)
	if err := s.repo.DeleteTeam(ctx, id); err != nil {
		return err
	}
	name := id
	if team != nil {
		name = team.Name
	}
	s.record(ctx, audit.ActionTeamDelete, id, fmt.Sprintf("Deleted team %s and unassigned members", name), nil)
	return nil
}

func (s *Service) AddMember(ctx context.Context, teamID, userID string) error {
	if err := s.repo.AddMember(ctx, teamID, userID); err != nil {
		return err
	}
	s.record(ctx, audit.ActionTeamAddMember, teamID, fmt.Sprintf("Added member %s to team %s", userID, teamID),
		map[string]any{"user_id": userID, "team_id": teamID})
	return nil
}

func (s *Service) RemoveMember(ctx context.Context, teamID, userID string) error {
	if err := s.repo.RemoveMember(ctx, teamID, userID); err != nil {
		return err
	}
	s.record(ctx, audit.ActionTeamRemoveMember, teamID, fmt.Sprintf("Removed member %s from team %s", userID, teamID),
		map[string]any{"user_id": userID, "team_id": teamID})
	return nil
}

func (s *Service) RegenerateCode(ctx context.Context, id string) (*TeamDetailResponse, error) {
	if err := s.repo.RegenerateCode(ctx, id); err != nil {
		return nil, err
	}
	s.record(ctx, audit.ActionTeamRegenCode, id, "Regenerated team join code", nil)
	return s.repo.GetTeamByID(ctx, id)
}

func (s *Service) SetFrozen(ctx context.Context, id string, frozen bool) (*TeamDetailResponse, error) {
	if err := s.repo.SetFrozen(ctx, id, frozen); err != nil {
		return nil, err
	}
	action, verb := audit.ActionTeamFreeze, "Froze"
	if !frozen {
		action, verb = audit.ActionTeamUnfreeze, "Unfroze"
	}
	s.record(ctx, action, id, fmt.Sprintf("%s team %s", verb, id), nil)
	return s.repo.GetTeamByID(ctx, id)
}

func (s *Service) SetFrozenAll(ctx context.Context, frozen bool) (*SettingsResponse, error) {
	actor := ""
	if claims, _ := auth.GetUserClaims(ctx); claims != nil {
		actor = claims.UserID
	}
	if err := s.repo.SetFrozenAll(ctx, frozen, actor); err != nil {
		return nil, err
	}
	action, desc := audit.ActionTeamFreezeAll, "Froze all teams"
	if !frozen {
		action, desc = audit.ActionTeamUnfreezeAll, "Unfroze all teams"
	}
	s.record(ctx, action, "", desc, nil)
	return s.GetSettings(ctx)
}

func (s *Service) GetSettings(ctx context.Context) (*SettingsResponse, error) {
	settings, err := s.repo.GetSettings(ctx)
	if err != nil {
		return nil, err
	}
	return &settings, nil
}

func (s *Service) UpdateSettings(ctx context.Context, req UpdateSettingsRequest) (*SettingsResponse, error) {
	if req.MaxTeamSize != nil {
		actor := ""
		if claims, _ := auth.GetUserClaims(ctx); claims != nil {
			actor = claims.UserID
		}
		if err := s.repo.SetMaxTeamSize(ctx, *req.MaxTeamSize, actor); err != nil {
			return nil, err
		}
		s.record(ctx, audit.ActionTeamSettings, "", fmt.Sprintf("Set max team size to %d", *req.MaxTeamSize),
			map[string]any{"max_team_size": *req.MaxTeamSize})
	}
	return s.GetSettings(ctx)
}
