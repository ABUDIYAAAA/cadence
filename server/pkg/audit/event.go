package audit

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Database column length bounds matching migrations/000004_create_audit_logs_table.up.sql.
const (
	MaxActorEmailLen  = 255
	MaxActorRoleLen   = 50
	MaxActionLen      = 100
	MaxTargetTypeLen  = 50
	MaxTargetIDLen    = 255
	MaxIPAddressLen   = 45
	MaxDeviceIDLen    = 64
	MaxDescriptionLen = 2048
	MaxUserAgentLen   = 1024
)

// Standardized audit action constants
const (
	// Auth actions
	ActionAuthLoginSuccess   = "auth.login_success"
	ActionAuthLoginFailure   = "auth.login_failure"
	ActionAuthLogout         = "auth.logout"
	ActionAuthPasswordSetup  = "auth.password_setup"
	ActionAuthPasswordReset  = "auth.password_reset"
	ActionAuthPasswordForgot = "auth.password_forgot"

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
	ActionTeamJoin         = "team.join"
	ActionTeamLeave        = "team.leave"
	ActionTeamKick         = "team.kick"
	ActionTeamFreeze       = "team.freeze"
	ActionTeamUnfreeze     = "team.unfreeze"
	ActionTeamFreezeAll    = "team.freeze_all"
	ActionTeamUnfreezeAll  = "team.unfreeze_all"
	ActionTeamRegenCode    = "team.regenerate_code"
	ActionTeamSettings     = "team.settings_update"

	// Ticket actions
	ActionTicketIssued         = "ticket.issued"
	ActionTicketCheckIn        = "ticket.check_in"
	ActionTicketDuplicateScan  = "ticket.duplicate_scan"
	ActionTicketReleased       = "ticket.released"
	ActionTicketSettingsUpdate = "ticket.settings_update"
	ActionScanSessionCreate    = "ticket.session_create"
	ActionScanSessionToggle    = "ticket.session_toggle"
	ActionTicketSessionScan    = "ticket.session_scan"
	ActionTicketScanRejected   = "ticket.scan_rejected"

	// Judging actions
	ActionJudgingRoundCreate     = "judging.round_create"
	ActionJudgingRoundActivate   = "judging.round_activate"
	ActionJudgingRoundEnd        = "judging.round_end"
	ActionJudgingRoundDelete     = "judging.round_delete"
	ActionJudgingSlotStart       = "judging.slot_start"
	ActionJudgingTimerPause      = "judging.timer_pause"
	ActionJudgingTimerResume     = "judging.timer_resume"
	ActionJudgingTimerExtend     = "judging.timer_extend"
	ActionJudgingScoreLock       = "judging.score_lock"
	ActionJudgingSlotComplete    = "judging.slot_complete"
	ActionJudgingSlotMove        = "judging.slot_move"
	ActionJudgingSlotQueue       = "judging.slot_queue"
	ActionJudgingSlotUnqueue     = "judging.slot_unqueue"
	ActionJudgingPanelCreate     = "judging.panel_create"
	ActionJudgingSlotsConfigure  = "judging.slots_configure"
	ActionJudgingSlotLinksUpdate = "judging.slot_links_update"

	// Notification actions
	ActionNotificationBroadcast   = "notification.broadcast"
	ActionNotificationSubscribe   = "notification.subscribe"
	ActionNotificationUnsubscribe = "notification.unsubscribe"

	// System actions (no human actor)
	ActionSystemBackupFailed    = "system.backup_failed"
	ActionSystemBackupRecovered = "system.backup_recovered"
)

// Standardized target type constants
const (
	TargetUser         = "user"
	TargetTeam         = "team"
	TargetSession      = "session"
	TargetAuth         = "auth"
	TargetTicket       = "ticket"
	TargetScanSession  = "scan_session"
	TargetJudgingRound = "judging_round"
	TargetJudgingSlot  = "judging_slot"
	TargetJudgingPanel = "judging_panel"
	TargetNotification = "notification"
	TargetSystem       = "system"
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

// SanitizeEvent strips NUL bytes / invalid UTF-8 sequences, validates security-critical
// identifiers, and truncates all string fields to their PostgreSQL column bounds while
// recording forensic metadata whenever a field was modified.
func SanitizeEvent(e Event) Event {
	sanitized, _ := SanitizeEventWithReport(e)
	return sanitized
}

// SanitizeEventWithReport performs the same sanitization as SanitizeEvent and returns
// a slice of non-sensitive forensic notes describing which fields were sanitized or truncated.
func SanitizeEventWithReport(e Event) (Event, []string) {
	var notes []string

	e.ActorID, notes = sanitizeActorUUID(e.ActorID, notes)
	e.ActorEmail, notes = sanitizeOptionalField("actor_email", e.ActorEmail, MaxActorEmailLen, MaxActorEmailLen*4, notes)
	e.ActorRole, notes = sanitizeOptionalField("actor_role", e.ActorRole, MaxActorRoleLen, MaxActorRoleLen*4, notes)
	e.Action, notes = sanitizeField("action", e.Action, MaxActionLen, MaxActionLen*4, notes)
	e.TargetType, notes = sanitizeField("target_type", e.TargetType, MaxTargetTypeLen, MaxTargetTypeLen*4, notes)
	e.TargetID, notes = sanitizeOptionalField("target_id", e.TargetID, MaxTargetIDLen, MaxTargetIDLen*4, notes)
	e.Description, notes = sanitizeField("description", e.Description, MaxDescriptionLen, MaxDescriptionLen*4, notes)
	e.IPAddress, notes = sanitizeOptionalField("ip_address", e.IPAddress, MaxIPAddressLen, MaxIPAddressLen, notes)
	e.UserAgent, notes = sanitizeOptionalField("user_agent", e.UserAgent, MaxUserAgentLen, MaxUserAgentLen*4, notes)
	e.DeviceID, notes = sanitizeOptionalField("device_id", e.DeviceID, MaxDeviceIDLen, MaxDeviceIDLen, notes)

	if len(notes) > 0 {
		copiedMeta := make(map[string]any, len(e.Metadata)+2)
		for k, v := range e.Metadata {
			copiedMeta[k] = v
		}
		copiedMeta["_audit_sanitized"] = true
		copiedMeta["_audit_sanitized_fields"] = notes
		e.Metadata = copiedMeta
	}

	return e, notes
}

func sanitizeActorUUID(s *string, notes []string) (*string, []string) {
	if s == nil {
		return nil, notes
	}
	trimmed := strings.TrimSpace(*s)
	if trimmed == "" {
		return nil, notes
	}
	parsed, err := uuid.Parse(trimmed)
	if err != nil {
		notes = append(notes, "actor_id:invalid_uuid_rejected")
		return nil, notes
	}
	canonical := parsed.String()
	return &canonical, notes
}

func sanitizeOptionalField(field string, s *string, maxChars, maxBytes int, notes []string) (*string, []string) {
	if s == nil {
		return nil, notes
	}
	cleaned, updatedNotes := sanitizeField(field, *s, maxChars, maxBytes, notes)
	return &cleaned, updatedNotes
}

func sanitizeField(field, s string, maxChars, maxBytes int, notes []string) (string, []string) {
	if strings.Contains(s, "\x00") {
		s = strings.ReplaceAll(s, "\x00", "")
		notes = append(notes, field+":nul_stripped")
	}
	if !utf8.ValidString(s) {
		s = strings.ToValidUTF8(s, "")
		notes = append(notes, field+":invalid_utf8_stripped")
	}

	origChars := utf8.RuneCountInString(s)
	origBytes := len(s)

	if (maxChars > 0 && origChars > maxChars) || (maxBytes > 0 && origBytes > maxBytes) {
		s = truncateUTF8OnRuneBoundary(s, maxChars, maxBytes)
		notes = append(notes, fmt.Sprintf("%s:truncated(chars=%d->%d,bytes=%d->%d)", field, origChars, utf8.RuneCountInString(s), origBytes, len(s)))
	}

	return s, notes
}

// truncateUTF8OnRuneBoundary truncates a valid UTF-8 string so that it exceeds neither
// maxChars (runes) nor maxBytes (UTF-8 bytes), never slicing halfway through a multibyte character.
func truncateUTF8OnRuneBoundary(s string, maxChars, maxBytes int) string {
	charCount := 0
	byteIdx := 0
	for byteIdx < len(s) {
		r, width := utf8.DecodeRuneInString(s[byteIdx:])
		if r == utf8.RuneError && width == 0 {
			break
		}
		if maxChars > 0 && charCount+1 > maxChars {
			break
		}
		if maxBytes > 0 && byteIdx+width > maxBytes {
			break
		}
		byteIdx += width
		charCount++
	}
	return s[:byteIdx]
}
