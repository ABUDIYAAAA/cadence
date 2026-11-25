import { describe, it } from "node:test";
import assert from "node:assert/strict";
import {
  BROADCAST_ROLE_OPTIONS,
  BROADCAST_TARGET_MODES,
  buildBroadcastRequest,
  formatBroadcastTargetSummary,
} from "../notifications-utils.ts";

describe("Seam 3: Notifications Broadcast Target Parity", () => {
  it("exposes all 6 recipient target modes and all 5 event roles including owner", () => {
    assert.deepEqual(
      BROADCAST_TARGET_MODES.map((m) => m.id),
      ["broadcast", "competition", "role", "team", "user", "attendance"]
    );
    assert.deepEqual(
      BROADCAST_ROLE_OPTIONS.map((r) => r.value),
      ["participant", "judge", "volunteer", "admin", "owner"]
    );
  });

  it("validates and builds broadcast requests across event, competition, role, team, user, and attendance modes", () => {
    // 1. Validation failures
    assert.deepEqual(
      buildBroadcastRequest({
        title: "   ",
        body: "Hello",
        targetMode: "broadcast",
      }),
      { error: "Title is required." }
    );
    assert.deepEqual(
      buildBroadcastRequest({
        title: "Update",
        body: "  ",
        targetMode: "broadcast",
      }),
      { error: "Message body is required." }
    );
    assert.deepEqual(
      buildBroadcastRequest({
        title: "Track Update",
        body: "Check your room",
        targetMode: "competition",
        targetId: "",
      }),
      { error: "Please select a competition from the list." }
    );
    assert.deepEqual(
      buildBroadcastRequest({
        title: "Check-in reminder",
        body: "Head to main desk",
        targetMode: "attendance",
        targetId: "",
      }),
      { error: "Please select an attendance session from the list." }
    );

    // 2. Whole-event broadcast
    assert.deepEqual(
      buildBroadcastRequest({
        title: "  Doors Open  ",
        body: "  Welcome to the event!  ",
        targetMode: "broadcast",
        actionUrl: " /dashboard ",
        iconUrl: "   ",
      }),
      {
        payload: {
          title: "Doors Open",
          body: "Welcome to the event!",
          target_type: "broadcast",
          target_id: undefined,
          action_url: "/dashboard",
          icon_url: undefined,
        },
      }
    );

    // 3. Competition-scoped broadcast (targets team members in that competition)
    assert.deepEqual(
      buildBroadcastRequest({
        title: "AI Track Pitching",
        body: "Round 1 starts in Room 202.",
        targetMode: "competition",
        targetId: "comp-uuid-ai",
      }),
      {
        payload: {
          title: "AI Track Pitching",
          body: "Round 1 starts in Room 202.",
          target_type: "competition",
          target_id: "comp-uuid-ai",
          action_url: undefined,
          icon_url: undefined,
        },
      }
    );

    // 4. Role broadcast (including owner)
    assert.deepEqual(
      buildBroadcastRequest({
        title: "Owner Sync",
        body: "Meet in green room",
        targetMode: "role",
        roleSelection: "owner",
      }),
      {
        payload: {
          title: "Owner Sync",
          body: "Meet in green room",
          target_type: "role",
          target_id: "owner",
          action_url: undefined,
          icon_url: undefined,
        },
      }
    );

    // 5. Attendance present/absent broadcast
    assert.deepEqual(
      buildBroadcastRequest({
        title: "Missing Check-in",
        body: "Please scan at the desk",
        targetMode: "attendance",
        attendanceMode: "absent",
        targetId: "sess-uuid-1",
      }),
      {
        payload: {
          title: "Missing Check-in",
          body: "Please scan at the desk",
          target_type: "attendance_absent",
          target_id: "sess-uuid-1",
          action_url: undefined,
          icon_url: undefined,
        },
      }
    );
  });

  it("formats human-readable recipient summary labels for preview and history", () => {
    assert.equal(
      formatBroadcastTargetSummary({ targetMode: "broadcast" }),
      "Everyone in event"
    );
    assert.equal(
      formatBroadcastTargetSummary({
        targetMode: "competition",
        competitionName: "AI & Robotics",
      }),
      "Competition: AI & Robotics"
    );
    assert.equal(
      formatBroadcastTargetSummary({
        targetMode: "role",
        roleSelection: "owner",
      }),
      "Role: owner"
    );
    assert.equal(
      formatBroadcastTargetSummary({
        targetMode: "team",
        teamName: "Alpha Squad",
      }),
      "Squad: Alpha Squad"
    );
    assert.equal(
      formatBroadcastTargetSummary({
        targetMode: "user",
        userName: "Alice",
      }),
      "User: Alice"
    );
    assert.equal(
      formatBroadcastTargetSummary({
        targetMode: "attendance",
        attendanceMode: "present",
        sessionName: "Night 1 Check-In",
      }),
      "Attendance (present): Night 1 Check-In"
    );
  });
});
