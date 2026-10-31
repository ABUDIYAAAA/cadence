package audit

import "devclubxnst.com/cadence/pkg/audit"

// Re-export types and constants from pkg/audit
type Event = audit.Event
type Recorder = audit.Recorder

var (
	SanitizeEvent           = audit.SanitizeEvent
	SanitizeEventWithReport = audit.SanitizeEventWithReport
)

const (
	ActionAuthLoginSuccess  = audit.ActionAuthLoginSuccess
	ActionAuthLoginFailure  = audit.ActionAuthLoginFailure
	ActionAuthLogout        = audit.ActionAuthLogout
	ActionAuthPasswordSetup = audit.ActionAuthPasswordSetup
	ActionAuthPasswordReset = audit.ActionAuthPasswordReset

	ActionUserCreate       = audit.ActionUserCreate
	ActionUserBulkImport   = audit.ActionUserBulkImport
	ActionUserUpdate       = audit.ActionUserUpdate
	ActionUserDelete       = audit.ActionUserDelete
	ActionUserSendInvite   = audit.ActionUserSendInvite
	ActionUserResetTrigger = audit.ActionUserResetTrigger

	ActionTeamCreate       = audit.ActionTeamCreate
	ActionTeamUpdate       = audit.ActionTeamUpdate
	ActionTeamDelete       = audit.ActionTeamDelete
	ActionTeamAddMember    = audit.ActionTeamAddMember
	ActionTeamRemoveMember = audit.ActionTeamRemoveMember
	ActionTeamJoin         = audit.ActionTeamJoin
	ActionTeamLeave        = audit.ActionTeamLeave
	ActionTeamKick         = audit.ActionTeamKick
	ActionTeamFreeze       = audit.ActionTeamFreeze
	ActionTeamUnfreeze     = audit.ActionTeamUnfreeze
	ActionTeamFreezeAll    = audit.ActionTeamFreezeAll
	ActionTeamUnfreezeAll  = audit.ActionTeamUnfreezeAll
	ActionTeamRegenCode    = audit.ActionTeamRegenCode
	ActionTeamSettings     = audit.ActionTeamSettings

	TargetUser    = audit.TargetUser
	TargetTeam    = audit.TargetTeam
	TargetSession = audit.TargetSession
	TargetAuth    = audit.TargetAuth
)
