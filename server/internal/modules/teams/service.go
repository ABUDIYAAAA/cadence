package teams

import (
	"context"
	"math"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreateTeam(ctx context.Context, req CreateTeamRequest) (*TeamDetailResponse, error) {
	return s.repo.CreateTeam(ctx, req.Name, req.LeaderID, req.InitialMemberIDs)
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
	return s.repo.UpdateTeam(ctx, id, req)
}

func (s *Service) DeleteTeam(ctx context.Context, id string) error {
	return s.repo.DeleteTeam(ctx, id)
}

func (s *Service) AddMember(ctx context.Context, teamID, userID string) error {
	return s.repo.AddMember(ctx, teamID, userID)
}

func (s *Service) RemoveMember(ctx context.Context, teamID, userID string) error {
	return s.repo.RemoveMember(ctx, teamID, userID)
}
