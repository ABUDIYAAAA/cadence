package audit

import (
	"context"
	"time"
)

// Standardized audit action constants
const (
	// Auth actions
	ActionAuthLoginSuccess  = "auth.login_success"
	ActionAuthLoginFailure  = "auth.login_failure"
	ActionAuthLogout        = "auth.logout"
	ActionAuthPasswordSetup = "auth.password_setup"
	ActionAuthPasswordReset = "auth.password_reset"

	// User actions
	ActionUserCreate       = "user.create"
	ActionUserBulkImport   = "user.bulk_import"
	ActionUserUpdate       = "user.update"
	ActionUserDelete       = "user.delete"
	ActionUserSendInvite   = "user.send_invite"
	ActionUserResetTrigger = "user.reset_trigger"

	// Team actions
	ActionTeamCreate       = "team.create"
	ActionTeamUpdate       = "team.update"
	ActionTeamDelete       = "team.delete"
	ActionTeamAddMember    = "team.add_member"
	ActionTeamRemoveMember = "team.remove_member"

	// Ticket actions
	ActionTicketIssued        = "ticket.issued"
	ActionTicketCheckIn       = "ticket.check_in"
	ActionTicketDuplicateScan = "ticket.duplicate_scan"
)

// Standardized target type constants
const (
	TargetUser    = "user"
	TargetTeam    = "team"
	TargetSession = "session"
	TargetAuth    = "auth"
	TargetTicket  = "ticket"
)

// Event encapsulates a structured domain action to be recorded in the audit trail.
type Event struct {
	ActorID     *string        `json:"actor_id,omitempty"`
	ActorEmail  *string        `json:"actor_email,omitempty"`
	ActorRole   *string        `json:"actor_role,omitempty"`
	Action      string         `json:"action"`
	TargetType  string         `json:"target_type"`
	TargetID    *string        `json:"target_id,omitempty"`
	Description string         `json:"description"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	IPAddress   *string        `json:"ip_address,omitempty"`
	UserAgent   *string        `json:"user_agent,omitempty"`
	DeviceID    *string        `json:"device_id,omitempty"`
	Timestamp   time.Time      `json:"timestamp"`
}

// Recorder defines an interface for capturing audit events asynchronously.
type Recorder interface {
	Record(ctx context.Context, event Event)
}
