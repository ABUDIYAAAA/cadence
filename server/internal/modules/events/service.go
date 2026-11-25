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
	if repo != nil && auditor != nil {
		repo.WithAuditor(auditor)
	}
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

	if s.auditor != nil {
		s.auditor.Record(ctx, audit.Event{
			EventID:     &ev.ID,
			ActorID:     &creatorUserID,
			Action:      audit.ActionEventCreate,
			TargetType:  audit.TargetEvent,
			TargetID:    &ev.ID,
			Description: fmt.Sprintf("Created event %s (%s)", ev.Name, ev.Slug),
			Metadata: map[string]any{
				"slug":          ev.Slug,
				"name":          ev.Name,
				"logo_url":      ev.LogoURL,
				"owner_user_id": ownerUserID,
			},
		})
	}

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

	if s.auditor != nil {
		s.auditor.RecordAction(ctx, audit.ActionEventDelete, audit.TargetEvent, &ev.ID, fmt.Sprintf("Deleted event %s (%s)", ev.Name, ev.Slug), map[string]any{
			"slug": ev.Slug,
			"name": ev.Name,
		})
	}
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

	if s.auditor != nil {
		s.auditor.RecordAction(ctx, audit.ActionEventUpdate, audit.TargetEvent, &ev.ID, fmt.Sprintf("Updated event %s", ev.Slug), map[string]any{
			"name":     req.Name,
			"logo_url": ev.LogoURL,
		})
	}

	return ev, nil
}

// TransferOwnership transfers the owner role to another member.
// Both the current event owner and platform devs may transfer ownership.
func (s *Service) TransferOwnership(ctx context.Context, eventID, actorID string, isDev bool, req TransferOwnershipRequest) error {
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

	prevOwnerID, err := s.repo.TransferOwnership(ctx, eventID, actorID, newOwnerID, isDev)
	if err != nil {
		return err
	}

	if s.rdb != nil {
		keys := []string{fmt.Sprintf("event:%s:member:%s", eventID, newOwnerID)}
		if prevOwnerID != "" {
			keys = append(keys, fmt.Sprintf("event:%s:member:%s", eventID, prevOwnerID))
		}
		_ = s.rdb.Del(ctx, keys...).Err()
		_ = s.rdb.SAdd(ctx, fmt.Sprintf("user:%s:events", newOwnerID), eventID).Err()
	}

	if s.auditor != nil {
		s.auditor.RecordAction(ctx, audit.ActionEventTransferOwnership, audit.TargetEvent, &eventID, fmt.Sprintf("Transferred event ownership from %s to %s", prevOwnerID, newOwnerID), map[string]any{
			"previous_owner_user_id": prevOwnerID,
			"new_owner_user_id":      newOwnerID,
		})
	}

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
	if s.auditor != nil {
		s.auditor.RecordAction(ctx, audit.ActionEventMemberAssign, audit.TargetEvent, &eventID, actionDesc, map[string]any{
			"target_user_id":  detail.ID,
			"target_email":    req.Email,
			"role":            role,
			"account_created": createdAccount,
		})
	}

	return detail, createdAccount, nil
}

// BulkAddEventUsers adds multiple users to an event in bulk by email,
// optionally assigning participants to teams within named competitions.
// Duplicate emails across rows (e.g. a participant registered in multiple competitions)
// are deduplicated for account/event enrollment while processing each team assignment.
func (s *Service) BulkAddEventUsers(
	ctx context.Context,
	eventID, actorID, actorRole string,
	req BulkAddEventUsersRequest,
) (*BulkAddEventUsersResponse, error) {
	resp := &BulkAddEventUsersResponse{
		TotalProcessed: len(req.Users),
		Users:          make([]EventUserDetailResponse, 0, len(req.Users)),
		SkippedErrors:  make([]string, 0),
		MissingUsers:   make([]MissingUserEntry, 0),
	}

	seenUsers := make(map[string]*EventUserDetailResponse, len(req.Users))
	missingEmails := make(map[string]bool)

	for i, u := range req.Users {
		normEmail := strings.TrimSpace(strings.ToLower(u.Email))
		if !isValidEmail(normEmail) {
			resp.SkippedCount++
			resp.SkippedErrors = append(resp.SkippedErrors, fmt.Sprintf("row %d (%s): invalid email address", i+1, u.Email))
			continue
		}

		role := u.Role
		if role == "" {
			role = "participant"
		}

		detail, alreadySeen := seenUsers[normEmail]
		if !alreadySeen {
			if req.SkipMissingUsers {
				if missingEmails[normEmail] {
					resp.SkippedCount++
					resp.MissingUsers = append(resp.MissingUsers, MissingUserEntry{
						Email:           normEmail,
						Name:            u.Name,
						PhoneNumber:     u.PhoneNumber,
						CompetitionName: u.CompetitionName,
						TeamName:        u.TeamName,
					})
					resp.SkippedErrors = append(resp.SkippedErrors, fmt.Sprintf("row %d (%s): account does not exist", i+1, u.Email))
					continue
				}

				existingDetail, exists, err := s.repo.FindOrEnrollExistingEventUser(ctx, eventID, normEmail, role)
				if err != nil {
					resp.SkippedCount++
					resp.SkippedErrors = append(resp.SkippedErrors, fmt.Sprintf("row %d (%s): %v", i+1, u.Email, err))
					continue
				}
				if !exists {
					missingEmails[normEmail] = true
					resp.SkippedCount++
					resp.MissingUsers = append(resp.MissingUsers, MissingUserEntry{
						Email:           normEmail,
						Name:            u.Name,
						PhoneNumber:     u.PhoneNumber,
						CompetitionName: u.CompetitionName,
						TeamName:        u.TeamName,
					})
					resp.SkippedErrors = append(resp.SkippedErrors, fmt.Sprintf("row %d (%s): account does not exist", i+1, u.Email))
					continue
				}

				resp.TotalAdded++
				if s.rdb != nil {
					_ = s.rdb.Set(ctx, fmt.Sprintf("event:%s:member:%s", eventID, existingDetail.ID), existingDetail.Role, 15*time.Minute).Err()
					_ = s.rdb.SAdd(ctx, fmt.Sprintf("user:%s:events", existingDetail.ID), eventID).Err()
				}

				detail = existingDetail
				seenUsers[normEmail] = detail
				resp.Users = append(resp.Users, *detail)
			} else {
				addedDetail, createdAccount, err := s.repo.AddEventUser(ctx, eventID, actorID, actorRole, normEmail, u.Name, u.PhoneNumber, role)
				if err != nil {
					resp.SkippedCount++
					resp.SkippedErrors = append(resp.SkippedErrors, fmt.Sprintf("row %d (%s): %v", i+1, u.Email, err))
					continue
				}

				if createdAccount {
					resp.TotalCreated++
				} else {
					resp.TotalAdded++
				}

				if s.rdb != nil {
					_ = s.rdb.Set(ctx, fmt.Sprintf("event:%s:member:%s", eventID, addedDetail.ID), role, 15*time.Minute).Err()
					_ = s.rdb.SAdd(ctx, fmt.Sprintf("user:%s:events", addedDetail.ID), eventID).Err()
				}

				detail = addedDetail
				seenUsers[normEmail] = detail
				resp.Users = append(resp.Users, *detail)
			}
		}

		compName := ""
		if u.CompetitionName != nil {
			compName = strings.TrimSpace(*u.CompetitionName)
		}
		teamName := ""
		if u.TeamName != nil {
			teamName = strings.TrimSpace(*u.TeamName)
		}

		if teamName != "" || compName != "" {
			if teamName == "" {
				// competition_name without team_name: nothing to assign
				continue
			}
			_, _, teamCreated, newlyAssigned, err := s.repo.AssignEventUserToCompetitionTeam(
				ctx, eventID, actorID, detail.ID, detail.Role, compName, teamName,
			)
			if err != nil {
				resp.SkippedCount++
				resp.SkippedErrors = append(resp.SkippedErrors, fmt.Sprintf("row %d (%s): %v", i+1, u.Email, err))
				continue
			}
			if teamCreated {
				resp.TeamsCreated++
			}
			if newlyAssigned {
				resp.TeamsAssigned++
			}
		}
	}

	if s.auditor != nil {
		s.auditor.RecordAction(ctx, audit.ActionEventMemberAssign, audit.TargetEvent, &eventID,
			fmt.Sprintf("Bulk added users: %d created, %d added, %d teams created, %d team assignments, %d skipped",
				resp.TotalCreated, resp.TotalAdded, resp.TeamsCreated, resp.TeamsAssigned, resp.SkippedCount),
			map[string]any{
				"total_processed": resp.TotalProcessed,
				"total_created":   resp.TotalCreated,
				"total_added":     resp.TotalAdded,
				"teams_created":   resp.TeamsCreated,
				"teams_assigned":  resp.TeamsAssigned,
				"skipped_count":   resp.SkippedCount,
			},
		)
	}

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

	if s.auditor != nil {
		s.auditor.RecordAction(ctx, audit.ActionEventMemberRoleUpdate, audit.TargetEvent, &eventID, fmt.Sprintf("Assigned role %s to user %s", newRole, targetUserID), map[string]any{
			"target_user_id": targetUserID,
			"new_role":       newRole,
		})
	}

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

	if s.auditor != nil {
		s.auditor.RecordAction(ctx, audit.ActionEventMemberRemove, audit.TargetEvent, &eventID, fmt.Sprintf("Removed user %s from event", targetUserID), map[string]any{
			"target_user_id": targetUserID,
		})
	}

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
	if s.auditor != nil {
		s.auditor.RecordAction(ctx, audit.ActionEventMemberBan, audit.TargetEvent, &eventID, desc, map[string]any{
			"target_user_id": targetUserID,
			"reason":         reason,
		})
	}

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

	if s.auditor != nil {
		s.auditor.RecordAction(ctx, audit.ActionEventMemberUnban, audit.TargetEvent, &eventID, fmt.Sprintf("Unbanned user %s in event", targetUserID), map[string]any{
			"target_user_id": targetUserID,
			"restored_role":  restoredRole,
		})
	}

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
