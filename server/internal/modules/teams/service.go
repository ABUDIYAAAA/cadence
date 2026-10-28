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

func (s *Service) CreateTeam(ctx context.Context, req CreateTeamRequest) (*TeamDetailResponse, error) {
	team, err := s.repo.CreateTeam(ctx, req.Name, req.LeaderID, req.InitialMemberIDs)
	if err == nil && s.auditor != nil {
		actorClaims, _ := auth.GetUserClaims(ctx)
		var actorID, actorEmail, actorRole *string
		if actorClaims != nil {
			actorID = &actorClaims.UserID
			actorEmail = &actorClaims.Email
			actorRole = &actorClaims.Role
		}
		s.auditor.Record(ctx, audit.Event{
			ActorID:     actorID,
			ActorEmail:  actorEmail,
			ActorRole:   actorRole,
			Action:      audit.ActionTeamCreate,
			TargetType:  audit.TargetTeam,
			TargetID:    &team.ID,
			Description: fmt.Sprintf("Created team %s", team.Name),
			Metadata: map[string]any{
				"name":        team.Name,
				"leader_id":   req.LeaderID,
				"member_count": len(team.Members),
			},
		})
	}
	return team, err
}

func (s *Service) GetTeamByID(ctx context.Context, id string) (*TeamDetailResponse, error) {
	return s.repo.GetTeamByID(ctx, id)
}

func (s *Service) GetMyTeam(ctx context.Context, userID string) (*TeamDetailResponse, error) {
	return s.repo.GetTeamByUserID(ctx, userID)
}

func (s *Service) ListTeams(ctx context.Context, filter TeamFilter) (*PaginatedTeamsResponse, error) {
	data, totalCount, err := s.repo.ListTeams(ctx, filter)
	if err != nil {
		return nil, err
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	page := filter.Page
	if page <= 0 {
		page = 1
	}

	totalPages := int(math.Ceil(float64(totalCount) / float64(limit)))

	return &PaginatedTeamsResponse{
		Data:       data,
		TotalCount: totalCount,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	}, nil
}

func (s *Service) UpdateTeam(ctx context.Context, id string, req UpdateTeamRequest) (*TeamDetailResponse, error) {
	team, err := s.repo.UpdateTeam(ctx, id, req)
	if err == nil && s.auditor != nil {
		actorClaims, _ := auth.GetUserClaims(ctx)
		var actorID, actorEmail, actorRole *string
		if actorClaims != nil {
			actorID = &actorClaims.UserID
			actorEmail = &actorClaims.Email
			actorRole = &actorClaims.Role
		}
		s.auditor.Record(ctx, audit.Event{
			ActorID:     actorID,
			ActorEmail:  actorEmail,
			ActorRole:   actorRole,
			Action:      audit.ActionTeamUpdate,
			TargetType:  audit.TargetTeam,
			TargetID:    &id,
			Description: fmt.Sprintf("Updated team %s", team.Name),
		})
	}
	return team, err
}

func (s *Service) DeleteTeam(ctx context.Context, id string) error {
	team, _ := s.repo.GetTeamByID(ctx, id)
	if err := s.repo.DeleteTeam(ctx, id); err != nil {
		return err
	}

	if s.auditor != nil {
		actorClaims, _ := auth.GetUserClaims(ctx)
		var actorID, actorEmail, actorRole *string
		if actorClaims != nil {
			actorID = &actorClaims.UserID
			actorEmail = &actorClaims.Email
			actorRole = &actorClaims.Role
		}
		teamName := id
		if team != nil {
			teamName = team.Name
		}
		s.auditor.Record(ctx, audit.Event{
			ActorID:     actorID,
			ActorEmail:  actorEmail,
			ActorRole:   actorRole,
			Action:      audit.ActionTeamDelete,
			TargetType:  audit.TargetTeam,
			TargetID:    &id,
			Description: fmt.Sprintf("Deleted team %s and unassigned members", teamName),
		})
	}
	return nil
}

func (s *Service) AddMember(ctx context.Context, teamID, userID string) error {
	if err := s.repo.AddMember(ctx, teamID, userID); err != nil {
		return err
	}

	if s.auditor != nil {
		actorClaims, _ := auth.GetUserClaims(ctx)
		var actorID, actorEmail, actorRole *string
		if actorClaims != nil {
			actorID = &actorClaims.UserID
			actorEmail = &actorClaims.Email
			actorRole = &actorClaims.Role
		}
		s.auditor.Record(ctx, audit.Event{
			ActorID:     actorID,
			ActorEmail:  actorEmail,
			ActorRole:   actorRole,
			Action:      audit.ActionTeamAddMember,
			TargetType:  audit.TargetTeam,
			TargetID:    &teamID,
			Description: fmt.Sprintf("Added member %s to team %s", userID, teamID),
			Metadata:    map[string]any{"user_id": userID, "team_id": teamID},
		})
	}
	return nil
}

func (s *Service) RemoveMember(ctx context.Context, teamID, userID string) error {
	if err := s.repo.RemoveMember(ctx, teamID, userID); err != nil {
		return err
	}

	if s.auditor != nil {
		actorClaims, _ := auth.GetUserClaims(ctx)
		var actorID, actorEmail, actorRole *string
		if actorClaims != nil {
			actorID = &actorClaims.UserID
			actorEmail = &actorClaims.Email
			actorRole = &actorClaims.Role
		}
		s.auditor.Record(ctx, audit.Event{
			ActorID:     actorID,
			ActorEmail:  actorEmail,
			ActorRole:   actorRole,
			Action:      audit.ActionTeamRemoveMember,
			TargetType:  audit.TargetTeam,
			TargetID:    &teamID,
			Description: fmt.Sprintf("Removed member %s from team %s", userID, teamID),
			Metadata:    map[string]any{"user_id": userID, "team_id": teamID},
		})
	}
	return nil
}
