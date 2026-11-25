import { describe, it } from "node:test";
import assert from "node:assert/strict";
import {
  resolveSubmissionViewState,
  formatSubmissionsCsv,
  TIMER_EXTENSION_PRESETS,
  isValidTimerExtensionSeconds,
  buildSlotAssignments,
} from "../judging-utils.ts";

describe("Seam 2: Judging Operations, Submissions & Migration 000010 Alignment", () => {
  it("resolves participant submission view state so gated/blocked teams see the block reason and any saved submission in read-only mode", () => {
    // 1. Not loaded or participant has no team -> hidden
    assert.deepEqual(resolveSubmissionViewState(null), {
      visible: false,
      blocked: false,
      canEdit: false,
      hasSavedContent: false,
      blockedReason: "",
    });
    assert.deepEqual(
      resolveSubmissionViewState({
        has_team: false,
        project_name: "",
        links: {},
        can_submit: false,
      }),
      {
        visible: false,
        blocked: false,
        canEdit: false,
        hasSavedContent: false,
        blockedReason: "",
      }
    );

    // 2. Has team, gate blocked, with existing saved submission -> visible in read-only mode with blocked banner!
    assert.deepEqual(
      resolveSubmissionViewState({
        has_team: true,
        team_name: "Alpha",
        project_name: "SmartCampus",
        links: { github: "https://github.com/alpha/smartcampus" },
        can_submit: false,
        submit_blocked_reason:
          'No member of your team was present at "Night 1 Attendance".',
      }),
      {
        visible: true,
        blocked: true,
        canEdit: false,
        hasSavedContent: true,
        blockedReason:
          'No member of your team was present at "Night 1 Attendance".',
      }
    );

    // 3. Has team, gate blocked, no saved submission yet -> visible with blocked reason so team knows why they cannot submit
    assert.deepEqual(
      resolveSubmissionViewState({
        has_team: true,
        team_name: "Beta",
        project_name: "",
        links: {},
        can_submit: false,
        submit_blocked_reason: "Submissions are currently closed.",
      }),
      {
        visible: true,
        blocked: true,
        canEdit: false,
        hasSavedContent: false,
        blockedReason: "Submissions are currently closed.",
      }
    );

    // 4. Has team, gate cleared -> visible and editable
    assert.deepEqual(
      resolveSubmissionViewState({
        has_team: true,
        team_name: "Gamma",
        project_name: "RoboNav",
        links: {},
        can_submit: true,
      }),
      {
        visible: true,
        blocked: false,
        canEdit: true,
        hasSavedContent: true,
        blockedReason: "",
      }
    );
  });

  it("exports all 5 submission columns (Team, Project name, Repository link, Demo link, Slides link) to CSV", () => {
    const csv = formatSubmissionsCsv([
      {
        team_name: "Alpha",
        project_name: "SmartCampus",
        repository: "https://github.com/alpha/smart",
        demo: "https://smart.demo.app",
        slides: "https://slides.com/alpha",
      },
      {
        team_name: "Beta",
        project_name: "",
        repository: "",
        demo: "",
        slides: "",
      },
    ]);

    const lines = csv.split("\r\n");
    assert.equal(
      lines[0],
      "Team,Project name,Repository link,Demo link,Slides link"
    );
    assert.equal(
      lines[1],
      "Alpha,SmartCampus,https://github.com/alpha/smart,https://smart.demo.app,https://slides.com/alpha"
    );
    assert.equal(lines[2], "Beta,,,,");
  });

  it("validates pitch timer extension presets against backend 30..900 seconds constraint", () => {
    assert.ok(TIMER_EXTENSION_PRESETS.length >= 3);
    for (const preset of TIMER_EXTENSION_PRESETS) {
      assert.equal(
        isValidTimerExtensionSeconds(preset.seconds),
        true,
        `expected preset ${preset.label} (${preset.seconds}s) to be valid`
      );
    }
    assert.equal(isValidTimerExtensionSeconds(29), false);
    assert.equal(isValidTimerExtensionSeconds(30), true);
    assert.equal(isValidTimerExtensionSeconds(900), true);
    assert.equal(isValidTimerExtensionSeconds(901), false);
    assert.equal(isValidTimerExtensionSeconds(120.5), false);
  });

  it("builds slot assignments without legacy per-slot submission_links after Migration 000010", () => {
    const slots = buildSlotAssignments(["team-1", "team-2", "team-3"]);
    assert.deepEqual(slots, [
      { team_id: "team-1", sequence_order: 1 },
      { team_id: "team-2", sequence_order: 2 },
      { team_id: "team-3", sequence_order: 3 },
    ]);
    for (const slot of slots) {
      assert.equal("submission_links" in slot, false);
    }
  });
});
