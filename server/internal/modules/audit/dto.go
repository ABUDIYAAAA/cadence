package audit

import "time"

// AuditLogFilter defines parameters for filtering the persistent audit trail.
type AuditLogFilter struct {
	Page       int        `json:"page"`
	Limit      int        `json:"limit"`
	Action     string     `json:"action"`
	ActorID    string     `json:"actor_id"`
	TargetType string     `json:"target_type"`
	TargetID   string     `json:"target_id"`
	Search     string     `json:"search"`
	FromDate   *time.Time `json:"from_date"`
	ToDate     *time.Time `json:"to_date"`
}

// AuditLogResponse represents a persisted audit log record.
type AuditLogResponse struct {
	ID          string         `json:"id"`
	ActorID     *string        `json:"actor_id,omitempty"`
	ActorEmail  *string        `json:"actor_email,omitempty"`
	ActorRole   *string        `json:"actor_role,omitempty"`
	Action      string         `json:"action"`
	TargetType  string         `json:"target_type"`
	TargetID    *string        `json:"target_id,omitempty"`
	Description string         `json:"description"`
	Metadata    map[string]any `json:"metadata"`
	IPAddress   *string        `json:"ip_address,omitempty"`
	UserAgent   *string        `json:"user_agent,omitempty"`
	DeviceID    *string        `json:"device_id,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
}

// PaginatedAuditLogsResponse wraps a page of audit log entries.
type PaginatedAuditLogsResponse struct {
	Data       []AuditLogResponse `json:"data"`
	TotalCount int64              `json:"total_count"`
	Page       int                `json:"page"`
	Limit      int                `json:"limit"`
	TotalPages int                `json:"total_pages"`
}
