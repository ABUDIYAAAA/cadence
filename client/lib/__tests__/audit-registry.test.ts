import { describe, it } from "node:test";
import assert from "node:assert/strict";
import {
  AUDIT_ACTION_OPTIONS,
  AUDIT_TARGET_TYPE_OPTIONS,
  getAuditActionBadgeVariant,
  hasActiveAuditFilters,
  getUserTableColumnVisibility,
} from "../audit-registry.ts";

describe("Seam 5: Users, Events & Audit Trail V2 Sync", () => {
  it("includes all v2 event, competition, ticket, judging, team, user, auth, notification, and system actions", () => {
    const actions = new Set(AUDIT_ACTION_OPTIONS.map((o) => o.value));

    const requiredV2Actions = [
      // Auth (Google OAuth v2)
      "auth.login_success",
      "auth.login_failure",
      "auth.logout",
      // Event
      "event.create",
      "event.update",
      "event.delete",
      "event.transfer_ownership",
      "event.member_assign",
      "event.member_role_update",
      "event.member_remove",
      "event.member.ban",
      "event.member.unban",
      // Competition
      "competition.create",
      "competition.update",
      "competition.delete",
      // User
      "user.create",
      "user.bulk_import",
      "user.update",
      "user.delete",
      // Team
      "team.create",
      "team.update",
      "team.delete",
      "team.add_member",
      "team.remove_member",
      "team.join",
      "team.leave",
      "team.kick",
      "team.freeze",
      "team.unfreeze",
      "team.freeze_all",
      "team.unfreeze_all",
      "team.regenerate_code",
      "team.settings_update",
      // Ticket & Scan Sessions
      "ticket.issued",
      "ticket.released",
      "ticket.check_in",
      "ticket.code_check_in",
      "ticket.duplicate_scan",
      "ticket.scan_rejected",
      "ticket.session_create",
      "ticket.session_toggle",
      "ticket.session_code_generate",
      "ticket.session_code_clear",
      "ticket.session_scan",
      "ticket.settings_update",
      // Judging
      "judging.round_create",
      "judging.round_activate",
      "judging.round_end",
      "judging.round_delete",
      "judging.panel_create",
      "judging.panel_judges_update",
      "judging.round_duration_update",
      "judging.slots_configure",
      "judging.slot_start",
      "judging.timer_pause",
      "judging.timer_resume",
      "judging.timer_extend",
      "judging.score_lock",
      "judging.score_unlock",
      "judging.slot_complete",
      "judging.slot_move",
      "judging.slot_timing",
      "judging.slot_queue",
      "judging.slot_unqueue",
      "judging.team_criterion_set",
      "judging.team_criterion_delete",
      "judging.team_criteria_import",
      "judging.slot_links_update",
      // Notifications & System
      "notification.broadcast",
      "notification.subscribe",
      "notification.unsubscribe",
      "system.backup_failed",
      "system.backup_recovered",
    ];

    for (const reqAction of requiredV2Actions) {
      assert.equal(
        actions.has(reqAction),
        true,
        `expected AUDIT_ACTION_OPTIONS to contain ${reqAction}`
      );
    }

    // Obsolete v1 password actions must not be listed
    for (const obsolete of [
      "auth.password_setup",
      "auth.password_forgot",
      "auth.password_reset",
      "user.send_invite",
      "user.reset_trigger",
    ]) {
      assert.equal(
        actions.has(obsolete),
        false,
        `expected obsolete v1 action ${obsolete} to be removed`
      );
    }
  });

  it("includes event and competition target types in AUDIT_TARGET_TYPE_OPTIONS", () => {
    const targets = new Set(AUDIT_TARGET_TYPE_OPTIONS.map((o) => o.value));
    for (const target of [
      "event",
      "competition",
      "user",
      "team",
      "ticket",
      "scan_session",
      "session",
      "auth",
      "judging_round",
      "judging_slot",
      "judging_panel",
      "notification",
      "system",
    ]) {
      assert.equal(
        targets.has(target),
        true,
        `expected target type ${target} to be present`
      );
    }
  });

  it("computes audit action badge variants and detects active audit filters including competition_id", () => {
    assert.equal(getAuditActionBadgeVariant("event.member.ban"), "coral");
    assert.equal(getAuditActionBadgeVariant("competition.delete"), "coral");
    assert.equal(getAuditActionBadgeVariant("competition.create"), "blue");
    assert.equal(getAuditActionBadgeVariant("event.update"), "subtle");

    assert.equal(hasActiveAuditFilters({ page: 1, limit: 20 }), false);
    assert.equal(
      hasActiveAuditFilters({ page: 1, limit: 20, competition_id: "comp-1" }),
      true
    );
  });

  it("hides event-only Team and Check-in columns when UserTable / UserDetailDialog is in platform mode after Migration 000009", () => {
    assert.deepEqual(getUserTableColumnVisibility("event"), {
      showTeam: true,
      showCheckIn: true,
    });
    assert.deepEqual(getUserTableColumnVisibility("platform"), {
      showTeam: false,
      showCheckIn: false,
    });
  });
});
