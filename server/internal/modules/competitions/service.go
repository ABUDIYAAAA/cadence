package competitions

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"devclubxnst.com/cadence/pkg/audit"
	"github.com/redis/go-redis/v9"
)

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
		resolvedID, err := s.repo.GetUserIDByEmail(ctx, *req.OwnerEmail)
		if err != nil {
			return nil, err
		}
		ownerUserID = resolvedID
	}

	comp, err := s.repo.CreateCompetition(ctx, req.Slug, req.Name, req.Description, ownerUserID)
	if err != nil {
		return nil, err
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
func (s *Service) TransferOwnership(ctx context.Context, compID, currentOwnerID, newOwnerID string) error {
	if currentOwnerID == newOwnerID {
		return nil
	}

	if err := s.repo.TransferOwnership(ctx, compID, currentOwnerID, newOwnerID); err != nil {
		return err
	}

	// Invalidate cached member roles in Redis immediately
	if s.rdb != nil {
		_ = s.rdb.Del(ctx,
			fmt.Sprintf("comp:%s:member:%s", compID, currentOwnerID),
			fmt.Sprintf("comp:%s:member:%s", compID, newOwnerID),
		).Err()
	}

	s.recordAudit(ctx, audit.ActionCompetitionTransferOwnership, compID, fmt.Sprintf("Transferred competition ownership from %s to %s", currentOwnerID, newOwnerID), &currentOwnerID, map[string]any{
		"new_owner_user_id": newOwnerID,
	})

	return nil
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
	existingRole, err := s.repo.GetMemberRole(ctx, compID, targetUserID)
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
	targetRole, err := s.repo.GetMemberRole(ctx, compID, targetUserID)
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

	// Invalidate member role in Redis
	if s.rdb != nil {
		_ = s.rdb.Del(ctx, fmt.Sprintf("comp:%s:member:%s", compID, targetUserID)).Err()
	}

	s.recordAudit(ctx, audit.ActionCompetitionMemberRemove, compID, fmt.Sprintf("Removed user %s from competition", targetUserID), &actorID, map[string]any{
		"target_user_id": targetUserID,
	})

	return nil
}

// ListMembers lists competition members.
func (s *Service) ListMembers(ctx context.Context, compID, roleFilter string) ([]MemberWithUser, error) {
	return s.repo.ListMembers(ctx, compID, roleFilter)
}

// GetMyMembership retrieves the active user's membership and role in this competition.
func (s *Service) GetMyMembership(ctx context.Context, compID, userID string) (*CompetitionMember, error) {
	role, err := s.repo.GetMemberRole(ctx, compID, userID)
	if err != nil {
		return nil, err
	}
	return &CompetitionMember{
		CompetitionID: compID,
		UserID:        userID,
		Role:          role,
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
