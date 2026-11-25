import type { AuditLogFilter } from "../types/audit.ts";
import { getUserTableColumnVisibility } from "./competition-utils.ts";

export { getUserTableColumnVisibility };

export const AUDIT_ACTION_OPTIONS: ReadonlyArray<{
  value: string;
  label: string;
}> = [
  { value: "", label: "All Actions" },
  // Auth (Google OAuth v2)
  { value: "auth.login_success", label: "[Auth] Login Success" },
  { value: "auth.login_failure", label: "[Auth] Login Failure" },
  { value: "auth.logout", label: "[Auth] Logout" },
  // Event
  { value: "event.create", label: "[Event] Event Created" },
  { value: "event.update", label: "[Event] Event Updated" },
  { value: "event.delete", label: "[Event] Event Deleted" },
  { value: "event.transfer_ownership", label: "[Event] Ownership Transferred" },
  { value: "event.member_assign", label: "[Event] Member Added" },
  { value: "event.member_role_update", label: "[Event] Member Role Updated" },
  { value: "event.member_remove", label: "[Event] Member Removed" },
  { value: "event.member.ban", label: "[Event] Member Banned" },
  { value: "event.member.unban", label: "[Event] Member Unbanned" },
  // Competition
  { value: "competition.create", label: "[Competition] Competition Created" },
  { value: "competition.update", label: "[Competition] Competition Updated" },
  { value: "competition.delete", label: "[Competition] Competition Deleted" },
  // User (Platform)
  { value: "user.create", label: "[User] Account Created" },
  { value: "user.bulk_import", label: "[User] Bulk Imported" },
  { value: "user.update", label: "[User] Record Updated" },
  { value: "user.delete", label: "[User] Account Deleted" },
  // Team
  { value: "team.create", label: "[Team] Team Created" },
  { value: "team.update", label: "[Team] Team Updated" },
  { value: "team.delete", label: "[Team] Team Deleted" },
  { value: "team.add_member", label: "[Team] Member Added" },
  { value: "team.remove_member", label: "[Team] Member Removed" },
  { value: "team.join", label: "[Team] Member Joined" },
  { value: "team.leave", label: "[Team] Member Left" },
  { value: "team.kick", label: "[Team] Member Kicked" },
  { value: "team.freeze", label: "[Team] Team Frozen" },
  { value: "team.unfreeze", label: "[Team] Team Unfrozen" },
  { value: "team.freeze_all", label: "[Team] All Teams Frozen" },
  { value: "team.unfreeze_all", label: "[Team] All Teams Unfrozen" },
  { value: "team.regenerate_code", label: "[Team] Join Code Regenerated" },
  { value: "team.settings_update", label: "[Team] Settings Updated" },
  // Ticket & Scan Sessions
  { value: "ticket.issued", label: "[Ticket] Pass Emailed" },
  { value: "ticket.released", label: "[Ticket] Passes Released" },
  { value: "ticket.check_in", label: "[Ticket] Check-In Recorded" },
  { value: "ticket.code_check_in", label: "[Ticket] Self Check-In by Code" },
  { value: "ticket.duplicate_scan", label: "[Ticket] Duplicate Scan" },
  { value: "ticket.scan_rejected", label: "[Ticket] Scan Refused" },
  { value: "ticket.session_create", label: "[Ticket] Scan Session Created" },
  { value: "ticket.session_toggle", label: "[Ticket] Scan Session Opened/Closed" },
  { value: "ticket.session_code_generate", label: "[Ticket] Session Code Generated" },
  { value: "ticket.session_code_clear", label: "[Ticket] Session Code Cleared" },
  { value: "ticket.session_scan", label: "[Ticket] Attendance/Exit Scan" },
  { value: "ticket.settings_update", label: "[Ticket] Ticket Settings Updated" },
  // Judging
  { value: "judging.round_create", label: "[Judging] Round Created" },
  { value: "judging.round_activate", label: "[Judging] Round Activated" },
  { value: "judging.round_end", label: "[Judging] Round Ended" },
  { value: "judging.round_delete", label: "[Judging] Round Deleted" },
  { value: "judging.round_duration_update", label: "[Judging] Round Duration Updated" },
  { value: "judging.panel_create", label: "[Judging] Panel Created" },
  { value: "judging.panel_judges_update", label: "[Judging] Panel Judges Updated" },
  { value: "judging.slots_configure", label: "[Judging] Slots Configured" },
  { value: "judging.slot_start", label: "[Judging] Slot Started" },
  { value: "judging.timer_pause", label: "[Judging] Timer Paused" },
  { value: "judging.timer_resume", label: "[Judging] Timer Resumed" },
  { value: "judging.timer_extend", label: "[Judging] Timer Extended" },
  { value: "judging.score_lock", label: "[Judging] Score Locked" },
  { value: "judging.score_unlock", label: "[Judging] Score Unlocked" },
  { value: "judging.slot_complete", label: "[Judging] Slot Completed" },
  { value: "judging.slot_move", label: "[Judging] Slot Moved" },
  { value: "judging.slot_timing", label: "[Judging] Slot Timing Updated" },
  { value: "judging.slot_queue", label: "[Judging] Slot Queued" },
  { value: "judging.slot_unqueue", label: "[Judging] Slot Unqueued" },
  { value: "judging.team_criterion_set", label: "[Judging] Team Criterion Set" },
  { value: "judging.team_criterion_delete", label: "[Judging] Team Criterion Removed" },
  { value: "judging.team_criteria_import", label: "[Judging] Team Criteria Imported" },
  { value: "judging.slot_links_update", label: "[Judging] Submission Links Updated" },
  // Notification
  { value: "notification.broadcast", label: "[Notification] Broadcast Sent" },
  { value: "notification.subscribe", label: "[Notification] Push Subscribed" },
  { value: "notification.unsubscribe", label: "[Notification] Push Unsubscribed" },
  // System
  { value: "system.backup_failed", label: "[System] Backup Failed" },
  { value: "system.backup_recovered", label: "[System] Backup Recovered" },
] as const;

export const AUDIT_TARGET_TYPE_OPTIONS: ReadonlyArray<{
  value: string;
  label: string;
}> = [
  { value: "", label: "All Target Types" },
  { value: "event", label: "Event" },
  { value: "competition", label: "Competition" },
  { value: "user", label: "User" },
  { value: "team", label: "Team" },
  { value: "ticket", label: "Ticket" },
  { value: "scan_session", label: "Scan Session" },
  { value: "session", label: "Session" },
  { value: "auth", label: "Auth" },
  { value: "judging_round", label: "Judging Round" },
  { value: "judging_slot", label: "Judging Slot" },
  { value: "judging_panel", label: "Judging Panel" },
  { value: "notification", label: "Notification" },
  { value: "system", label: "System" },
] as const;

export function getAuditActionBadgeVariant(
  action: string
): "coral" | "blue" | "subtle" {
  if (
    action.includes("delete") ||
    action.includes("remove") ||
    action.includes("ban") && !action.includes("unban") ||
    action.includes("kick") ||
    action.includes("failure") ||
    action.includes("failed") ||
    action.includes("duplicate") ||
    action.includes("rejected")
  ) {
    return "coral";
  }
  if (
    action.includes("create") ||
    action.includes("assign") ||
    action.includes("success") ||
    action.includes("activate") ||
    action.includes("unban")
  ) {
    return "blue";
  }
  return "subtle";
}

export function hasActiveAuditFilters(filters: AuditLogFilter): boolean {
  return Boolean(
    filters.search ||
      filters.action ||
      filters.target_type ||
      filters.competition_id ||
      filters.from_date ||
      filters.to_date
  );
}
