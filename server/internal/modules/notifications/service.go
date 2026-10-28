package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"devclubxnst.com/cadence/internal/config"
	"devclubxnst.com/cadence/pkg/audit"
)

var (
	ErrInvalidPushScheme  = errors.New("push endpoint must use https scheme")
	ErrInvalidPushHost    = errors.New("push endpoint must have a valid hostname")
	ErrPushCustomPort     = errors.New("push endpoint cannot use custom ports")
	ErrPushPrivateNetwork = errors.New("push endpoint cannot target loopback, private, or internal network addresses")
)

// ValidatePushEndpoint ensures that the push endpoint uses HTTPS, has no custom ports,
// and does not resolve to any loopback, private RFC 1918, link-local, or cloud metadata IP addresses.
func ValidatePushEndpoint(rawURL string) error {
	u, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return fmt.Errorf("invalid push endpoint URL: %w", err)
	}

	if !strings.EqualFold(u.Scheme, "https") {
		return ErrInvalidPushScheme
	}

	hostname := u.Hostname()
	if hostname == "" {
		return ErrInvalidPushHost
	}

	// Reject custom ports; standard push gateways operate on standard 443
	port := u.Port()
	if port != "" && port != "443" {
		return ErrPushCustomPort
	}

	// If hostname is directly an IP literal
	if ip := net.ParseIP(hostname); ip != nil {
		if IsRestrictedIP(ip) {
			return ErrPushPrivateNetwork
		}
	} else {
		// Resolve DNS hostname
		ips, err := net.LookupIP(hostname)
		if err != nil {
			return fmt.Errorf("cannot resolve push endpoint hostname: %w", err)
		}
		if len(ips) == 0 {
			return errors.New("push endpoint hostname did not resolve to any IP address")
		}
		for _, ip := range ips {
			if IsRestrictedIP(ip) {
				return ErrPushPrivateNetwork
			}
		}
	}

	return nil
}

// IsRestrictedIP checks if an IP belongs to loopback, private RFC 1918, link-local, carrier-grade NAT, or metadata ranges.
func IsRestrictedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}

	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}

	if ipv4 := ip.To4(); ipv4 != nil {
		// 0.0.0.0/8 (current network)
		if ipv4[0] == 0 {
			return true
		}
		// 100.64.0.0/10 (Carrier-grade NAT)
		if ipv4[0] == 100 && (ipv4[1]&0xc0) == 64 {
			return true
		}
		// 169.254.0.0/16 (Link-local / Cloud metadata: 169.254.169.254)
		if ipv4[0] == 169 && ipv4[1] == 254 {
			return true
		}
		// 192.0.0.0/24 (IETF Protocol Assignments)
		if ipv4[0] == 192 && ipv4[1] == 0 && ipv4[2] == 0 {
			return true
		}
		// 192.0.2.0/24 (TEST-NET-1)
		if ipv4[0] == 192 && ipv4[1] == 0 && ipv4[2] == 2 {
			return true
		}
		// 198.18.0.0/15 (Benchmarking)
		if ipv4[0] == 198 && (ipv4[1]&0xfe) == 18 {
			return true
		}
		// 198.51.100.0/24 (TEST-NET-2)
		if ipv4[0] == 198 && ipv4[1] == 51 && ipv4[2] == 100 {
			return true
		}
		// 203.0.113.0/24 (TEST-NET-3)
		if ipv4[0] == 203 && ipv4[1] == 0 && ipv4[2] == 113 {
			return true
		}
		// 240.0.0.0/4 (Reserved)
		if (ipv4[0] & 0xf0) == 240 {
			return true
		}
		// 255.255.255.255/32 (Broadcast)
		if ipv4[0] == 255 && ipv4[1] == 255 && ipv4[2] == 255 && ipv4[3] == 255 {
			return true
		}
	} else {
		// IPv4-mapped IPv6 check
		if len(ip) == net.IPv6len && ip[0] == 0 && ip[1] == 0 && ip[2] == 0 && ip[3] == 0 &&
			ip[4] == 0 && ip[5] == 0 && ip[6] == 0 && ip[7] == 0 &&
			ip[8] == 0 && ip[9] == 0 && ip[10] == 0xff && ip[11] == 0xff {
			return IsRestrictedIP(net.IPv4(ip[12], ip[13], ip[14], ip[15]))
		}
	}

	return false
}

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
	if err := ValidatePushEndpoint(req.Endpoint); err != nil {
		return err
	}

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
	if err := s.repo.DeleteUserSubscription(ctx, userID, req.Endpoint); err != nil {
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
