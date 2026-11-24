package events

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"devclubxnst.com/cadence/internal/modules/auth"
	"devclubxnst.com/cadence/pkg/audit"
	"devclubxnst.com/cadence/pkg/logger"
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
	slugRegex            = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	ErrInvalidSlug       = errors.New("slug must contain only lowercase letters, numbers, and single hyphens")
	ErrPermissionDenied  = errors.New("insufficient permissions to perform this role management action")
	ErrCannotSelfDemote  = errors.New("owner cannot demote themselves; transfer ownership instead")
	ErrCannotModifyOwner = errors.New("cannot change the role of the event owner; transfer ownership instead")
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

// CreateEvent validates inputs, ensures slug format, and binds the owner. Restricted to devs.
func (s *Service) CreateEvent(ctx context.Context, req CreateEventRequest, creatorUserID string) (*Event, error) {
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

	logoURL := req.LogoURL
	if logoURL == nil {
		logoURL = req.PhotoURL
	}

	ev, err := s.repo.CreateEvent(ctx, req.Slug, req.Name, req.Description, logoURL, ownerUserID)
	if err != nil {
		logger.FromContext(ctx).Warn("failed to create event", slog.String("slug", req.Slug), slog.Any("error", err))
		return nil, err
	}

	if s.rdb != nil {
		_ = s.rdb.Set(ctx, fmt.Sprintf("event:%s:member:%s", ev.ID, ownerUserID), "owner", 15*time.Minute).Err()
		_ = s.rdb.SAdd(ctx, fmt.Sprintf("user:%s:events", ownerUserID), ev.ID).Err()
	}

	logger.FromContext(ctx).Info("event created",
		slog.String("event_id", ev.ID),
		slog.String("slug", ev.Slug),
		slog.String("owner_user_id", ownerUserID),
	)

	s.recordAudit(ctx, audit.ActionEventCreate, ev.ID, fmt.Sprintf("Created event %s (%s)", ev.Name, ev.Slug), &creatorUserID, map[string]any{
		"slug":          ev.Slug,
		"name":          ev.Name,
		"logo_url":      ev.LogoURL,
		"owner_user_id": ownerUserID,
	})

	return ev, nil
}

// DeleteEvent deletes an event and purges its Redis cache. Restricted to devs.
func (s *Service) DeleteEvent(ctx context.Context, eventID, actorID string) error {
	ev, err := s.repo.DeleteEvent(ctx, eventID)
	if err != nil {
		logger.FromContext(ctx).Warn("failed to delete event", slog.String("event_id", eventID), slog.Any("error", err))
		return err
	}

	if s.rdb != nil {
		_ = s.rdb.Del(ctx, "event:slug:"+ev.Slug).Err()
	}

	logger.FromContext(ctx).Info("event deleted", slog.String("event_id", ev.ID), slog.String("slug", ev.Slug))

	s.recordAudit(ctx, audit.ActionEventDelete, ev.ID, fmt.Sprintf("Deleted event %s (%s)", ev.Name, ev.Slug), &actorID, map[string]any{
		"slug": ev.Slug,
		"name": ev.Name,
	})
	return nil
}

// ListEvents returns all events.
func (s *Service) ListEvents(ctx context.Context) ([]Event, error) {
	return s.repo.ListEvents(ctx)
}

// GetEventBySlug retrieves event metadata.
func (s *Service) GetEventBySlug(ctx context.Context, slug string) (*Event, error) {
	return s.repo.GetEventBySlug(ctx, slug)
}

// UpdateEvent updates event name, description, and logo/photo and purges Redis cache.
func (s *Service) UpdateEvent(ctx context.Context, eventID string, req UpdateEventRequest, actorID string) (*Event, error) {
	logoURL := req.LogoURL
	if logoURL == nil {
		logoURL = req.PhotoURL
	}

	ev, err := s.repo.UpdateEvent(ctx, eventID, req.Name, req.Description, logoURL)
	if err != nil {
		logger.FromContext(ctx).Warn("failed to update event", slog.String("event_id", eventID), slog.Any("error", err))
		return nil, err
	}

	if s.rdb != nil {
		_ = s.rdb.Del(ctx, "event:slug:"+ev.Slug).Err()
	}

	logger.FromContext(ctx).Info("event updated", slog.String("event_id", ev.ID), slog.String("slug", ev.Slug))

	s.recordAudit(ctx, audit.ActionEventUpdate, ev.ID, fmt.Sprintf("Updated event %s", ev.Slug), &actorID, map[string]any{
		"name":     req.Name,
		"logo_url": ev.LogoURL,
	})

	return ev, nil
}

// TransferOwnership transfers the owner role to another member.
func (s *Service) TransferOwnership(ctx context.Context, eventID, currentOwnerID string, req TransferOwnershipRequest) error {
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

	if err := s.repo.TransferOwnership(ctx, eventID, currentOwnerID, newOwnerID); err != nil {
		return err
	}

	if s.rdb != nil {
		_ = s.rdb.Del(ctx,
			fmt.Sprintf("event:%s:member:%s", eventID, currentOwnerID),
			fmt.Sprintf("event:%s:member:%s", eventID, newOwnerID),
		).Err()
		_ = s.rdb.SAdd(ctx, fmt.Sprintf("user:%s:events", newOwnerID), eventID).Err()
	}

	s.recordAudit(ctx, audit.ActionEventTransferOwnership, eventID, fmt.Sprintf("Transferred event ownership from %s to %s", currentOwnerID, newOwnerID), &currentOwnerID, map[string]any{
		"new_owner_user_id": newOwnerID,
	})

	return nil
}

// AddEventUser adds a single user to an event by email, creating a background account if needed.
func (s *Service) AddEventUser(
	ctx context.Context,
	eventID, actorID, actorRole string,
	req AddEventUserRequest,
) (*EventUserDetailResponse, bool, error) {
	if !isValidEmail(req.Email) {
		return nil, false, errors.New("invalid email address")
	}

	role := req.Role
	if role == "" {
		role = "participant"
	}

	detail, createdAccount, err := s.repo.AddEventUser(ctx, eventID, actorID, actorRole, req.Email, req.Name, req.PhoneNumber, role)
	if err != nil {
		return nil, false, err
	}

	if s.rdb != nil {
		_ = s.rdb.Set(ctx, fmt.Sprintf("event:%s:member:%s", eventID, detail.ID), role, 15*time.Minute).Err()
		_ = s.rdb.SAdd(ctx, fmt.Sprintf("user:%s:events", detail.ID), eventID).Err()
	}

	actionDesc := fmt.Sprintf("Added existing user %s to event as %s", req.Email, role)
	if createdAccount {
		actionDesc = fmt.Sprintf("Created background account and added user %s to event as %s", req.Email, role)
	}
	s.recordAudit(ctx, audit.ActionEventMemberAssign, eventID, actionDesc, &actorID, map[string]any{
		"target_user_id":  detail.ID,
		"target_email":    req.Email,
		"role":            role,
		"account_created": createdAccount,
	})

	return detail, createdAccount, nil
}

// BulkAddEventUsers adds multiple users to an event in bulk by email.
func (s *Service) BulkAddEventUsers(
	ctx context.Context,
	eventID, actorID, actorRole string,
	req BulkAddEventUsersRequest,
) (*BulkAddEventUsersResponse, error) {
	resp := &BulkAddEventUsersResponse{
		TotalProcessed: len(req.Users),
		Users:          make([]EventUserDetailResponse, 0, len(req.Users)),
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

		detail, createdAccount, err := s.repo.AddEventUser(ctx, eventID, actorID, actorRole, u.Email, u.Name, u.PhoneNumber, role)
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

		if s.rdb != nil {
			_ = s.rdb.Set(ctx, fmt.Sprintf("event:%s:member:%s", eventID, detail.ID), role, 15*time.Minute).Err()
			_ = s.rdb.SAdd(ctx, fmt.Sprintf("user:%s:events", detail.ID), eventID).Err()
		}

		resp.Users = append(resp.Users, *detail)
	}

	s.recordAudit(ctx, audit.ActionEventMemberAssign, eventID,
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

// AssignOrUpdateMemberRole handles role assignment with strict hierarchy enforcement.
func (s *Service) AssignOrUpdateMemberRole(ctx context.Context, eventID, actorID, actorRole, targetUserID, newRole string) error {
	if newRole == "owner" {
		return errors.New("cannot assign owner role; use transfer ownership endpoint")
	}

	if actorID == targetUserID && actorRole == "owner" {
		return ErrCannotSelfDemote
	}

	if actorRole == "admin" && newRole == "admin" {
		return ErrPermissionDenied
	}

	existingRole, _, err := s.repo.GetMemberRole(ctx, eventID, targetUserID)
	if err == nil {
		if existingRole == "owner" {
			return ErrCannotModifyOwner
		}
		if actorRole == "admin" && existingRole == "admin" && actorID != targetUserID {
			return ErrPermissionDenied
		}
	} else if !errors.Is(err, ErrMemberNotFound) {
		return fmt.Errorf("check member role: %w", err)
	}

	if err := s.repo.AssignOrUpdateMemberRole(ctx, eventID, targetUserID, newRole); err != nil {
		return err
	}

	if s.rdb != nil {
		_ = s.rdb.Del(ctx, fmt.Sprintf("event:%s:member:%s", eventID, targetUserID)).Err()
	}

	s.recordAudit(ctx, audit.ActionEventMemberRoleUpdate, eventID, fmt.Sprintf("Assigned role %s to user %s", newRole, targetUserID), &actorID, map[string]any{
		"target_user_id": targetUserID,
		"new_role":       newRole,
	})

	return nil
}

// RemoveMember removes a user from an event.
func (s *Service) RemoveMember(ctx context.Context, eventID, actorID, actorRole, targetUserID string) error {
	targetRole, _, err := s.repo.GetMemberRole(ctx, eventID, targetUserID)
	if err != nil {
		return err
	}

	if targetRole == "owner" {
		return ErrCannotRemoveOwner
	}

	if actorRole == "admin" && targetRole == "admin" && actorID != targetUserID {
		return ErrPermissionDenied
	}

	if err := s.repo.RemoveMember(ctx, eventID, targetUserID); err != nil {
		return err
	}

	if s.rdb != nil {
		_ = s.rdb.Del(ctx, fmt.Sprintf("event:%s:member:%s", eventID, targetUserID)).Err()
		_ = s.rdb.SRem(ctx, fmt.Sprintf("user:%s:events", targetUserID), eventID).Err()
	}

	s.recordAudit(ctx, audit.ActionEventMemberRemove, eventID, fmt.Sprintf("Removed user %s from event", targetUserID), &actorID, map[string]any{
		"target_user_id": targetUserID,
	})

	return nil
}

// BanMember bans a member from the event and evicts them from all competition teams in the event.
func (s *Service) BanMember(ctx context.Context, eventID, actorID, actorRole, targetUserID string, reason *string) error {
	targetRole, isBanned, err := s.repo.GetMemberRole(ctx, eventID, targetUserID)
	if err != nil {
		return err
	}
	if isBanned {
		return nil
	}
	if targetRole == "owner" {
		return errors.New("cannot ban the event owner")
	}
	if actorRole == "admin" && targetRole == "admin" && actorID != targetUserID {
		return ErrPermissionDenied
	}

	if err := s.repo.BanMember(ctx, eventID, targetUserID); err != nil {
		return err
	}

	if s.rdb != nil {
		memberKey := fmt.Sprintf("event:%s:member:%s", eventID, targetUserID)
		_ = s.rdb.Set(ctx, memberKey, "banned", 15*time.Minute).Err()
		_ = s.rdb.SRem(ctx, fmt.Sprintf("user:%s:events", targetUserID), eventID).Err()
	}

	desc := fmt.Sprintf("Banned user %s from event", targetUserID)
	if reason != nil && *reason != "" {
		desc += fmt.Sprintf(" (reason: %s)", *reason)
	}
	s.recordAudit(ctx, audit.ActionEventMemberBan, eventID, desc, &actorID, map[string]any{
		"target_user_id": targetUserID,
		"reason":         reason,
	})

	return nil
}

// UnbanMember restores a banned member's access in the event.
func (s *Service) UnbanMember(ctx context.Context, eventID, actorID, targetUserID string) error {
	restoredRole, err := s.repo.UnbanMember(ctx, eventID, targetUserID)
	if err != nil {
		return err
	}

	if s.rdb != nil {
		memberKey := fmt.Sprintf("event:%s:member:%s", eventID, targetUserID)
		_ = s.rdb.Set(ctx, memberKey, restoredRole, 15*time.Minute).Err()
		userEventsKey := fmt.Sprintf("user:%s:events", targetUserID)
		_ = s.rdb.SAdd(ctx, userEventsKey, eventID).Err()
		_ = s.rdb.Expire(ctx, userEventsKey, time.Hour).Err()
	}

	s.recordAudit(ctx, audit.ActionEventMemberUnban, eventID, fmt.Sprintf("Unbanned user %s in event", targetUserID), &actorID, map[string]any{
		"target_user_id": targetUserID,
		"restored_role":  restoredRole,
	})

	return nil
}

// ListEventUsers returns paginated user profiles scoped to this event.
func (s *Service) ListEventUsers(ctx context.Context, eventID string, filter EventUserFilter) (*EventUserListResponse, error) {
	users, total, err := s.repo.ListEventUsers(ctx, eventID, filter)
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

	return &EventUserListResponse{
		Users: users,
		Total: total,
		Page:  page,
		Limit: limit,
	}, nil
}

// ListMembers lists event members.
func (s *Service) ListMembers(ctx context.Context, eventID, roleFilter string) ([]MemberWithUser, error) {
	return s.repo.ListMembers(ctx, eventID, roleFilter)
}

func (s *Service) recordAudit(ctx context.Context, action, eventID, description string, actorID *string, meta map[string]any) {
	if s.auditor == nil {
		return
	}
	var actorEmail, actorRole *string
	if claims, ok := auth.GetUserClaims(ctx); ok && claims != nil {
		if actorID == nil && claims.UserID != "" {
			uid := claims.UserID
			actorID = &uid
		}
		if claims.Email != "" {
			email := claims.Email
			actorEmail = &email
		}
		if claims.Role != "" {
			role := claims.Role
			actorRole = &role
		} else if claims.GlobalRole != "" {
			gRole := claims.GlobalRole
			actorRole = &gRole
		}
	}
	if evCtx, ok := FromContext(ctx); ok && evCtx != nil && evCtx.Role != "" {
		role := evCtx.Role
		actorRole = &role
	}

	s.auditor.Record(ctx, audit.Event{
		EventID:     &eventID,
		ActorID:     actorID,
		ActorEmail:  actorEmail,
		ActorRole:   actorRole,
		Action:      action,
		TargetType:  audit.TargetEvent,
		TargetID:    &eventID,
		Description: description,
		Metadata:    meta,
		Timestamp:   time.Now(),
	})
}
