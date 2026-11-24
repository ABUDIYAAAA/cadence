package competitions

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"devclubxnst.com/cadence/pkg/audit"
	"github.com/redis/go-redis/v9"
)

func isValidEmail(email string) bool {
	email = strings.TrimSpace(email)
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return false
	}
	parts := strings.Split(email, "@")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return false
	}
	return strings.Contains(parts[1], ".")
}

var (
	slugRegex           = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	ErrInvalidSlug      = errors.New("slug must contain only lowercase letters, numbers, and single hyphens")
	ErrPermissionDenied = errors.New("insufficient permissions to perform this role management action")
	ErrCannotSelfDemote = errors.New("owner cannot demote themselves; transfer ownership instead")
	ErrCannotModifyOwner= errors.New("cannot change the role of the competition owner; transfer ownership instead")
)

type Service struct {
	repo    *Repository
	rdb     *redis.Client
	auditor audit.Recorder
}

func NewService(repo *Repository, rdb *redis.Client, auditor audit.Recorder) *Service {
	return &Service{
		repo:    repo,
		rdb:     rdb,
		auditor: auditor,
	}
}

// CreateCompetition validates inputs, ensures slug format, and binds the owner.
func (s *Service) CreateCompetition(ctx context.Context, req CreateCompetitionRequest, creatorUserID string) (*Competition, error) {
	if !slugRegex.MatchString(req.Slug) {
		return nil, ErrInvalidSlug
	}

	ownerUserID := creatorUserID
	if req.OwnerID != nil && *req.OwnerID != "" {
		ownerUserID = *req.OwnerID
	} else if req.OwnerEmail != nil && *req.OwnerEmail != "" {
		var ownerName string
		if req.OwnerName != nil {
			ownerName = *req.OwnerName
		}
		resolvedID, err := s.repo.GetOrCreateUserByEmail(ctx, *req.OwnerEmail, ownerName)
		if err != nil {
			return nil, err
		}
		ownerUserID = resolvedID
	}

	comp, err := s.repo.CreateCompetition(ctx, req.Slug, req.Name, req.Description, ownerUserID)
	if err != nil {
		return nil, err
	}

	if s.rdb != nil {
		_ = s.rdb.Set(ctx, fmt.Sprintf("comp:%s:member:%s", comp.ID, ownerUserID), "owner", 15*time.Minute).Err()
		_ = s.rdb.SAdd(ctx, fmt.Sprintf("user:%s:competitions", ownerUserID), comp.ID).Err()
	}

	s.recordAudit(ctx, audit.ActionCompetitionCreate, comp.ID, fmt.Sprintf("Created competition %s (%s)", comp.Name, comp.Slug), &creatorUserID, map[string]any{
		"slug":          comp.Slug,
		"name":          comp.Name,
		"owner_user_id": ownerUserID,
	})

	return comp, nil
}

// GetCompetitionBySlug retrieves competition metadata.
func (s *Service) GetCompetitionBySlug(ctx context.Context, slug string) (*Competition, error) {
	return s.repo.GetCompetitionBySlug(ctx, slug)
}

// UpdateCompetition updates competition name and description and purges Redis cache.
func (s *Service) UpdateCompetition(ctx context.Context, compID string, req UpdateCompetitionRequest, actorID string) (*Competition, error) {
	comp, err := s.repo.UpdateCompetition(ctx, compID, req.Name, req.Description)
	if err != nil {
		return nil, err
	}

	// Invalidate cache
	if s.rdb != nil {
		_ = s.rdb.Del(ctx, "comp:slug:"+comp.Slug).Err()
	}

	s.recordAudit(ctx, audit.ActionCompetitionUpdate, comp.ID, fmt.Sprintf("Updated competition %s", comp.Slug), &actorID, map[string]any{
		"name": req.Name,
	})

	return comp, nil
}

// TransferOwnership transfers the owner role to another member.
// Only the current owner can perform this. The current owner becomes admin.
func (s *Service) TransferOwnership(ctx context.Context, compID, currentOwnerID string, req TransferOwnershipRequest) error {
	newOwnerID := req.NewOwnerUserID
	if newOwnerID == "" && req.NewOwnerEmail != nil && *req.NewOwnerEmail != "" {
		var name string
		if req.NewOwnerName != nil {
			name = *req.NewOwnerName
		}
		resolvedID, err := s.repo.GetOrCreateUserByEmail(ctx, *req.NewOwnerEmail, name)
		if err != nil {
			return err
		}
		newOwnerID = resolvedID
	}
	if newOwnerID == "" {
		return errors.New("either new_owner_user_id or new_owner_email must be provided")
	}

	if currentOwnerID == newOwnerID {
		return nil
	}

	if err := s.repo.TransferOwnership(ctx, compID, currentOwnerID, newOwnerID); err != nil {
		return err
	}

	// Invalidate cached member roles in Redis immediately and register active competition
	if s.rdb != nil {
		_ = s.rdb.Del(ctx,
			fmt.Sprintf("comp:%s:member:%s", compID, currentOwnerID),
			fmt.Sprintf("comp:%s:member:%s", compID, newOwnerID),
		).Err()
		_ = s.rdb.SAdd(ctx, fmt.Sprintf("user:%s:competitions", newOwnerID), compID).Err()
	}

	s.recordAudit(ctx, audit.ActionCompetitionTransferOwnership, compID, fmt.Sprintf("Transferred competition ownership from %s to %s", currentOwnerID, newOwnerID), &currentOwnerID, map[string]any{
		"new_owner_user_id": newOwnerID,
	})

	return nil
}

// AddCompetitionUser adds a single user to a competition by email, creating a background account if needed.
func (s *Service) AddCompetitionUser(
	ctx context.Context,
	compID, actorID, actorRole string,
	req AddCompetitionUserRequest,
) (*CompetitionUserDetailResponse, bool, error) {
	if !isValidEmail(req.Email) {
		return nil, false, errors.New("invalid email address")
	}

	role := req.Role
	if role == "" {
		role = "participant"
	}

	detail, createdAccount, err := s.repo.AddCompetitionUser(ctx, compID, actorID, actorRole, req.Email, req.Name, req.PhoneNumber, role)
	if err != nil {
		return nil, false, err
	}

	// Update Redis cache immediately
	if s.rdb != nil {
		_ = s.rdb.Set(ctx, fmt.Sprintf("comp:%s:member:%s", compID, detail.ID), role, 15*time.Minute).Err()
		_ = s.rdb.SAdd(ctx, fmt.Sprintf("user:%s:competitions", detail.ID), compID).Err()
	}

	actionDesc := fmt.Sprintf("Added existing user %s to competition as %s", req.Email, role)
	if createdAccount {
		actionDesc = fmt.Sprintf("Created background account and added user %s to competition as %s", req.Email, role)
	}
	s.recordAudit(ctx, audit.ActionCompetitionMemberAssign, compID, actionDesc, &actorID, map[string]any{
		"target_user_id":  detail.ID,
		"target_email":    req.Email,
		"role":            role,
		"account_created": createdAccount,
	})

	return detail, createdAccount, nil
}

// BulkAddCompetitionUsers adds multiple users to a competition in bulk by email.
func (s *Service) BulkAddCompetitionUsers(
	ctx context.Context,
	compID, actorID, actorRole string,
	req BulkAddCompetitionUsersRequest,
) (*BulkAddCompetitionUsersResponse, error) {
	resp := &BulkAddCompetitionUsersResponse{
		TotalProcessed: len(req.Users),
		Users:          make([]CompetitionUserDetailResponse, 0, len(req.Users)),
		SkippedErrors:  make([]string, 0),
	}

	for i, u := range req.Users {
		if !isValidEmail(u.Email) {
			resp.SkippedCount++
			resp.SkippedErrors = append(resp.SkippedErrors, fmt.Sprintf("user %d (%s): invalid email address", i+1, u.Email))
			continue
		}

		role := u.Role
		if role == "" {
			role = "participant"
		}

		detail, createdAccount, err := s.repo.AddCompetitionUser(ctx, compID, actorID, actorRole, u.Email, u.Name, u.PhoneNumber, role)
		if err != nil {
			resp.SkippedCount++
			resp.SkippedErrors = append(resp.SkippedErrors, fmt.Sprintf("user %d (%s): %v", i+1, u.Email, err))
			continue
		}

		if createdAccount {
			resp.TotalCreated++
		} else {
			resp.TotalAdded++
		}

		// Update Redis cache immediately
		if s.rdb != nil {
			_ = s.rdb.Set(ctx, fmt.Sprintf("comp:%s:member:%s", compID, detail.ID), role, 15*time.Minute).Err()
			_ = s.rdb.SAdd(ctx, fmt.Sprintf("user:%s:competitions", detail.ID), compID).Err()
		}

		resp.Users = append(resp.Users, *detail)
	}

	s.recordAudit(ctx, audit.ActionCompetitionMemberAssign, compID,
		fmt.Sprintf("Bulk added users: %d created, %d added, %d skipped", resp.TotalCreated, resp.TotalAdded, resp.SkippedCount),
		&actorID, map[string]any{
			"total_processed": resp.TotalProcessed,
			"total_created":   resp.TotalCreated,
			"total_added":     resp.TotalAdded,
			"skipped_count":   resp.SkippedCount,
		},
	)

	return resp, nil
}

// AssignOrUpdateMemberRole handles role assignment with strict hierarchy enforcement:
// - Only 'owner' can assign or change the 'admin' role.
// - 'admin' can only assign or change 'volunteer', 'judge', or 'participant'.
// - 'admin' cannot change the role of another 'admin' or the 'owner'.
// - Nobody can be promoted to 'owner' through role update (must use TransferOwnership).
func (s *Service) AssignOrUpdateMemberRole(ctx context.Context, compID, actorID, actorRole, targetUserID, newRole string) error {
	if newRole == "owner" {
		return errors.New("cannot assign owner role; use transfer ownership endpoint")
	}

	// Owner cannot demote themselves here
	if actorID == targetUserID && actorRole == "owner" {
		return ErrCannotSelfDemote
	}

	// Admins cannot assign or promote to the admin role
	if actorRole == "admin" && newRole == "admin" {
		return ErrPermissionDenied
	}

	// Check target user's current role if they are already in the competition
	existingRole, _, err := s.repo.GetMemberRole(ctx, compID, targetUserID)
	if err == nil {
		if existingRole == "owner" {
			return ErrCannotModifyOwner
		}
		// Admin cannot modify another admin
		if actorRole == "admin" && existingRole == "admin" && actorID != targetUserID {
			return ErrPermissionDenied
		}
	} else if !errors.Is(err, ErrMemberNotFound) {
		return fmt.Errorf("check member role: %w", err)
	}

	if err := s.repo.AssignOrUpdateMemberRole(ctx, compID, targetUserID, newRole); err != nil {
		return err
	}

	// Invalidate member role in Redis
	if s.rdb != nil {
		_ = s.rdb.Del(ctx, fmt.Sprintf("comp:%s:member:%s", compID, targetUserID)).Err()
	}

	s.recordAudit(ctx, audit.ActionCompetitionMemberRoleUpdate, compID, fmt.Sprintf("Assigned role %s to user %s", newRole, targetUserID), &actorID, map[string]any{
		"target_user_id": targetUserID,
		"new_role":       newRole,
	})

	return nil
}

// RemoveMember removes a user from a competition:
// - Cannot remove the owner.
// - Admins cannot remove other admins.
func (s *Service) RemoveMember(ctx context.Context, compID, actorID, actorRole, targetUserID string) error {
	targetRole, _, err := s.repo.GetMemberRole(ctx, compID, targetUserID)
	if err != nil {
		return err
	}

	if targetRole == "owner" {
		return ErrCannotRemoveOwner
	}

	if actorRole == "admin" && targetRole == "admin" && actorID != targetUserID {
		return ErrPermissionDenied
	}

	if err := s.repo.RemoveMember(ctx, compID, targetUserID); err != nil {
		return err
	}

	// Invalidate member role in Redis & remove from accessible competitions set
	if s.rdb != nil {
		_ = s.rdb.Del(ctx, fmt.Sprintf("comp:%s:member:%s", compID, targetUserID)).Err()
		_ = s.rdb.SRem(ctx, fmt.Sprintf("user:%s:competitions", targetUserID), compID).Err()
	}

	s.recordAudit(ctx, audit.ActionCompetitionMemberRemove, compID, fmt.Sprintf("Removed user %s from competition", targetUserID), &actorID, map[string]any{
		"target_user_id": targetUserID,
	})

	return nil
}

// BanMember bans a member from the competition.
// - Cannot ban the owner.
// - Admins cannot ban other admins (only owner can ban admins).
func (s *Service) BanMember(ctx context.Context, compID, actorID, actorRole, targetUserID string, reason *string) error {
	targetRole, isBanned, err := s.repo.GetMemberRole(ctx, compID, targetUserID)
	if err != nil {
		return err
	}
	if isBanned {
		return nil
	}
	if targetRole == "owner" {
		return errors.New("cannot ban the competition owner")
	}
	if actorRole == "admin" && targetRole == "admin" && actorID != targetUserID {
		return ErrPermissionDenied
	}

	if err := s.repo.BanMember(ctx, compID, targetUserID); err != nil {
		return err
	}

	// Update Redis immediately:
	// 1. Mark comp:id:member:id as "banned" with 15m TTL
	// 2. Remove compID from user:id:competitions set
	if s.rdb != nil {
		memberKey := fmt.Sprintf("comp:%s:member:%s", compID, targetUserID)
		_ = s.rdb.Set(ctx, memberKey, "banned", 15*time.Minute).Err()
		_ = s.rdb.SRem(ctx, fmt.Sprintf("user:%s:competitions", targetUserID), compID).Err()
	}

	desc := fmt.Sprintf("Banned user %s from competition", targetUserID)
	if reason != nil && *reason != "" {
		desc += fmt.Sprintf(" (reason: %s)", *reason)
	}
	s.recordAudit(ctx, "competition.member.ban", compID, desc, &actorID, map[string]any{
		"target_user_id": targetUserID,
		"reason":         reason,
	})

	return nil
}

// UnbanMember restores a banned member's access in the competition.
func (s *Service) UnbanMember(ctx context.Context, compID, actorID, targetUserID string) error {
	restoredRole, err := s.repo.UnbanMember(ctx, compID, targetUserID)
	if err != nil {
		return err
	}

	// Update Redis immediately:
	// 1. Set comp:id:member:id to restoredRole with 15m TTL
	// 2. Add compID back to user:id:competitions set
	if s.rdb != nil {
		memberKey := fmt.Sprintf("comp:%s:member:%s", compID, targetUserID)
		_ = s.rdb.Set(ctx, memberKey, restoredRole, 15*time.Minute).Err()
		userCompsKey := fmt.Sprintf("user:%s:competitions", targetUserID)
		_ = s.rdb.SAdd(ctx, userCompsKey, compID).Err()
		_ = s.rdb.Expire(ctx, userCompsKey, time.Hour).Err()
	}

	s.recordAudit(ctx, "competition.member.unban", compID, fmt.Sprintf("Unbanned user %s in competition", targetUserID), &actorID, map[string]any{
		"target_user_id": targetUserID,
		"restored_role":  restoredRole,
	})

	return nil
}

// ListCompetitionUsers returns paginated user profiles scoped to this competition.
func (s *Service) ListCompetitionUsers(ctx context.Context, compID string, filter CompetitionUserFilter) (*CompetitionUserListResponse, error) {
	users, total, err := s.repo.ListCompetitionUsers(ctx, compID, filter)
	if err != nil {
		return nil, err
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

	return &CompetitionUserListResponse{
		Users: users,
		Total: total,
		Page:  page,
		Limit: limit,
	}, nil
}

// ListMembers lists competition members.
func (s *Service) ListMembers(ctx context.Context, compID, roleFilter string) ([]MemberWithUser, error) {
	return s.repo.ListMembers(ctx, compID, roleFilter)
}

// GetMyMembership retrieves the active user's membership and role in this competition.
func (s *Service) GetMyMembership(ctx context.Context, compID, userID string) (*CompetitionMember, error) {
	role, isBanned, err := s.repo.GetMemberRole(ctx, compID, userID)
	if err != nil {
		return nil, err
	}
	return &CompetitionMember{
		CompetitionID: compID,
		UserID:        userID,
		Role:          role,
		IsBanned:      isBanned,
	}, nil
}

func (s *Service) recordAudit(ctx context.Context, action, compID, description string, actorID *string, meta map[string]any) {
	if s.auditor == nil {
		return
	}
	s.auditor.Record(ctx, audit.Event{
		ActorID:     actorID,
		Action:      action,
		TargetType:  audit.TargetCompetition,
		TargetID:    &compID,
		Description: description,
		Metadata:    meta,
	})
}
