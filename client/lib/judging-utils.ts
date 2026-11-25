import { toCsv } from "./csv.ts";
import type {
  MySubmission,
  SlotAssignment,
  SubmissionRow,
} from "../types/judging.ts";

export interface SubmissionViewState {
  visible: boolean;
  blocked: boolean;
  canEdit: boolean;
  hasSavedContent: boolean;
  blockedReason: string;
}

export function resolveSubmissionViewState(
  data?: MySubmission | null
): SubmissionViewState {
  if (!data || !data.has_team) {
    return {
      visible: false,
      blocked: false,
      canEdit: false,
      hasSavedContent: false,
      blockedReason: "",
    };
  }

  const hasSavedLinks = Object.values(data.links ?? {}).some(
    (v) => typeof v === "string" && v.trim().length > 0
  );
  const hasSavedContent =
    Boolean(data.project_name && data.project_name.trim().length > 0) ||
    hasSavedLinks;
  const blocked = !data.can_submit;

  return {
    visible: true,
    blocked,
    canEdit: data.can_submit,
    hasSavedContent,
    blockedReason: blocked
      ? data.submit_blocked_reason ||
        "Submissions are currently closed for your team."
      : "",
  };
}

export const SUBMISSION_CSV_HEADERS = [
  "Team",
  "Project name",
  "Repository link",
  "Demo link",
  "Slides link",
] as const;

export function formatSubmissionsCsv(
  submissions: ReadonlyArray<SubmissionRow>
): string {
  const rows = submissions.map((r) => [
    r.team_name ?? "",
    r.project_name ?? "",
    r.repository ?? "",
    r.demo ?? "",
    r.slides ?? "",
  ]);
  return toCsv([...SUBMISSION_CSV_HEADERS], rows);
}

export const TIMER_EXTENSION_PRESETS = [
  { label: "+1m", seconds: 60 },
  { label: "+2m", seconds: 120 },
  { label: "+5m", seconds: 300 },
] as const;

export function isValidTimerExtensionSeconds(seconds: number): boolean {
  return Number.isInteger(seconds) && seconds >= 30 && seconds <= 900;
}

export function buildSlotAssignments(
  teamIds: ReadonlyArray<string>
): SlotAssignment[] {
  return teamIds.map((teamId, index) => ({
    team_id: teamId,
    sequence_order: index + 1,
  }));
}
