package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"devclubxnst.com/cadence/internal/config"
	"devclubxnst.com/cadence/pkg/audit"
)

type Service struct {
	repo       *Repository
	queue      *Queue
	cfg        *config.Config
	auditQueue audit.Recorder
}

func NewService(
	repo *Repository,
	queue *Queue,
	cfg *config.Config,
	auditQueue audit.Recorder,
) *Service {
	return &Service{
		repo:       repo,
		queue:      queue,
		cfg:        cfg,
		auditQueue: auditQueue,
	}
}

// GetVAPIDPublicKey returns the public key that web clients need to call PushManager.subscribe.
func (s *Service) GetVAPIDPublicKey() string {
	if s.queue != nil {
		return s.queue.VAPIDPublicKey()
	}
	return s.cfg.VAPIDPublicKey
}

// Subscribe saves or updates the browser push subscription for a user.
func (s *Service) Subscribe(
	ctx context.Context,
	userID string,
	req SubscribeRequest,
	userAgent *string,
	ip string,
) error {
	if err := s.repo.UpsertSubscription(ctx, userID, req.Endpoint, req.Keys.P256dh, req.Keys.Auth, userAgent); err != nil {
		return err
	}

	s.recordAudit(
		ctx,
		audit.ActionNotificationSubscribe,
		audit.TargetNotification,
		nil,
		"Client registered for Web Push notifications",
		&userID, nil, nil,
		map[string]any{"endpoint": req.Endpoint},
		&ip, userAgent,
	)

	return nil
}

// Unsubscribe removes a push endpoint for the user.
func (s *Service) Unsubscribe(
	ctx context.Context,
	userID string,
	req UnsubscribeRequest,
	userAgent *string,
	ip string,
) error {
	if err := s.repo.DeleteSubscription(ctx, req.Endpoint); err != nil {
		return err
	}

	s.recordAudit(
		ctx,
		audit.ActionNotificationUnsubscribe,
		audit.TargetNotification,
		nil,
		"Client unregistered from Web Push notifications",
		&userID, nil, nil,
		map[string]any{"endpoint": req.Endpoint},
		&ip, userAgent,
	)

	return nil
}

// Broadcast dispatches notifications to target recipients, saves in-app feed, and enqueues push tasks.
func (s *Service) Broadcast(
	ctx context.Context,
	req BroadcastNotificationRequest,
	actorID, actorEmail, actorRole string,
	ip, userAgent string,
) (*BroadcastNotificationResponse, error) {
	recipientIDs, err := s.repo.GetEligibleRecipientIDs(ctx, req.TargetType, req.TargetID)
	if err != nil {
		return nil, fmt.Errorf("resolve recipients: %w", err)
	}

	notifID, err := s.repo.CreateNotification(
		ctx,
		req.Title,
		req.Body,
		req.ActionURL,
		req.IconURL,
		req.TargetType,
		req.TargetID,
		&actorID,
		recipientIDs,
	)
	if err != nil {
		return nil, fmt.Errorf("create notification record: %w", err)
	}

	enqueuedCount := 0
	if s.queue != nil && len(recipientIDs) > 0 {
		subs, err := s.repo.GetPushSubscriptionsForUsers(ctx, recipientIDs)
		if err == nil && len(subs) > 0 {
			payloadBytes, _ := json.Marshal(PushPayload{
				Title:          req.Title,
				Body:           req.Body,
				URL:            req.ActionURL,
				Icon:           req.IconURL,
				NotificationID: notifID,
			})

			for _, sub := range subs {
				err := s.queue.Enqueue(ctx, PushJob{
					Endpoint: sub.Endpoint,
					P256dh:   sub.P256dh,
					Auth:     sub.Auth,
					Payload:  payloadBytes,
				})
				if err == nil {
					enqueuedCount++
				}
			}
		}
	}

	s.recordAudit(
		ctx,
		audit.ActionNotificationBroadcast,
		audit.TargetNotification,
		&notifID,
		fmt.Sprintf("Notification '%s' broadcast by %s (target: %s, recipients: %d, pushes: %d)",
			req.Title, actorEmail, req.TargetType, len(recipientIDs), enqueuedCount),
		&actorID, &actorEmail, &actorRole,
		map[string]any{
			"notification_id":  notifID,
			"title":            req.Title,
			"target_type":      req.TargetType,
			"target_id":        req.TargetID,
			"recipients_count": len(recipientIDs),
			"enqueued_pushes":  enqueuedCount,
		},
		&ip, &userAgent,
	)

	return &BroadcastNotificationResponse{
		NotificationID:  notifID,
		RecipientsCount: len(recipientIDs),
		EnqueuedPushes:  enqueuedCount,
		Message:         "Notification published and queued for push delivery",
	}, nil
}

// GetInbox returns paginated notifications with unread counts for the user.
func (s *Service) GetInbox(ctx context.Context, userID string, page, limit int) (*InboxResponse, error) {
	return s.repo.GetUserInbox(ctx, userID, page, limit)
}

// MarkAsRead marks a single notification as read.
func (s *Service) MarkAsRead(ctx context.Context, userID, notificationID string) error {
	return s.repo.MarkAsRead(ctx, userID, notificationID)
}

// MarkAllAsRead marks all user notifications as read.
func (s *Service) MarkAllAsRead(ctx context.Context, userID string) error {
	return s.repo.MarkAllAsRead(ctx, userID)
}

// GetAdminHistory returns sent notification history.
func (s *Service) GetAdminHistory(ctx context.Context, page, limit int) (*AdminHistoryResponse, error) {
	return s.repo.GetAdminHistory(ctx, page, limit)
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
