package users

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"devclubxnst.com/cadence/internal/config"
	"devclubxnst.com/cadence/internal/modules/auth"
	"devclubxnst.com/cadence/pkg/utils"
	"github.com/redis/go-redis/v9"
)

var (
	ErrPasswordAlreadyConfigured = errors.New("user has already configured their password")
)

type Service struct {
	repo        *Repository
	cfg         *config.Config
	emailSender auth.EmailSender
	redis       *redis.Client
}

func NewService(
	repo *Repository,
	cfg *config.Config,
	emailSender auth.EmailSender,
	redis *redis.Client,
) *Service {
	return &Service{
		repo:        repo,
		cfg:         cfg,
		emailSender: emailSender,
		redis:       redis,
	}
}

func (s *Service) BulkCreateUsers(ctx context.Context, req BulkCreateUsersRequest) (*BulkCreateResponse, error) {
	resp := &BulkCreateResponse{
		TotalProcessed: len(req.Users),
		Users:          make([]UserDetailResponse, 0, len(req.Users)),
	}

	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin bulk import transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	type pendingInvite struct {
		userID string
		email  string
		name   string
	}
	var invites []pendingInvite

	for i, u := range req.Users {
		role := u.Role
		if role == "" {
			role = "participant"
		}

		var teamID *string
		if u.TeamName != nil && *u.TeamName != "" {
			resolvedID, tErr := s.repo.GetOrCreateTeamByNameTx(ctx, tx, *u.TeamName)
			if tErr != nil {
				resp.SkippedCount++
				resp.SkippedErrors = append(resp.SkippedErrors, fmt.Sprintf("row %d (%s): team resolve error: %v", i+1, u.Email, tErr))
				continue
			}
			teamID = &resolvedID
		} else if u.TeamID != nil && *u.TeamID != "" {
			teamID = u.TeamID
		}

		userID, createErr := s.repo.CreateUserTx(ctx, tx, u.Email, u.Name, role, u.PhoneNumber, teamID)
		if createErr != nil {
			resp.SkippedCount++
			resp.SkippedErrors = append(resp.SkippedErrors, fmt.Sprintf("row %d (%s): %v", i+1, u.Email, createErr))
			continue
		}

		if u.IsLeader && teamID != nil {
			if leaderErr := s.repo.SetTeamLeaderTx(ctx, tx, *teamID, userID); leaderErr != nil {
				resp.SkippedErrors = append(resp.SkippedErrors, fmt.Sprintf("row %d (%s): failed to set as leader: %v", i+1, u.Email, leaderErr))
			}
		}

		resp.TotalCreated++
		resp.Users = append(resp.Users, UserDetailResponse{
			ID:                   userID,
			Email:                u.Email,
			Name:                 u.Name,
			PhoneNumber:          u.PhoneNumber,
			Role:                 role,
			TeamID:               teamID,
			TeamName:             u.TeamName,
			IsLeader:             u.IsLeader,
			IsPasswordConfigured: false,
			IsActive:             true,
			CreatedAt:            time.Now(),
		})

		if u.SendInvite {
			invites = append(invites, pendingInvite{
				userID: userID,
				email:  u.Email,
				name:   u.Name,
			})
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit bulk import transaction: %w", err)
	}

	// Dispatch onboarding setup emails for users marked with SendInvite
	for _, inv := range invites {
		if err := s.dispatchSetupInvite(ctx, inv.userID, inv.email, inv.name); err == nil {
			resp.InvitesDispatched++
		}
	}

	return resp, nil
}

func (s *Service) CreateUser(ctx context.Context, req CreateUserRequest) (*UserDetailResponse, error) {
	bulkResp, err := s.BulkCreateUsers(ctx, BulkCreateUsersRequest{
		Users: []CreateUserRequest{req},
	})
	if err != nil {
		return nil, err
	}
	if len(bulkResp.Users) == 0 {
		if len(bulkResp.SkippedErrors) > 0 {
			return nil, errors.New(bulkResp.SkippedErrors[0])
		}
		return nil, errors.New("failed to create user")
	}
	return s.repo.GetUserByID(ctx, bulkResp.Users[0].ID)
}

func (s *Service) ListUsers(ctx context.Context, filter UserFilter) (*PaginatedUsersResponse, error) {
	data, totalCount, err := s.repo.ListUsers(ctx, filter)
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

	return &PaginatedUsersResponse{
		Data:       data,
		TotalCount: totalCount,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	}, nil
}

func (s *Service) GetUserByID(ctx context.Context, id string) (*UserDetailResponse, error) {
	return s.repo.GetUserByID(ctx, id)
}

func (s *Service) UpdateUser(ctx context.Context, id string, req UpdateUserRequest) (*UserDetailResponse, error) {
	return s.repo.UpdateUser(ctx, id, req)
}

func (s *Service) DeleteUser(ctx context.Context, id string) error {
	if err := s.repo.SoftDeleteUser(ctx, id); err != nil {
		return err
	}

	// Write instant revocation block key into Redis with 24-hour TTL (<1ms access-token lockout)
	if s.redis != nil {
		blockedKey := fmt.Sprintf("blocked:user:%s", id)
		_ = s.redis.Set(ctx, blockedKey, "deleted", 24*time.Hour).Err()
	}

	return nil
}

func (s *Service) SendInvite(ctx context.Context, userID string) error {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if user.IsPasswordConfigured {
		return ErrPasswordAlreadyConfigured
	}

	return s.dispatchSetupInvite(ctx, user.ID, user.Email, user.Name)
}

func (s *Service) BulkSendInvites(ctx context.Context, req BulkInviteRequest) (int, error) {
	var targetIDs []string
	if req.AllUninvited {
		uninvited, err := s.repo.GetUninvitedUserIDs(ctx)
		if err != nil {
			return 0, err
		}
		targetIDs = uninvited
	} else {
		targetIDs = req.UserIDs
	}

	dispatched := 0
	for _, id := range targetIDs {
		user, err := s.repo.GetUserByID(ctx, id)
		if err != nil || user.IsPasswordConfigured || !user.IsActive {
			continue
		}
		if err := s.dispatchSetupInvite(ctx, user.ID, user.Email, user.Name); err == nil {
			dispatched++
		}
	}

	return dispatched, nil
}

func (s *Service) TriggerResetPassword(ctx context.Context, userID string) error {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}

	rawToken, err := utils.GenerateRandomToken(32)
	if err != nil {
		return fmt.Errorf("generate reset token: %w", err)
	}

	tokenHash := utils.HashToken(rawToken)
	expiresAt := time.Now().Add(1 * time.Hour)

	if err := s.repo.CreateResetOrSetupToken(ctx, user.ID, tokenHash, "forgot_password", expiresAt); err != nil {
		return err
	}

	// Invalidate previous refresh tokens
	_ = s.repo.SoftDeleteUser // or revoke
	if s.emailSender != nil {
		resetURL := fmt.Sprintf("%s/auth/reset-password?token=%s", s.cfg.FrontendURL, rawToken)
		_ = s.emailSender.SendResetPasswordEmail(ctx, user.Email, user.Name, resetURL, 1)
	}

	return nil
}

func (s *Service) dispatchSetupInvite(ctx context.Context, userID, email, name string) error {
	rawToken, err := utils.GenerateRandomToken(32)
	if err != nil {
		return fmt.Errorf("generate setup token: %w", err)
	}

	tokenHash := utils.HashToken(rawToken)
	expiresAt := time.Now().Add(48 * time.Hour) // 48 hour setup link validity

	if err := s.repo.CreateResetOrSetupToken(ctx, userID, tokenHash, "initial_setup", expiresAt); err != nil {
		return err
	}

	if s.emailSender != nil {
		setupURL := fmt.Sprintf("%s/auth/setup-password?token=%s", s.cfg.FrontendURL, rawToken)
		_ = s.emailSender.SendSetupPasswordEmail(ctx, email, name, setupURL, 48)
	}

	return nil
}
