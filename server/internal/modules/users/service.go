package users

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"devclubxnst.com/cadence/internal/config"
	"devclubxnst.com/cadence/internal/modules/auth"
	"devclubxnst.com/cadence/pkg/audit"
	"devclubxnst.com/cadence/pkg/utils"
	"github.com/redis/go-redis/v9"
)

var (
	ErrPasswordAlreadyConfigured = errors.New("user has already configured their password")
	ErrRoleNotPermitted          = errors.New("volunteers can only add participants")
)

type Service struct {
	repo        *Repository
	cfg         *config.Config
	emailSender auth.EmailSender
	redis       *redis.Client
	auditor     audit.Recorder
}

func NewService(
	repo *Repository,
	cfg *config.Config,
	emailSender auth.EmailSender,
	redis *redis.Client,
	auditor audit.Recorder,
) *Service {
	return &Service{
		repo:        repo,
		cfg:         cfg,
		emailSender: emailSender,
		redis:       redis,
		auditor:     auditor,
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

	for i, u := range req.Users {
		role := u.Role
		if role == "" {
			role = "participant"
		}

		// Savepoint per row: any single-row DB error (bad enum, length, etc.) must not abort the whole batch.
		sp, spErr := tx.Begin(ctx)
		if spErr != nil {
			return nil, fmt.Errorf("begin row savepoint: %w", spErr)
		}
		userID, createErr := s.repo.CreateUserTx(ctx, sp, u.Email, u.Name, role, u.PhoneNumber)
		if createErr != nil {
			_ = sp.Rollback(ctx)
		} else if cErr := sp.Commit(ctx); cErr != nil {
			createErr = fmt.Errorf("release row savepoint: %w", cErr)
		}
		if createErr != nil {
			resp.SkippedCount++
			resp.SkippedErrors = append(resp.SkippedErrors, fmt.Sprintf("row %d (%s): %v", i+1, u.Email, createErr))
			continue
		}

		resp.TotalCreated++
		resp.Users = append(resp.Users, UserDetailResponse{
			ID:                   userID,
			Email:                u.Email,
			Name:                 u.Name,
			PhoneNumber:          u.PhoneNumber,
			Role:                 role,
			IsPasswordConfigured: false,
			IsActive:             true,
			CreatedAt:            time.Now(),
		})
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit bulk import transaction: %w", err)
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
			Action:      audit.ActionUserBulkImport,
			TargetType:  audit.TargetUser,
			Description: fmt.Sprintf("Bulk import completed: %d created, %d skipped", resp.TotalCreated, resp.SkippedCount),
			Metadata: map[string]any{
				"total_processed": resp.TotalProcessed,
				"total_created":   resp.TotalCreated,
				"skipped_count":   resp.SkippedCount,
			},
		})
	}

	return resp, nil
}

func (s *Service) CreateUser(ctx context.Context, req CreateUserRequest) (*UserDetailResponse, error) {
	// Volunteers add participants (for example, someone who registered with a wrong email); nothing else.
	if volunteerCaller(ctx) {
		if req.Role != "" && req.Role != "participant" {
			return nil, ErrRoleNotPermitted
		}
		req.Role = "participant"
	}
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

	user, err := s.repo.GetUserByID(ctx, bulkResp.Users[0].ID)
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
			Action:      audit.ActionUserCreate,
			TargetType:  audit.TargetUser,
			TargetID:    &user.ID,
			Description: fmt.Sprintf("Created user %s (%s)", user.Email, user.Role),
			Metadata: map[string]any{
				"email": user.Email,
				"role":  user.Role,
			},
		})
	}

	return user, err
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

// UpdateUser modifies user attributes. When role or active status changes, it synchronizes
// the state into Redis with monotonic version progression and deactivation clearing.
func (s *Service) UpdateUser(ctx context.Context, id string, req UpdateUserRequest) (*UserDetailResponse, error) {
	user, err := s.repo.UpdateUser(ctx, id, req)
	if err != nil {
		return nil, err
	}

	// Synchronize revocation state into Redis after DB commit
	if s.redis != nil && (req.Role != nil || req.IsActive != nil) {
		ttl := 24 * time.Hour
		if s.cfg != nil && s.cfg.JWTAccessExpiry > 0 {
			ttl = s.cfg.JWTAccessExpiry * 2
		}

		if req.IsActive != nil {
			if !*req.IsActive {
				// Deactivation: block user immediately
				_ = s.redis.Set(ctx, fmt.Sprintf("blocked:user:%s", id), "deactivated", ttl).Err()
			} else {
				// Reactivation: clear the deactivation block key so user can log in again.
				// Their old tokens remain revoked because token_version was incremented in the DB.
				_ = s.redis.Del(ctx, fmt.Sprintf("blocked:user:%s", id)).Err()
			}
		}

		// For any role change or status change, monotonically update min_token_version in Redis.
		// Uses atomic string comparison/check so a delayed write can never decrement the version.
		updateMinTokenVersion(ctx, s.redis, id, user.TokenVersion, ttl)
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
			Action:      audit.ActionUserUpdate,
			TargetType:  audit.TargetUser,
			TargetID:    &id,
			Description: fmt.Sprintf("Updated user profile %s", user.Email),
		})
	}
	return user, nil
}

func updateMinTokenVersion(ctx context.Context, rdb *redis.Client, userID string, newVersion int, ttl time.Duration) {
	key := fmt.Sprintf("user:min_token_version:%s", userID)
	// Ensure monotonic update: if existing version >= newVersion, do not overwrite.
	// Watch the key in a simple Redis transaction.
	for attempts := 0; attempts < 3; attempts++ {
		err := rdb.Watch(ctx, func(tx *redis.Tx) error {
			currentVal, getErr := tx.Get(ctx, key).Int()
			if getErr != nil && !errors.Is(getErr, redis.Nil) {
				return getErr
			}
			if getErr == nil && currentVal >= newVersion {
				return nil // existing version is already higher or equal
			}

			_, pipeErr := tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
				pipe.Set(ctx, key, fmt.Sprintf("%d", newVersion), ttl)
				return nil
			})
			return pipeErr
		}, key)

		if err == nil {
			return
		}
		if errors.Is(err, redis.TxFailedErr) {
			continue // retry optimistic lock
		}
		break
	}
}

func (s *Service) DeleteUser(ctx context.Context, id string) error {
	user, _ := s.repo.GetUserByID(ctx, id)
	if err := s.repo.SoftDeleteUser(ctx, id); err != nil {
		return err
	}

	// Write instant revocation block key into Redis with 24-hour TTL (<1ms access-token lockout)
	if s.redis != nil {
		blockedKey := fmt.Sprintf("blocked:user:%s", id)
		_ = s.redis.Set(ctx, blockedKey, "deleted", 24*time.Hour).Err()
	}

	if s.auditor != nil {
		actorClaims, _ := auth.GetUserClaims(ctx)
		var actorID, actorEmail, actorRole *string
		if actorClaims != nil {
			actorID = &actorClaims.UserID
			actorEmail = &actorClaims.Email
			actorRole = &actorClaims.Role
		}
		email := id
		if user != nil {
			email = user.Email
		}
		s.auditor.Record(ctx, audit.Event{
			ActorID:     actorID,
			ActorEmail:  actorEmail,
			ActorRole:   actorRole,
			Action:      audit.ActionUserDelete,
			TargetType:  audit.TargetUser,
			TargetID:    &id,
			Description: fmt.Sprintf("Administrator removed and blocked user %s", email),
		})
	}

	return nil
}

// RestoreUser reactivates a removed user and lifts the instant-revocation block.
func (s *Service) RestoreUser(ctx context.Context, id string) error {
	if err := s.repo.RestoreUser(ctx, id); err != nil {
		return err
	}

	if s.redis != nil {
		_ = s.redis.Del(ctx, fmt.Sprintf("blocked:user:%s", id)).Err()
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
			Action:      audit.ActionUserUpdate,
			TargetType:  audit.TargetUser,
			TargetID:    &id,
			Description: "Administrator reactivated a removed user",
		})
	}
	return nil
}

// volunteerCaller reports whether the caller in ctx is a volunteer, who may only add participants.
func volunteerCaller(ctx context.Context) bool {
	claims, ok := auth.GetUserClaims(ctx)
	return ok && claims.Role == "volunteer"
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

	if s.emailSender != nil {
		resetURL := fmt.Sprintf("%s/auth/reset-password?token=%s", s.cfg.FrontendURL, rawToken)
		_ = s.emailSender.SendResetPasswordEmail(ctx, user.Email, user.Name, resetURL, 1)
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
			Action:      audit.ActionUserResetTrigger,
			TargetType:  audit.TargetUser,
			TargetID:    &user.ID,
			Description: fmt.Sprintf("Administrator triggered password recovery email for %s", user.Email),
		})
	}

	return nil
}
