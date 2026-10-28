package notifications

import "time"

// SubscriptionKeys contains the client's public key and authentication secret.
type SubscriptionKeys struct {
	P256dh string `json:"p256dh" validate:"required"`
	Auth   string `json:"auth" validate:"required"`
}

// SubscribeRequest is sent by browser service worker to register a push subscription.
type SubscribeRequest struct {
	Endpoint string           `json:"endpoint" validate:"required,url"`
	Keys     SubscriptionKeys `json:"keys" validate:"required"`
}

// UnsubscribeRequest is sent to remove an existing push subscription.
type UnsubscribeRequest struct {
	Endpoint string `json:"endpoint" validate:"required,url"`
}

// BroadcastNotificationRequest allows administrators to dispatch messages to targets.
type BroadcastNotificationRequest struct {
	Title      string  `json:"title" validate:"required,max=255"`
	Body       string  `json:"body" validate:"required"`
	ActionURL  *string `json:"action_url,omitempty"`
	IconURL    *string `json:"icon_url,omitempty"`
	TargetType string  `json:"target_type" validate:"required,oneof=broadcast team role user"`
	TargetID   *string `json:"target_id,omitempty"`
}

// BroadcastNotificationResponse summarizes an admin notification dispatch.
type BroadcastNotificationResponse struct {
	NotificationID  string `json:"notification_id"`
	RecipientsCount int    `json:"recipients_count"`
	EnqueuedPushes  int    `json:"enqueued_pushes"`
	Message         string `json:"message"`
}

// NotificationItem represents a notification in the user's inbox feed.
type NotificationItem struct {
	ID        string     `json:"id"`
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	ActionURL *string    `json:"action_url,omitempty"`
	IconURL   *string    `json:"icon_url,omitempty"`
	IsRead    bool       `json:"is_read"`
	ReadAt    *time.Time `json:"read_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// InboxResponse encapsulates the user's paginated notification feed and unread badge count.
type InboxResponse struct {
	Notifications []NotificationItem `json:"notifications"`
	UnreadCount   int                `json:"unread_count"`
	TotalCount    int                `json:"total_count"`
	Page          int                `json:"page"`
	Limit         int                `json:"limit"`
}

// AdminNotificationHistoryItem provides dispatch history with delivery/read statistics.
type AdminNotificationHistoryItem struct {
	ID              string    `json:"id"`
	Title           string    `json:"title"`
	Body            string    `json:"body"`
	ActionURL       *string   `json:"action_url,omitempty"`
	TargetType      string    `json:"target_type"`
	TargetID        *string   `json:"target_id,omitempty"`
	CreatedByEmail  *string   `json:"created_by_email,omitempty"`
	RecipientsCount int       `json:"recipients_count"`
	ReadCount       int       `json:"read_count"`
	CreatedAt       time.Time `json:"created_at"`
}

// AdminHistoryResponse provides paginated admin dispatch history.
type AdminHistoryResponse struct {
	Items      []AdminNotificationHistoryItem `json:"items"`
	TotalCount int                            `json:"total_count"`
	Page       int                            `json:"page"`
	Limit      int                            `json:"limit"`
}

// PushPayload is the serialized JSON sent inside the encrypted Web Push frame to the browser.
type PushPayload struct {
	Title          string  `json:"title"`
	Body           string  `json:"body"`
	URL            *string `json:"url,omitempty"`
	Icon           *string `json:"icon,omitempty"`
	NotificationID string  `json:"notification_id"`
}
