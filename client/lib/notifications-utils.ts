import type { BroadcastNotificationRequest } from "../types/notifications.ts";

export type BroadcastTargetMode =
  | "broadcast"
  | "competition"
  | "role"
  | "team"
  | "user"
  | "attendance";

export const BROADCAST_TARGET_MODES: ReadonlyArray<{
  id: BroadcastTargetMode;
  label: string;
}> = [
  { id: "broadcast", label: "Everyone" },
  { id: "competition", label: "By competition" },
  { id: "role", label: "By role" },
  { id: "team", label: "By team" },
  { id: "user", label: "Specific user" },
  { id: "attendance", label: "By attendance" },
] as const;

export const BROADCAST_ROLE_OPTIONS = [
  { value: "participant", label: "Participant (hackers and attendees)" },
  { value: "judge", label: "Judge (evaluators)" },
  { value: "volunteer", label: "Volunteer (staff and coordinators)" },
  { value: "admin", label: "Administrator" },
  { value: "owner", label: "Event owner" },
] as const;

export interface BuildBroadcastInput {
  title: string;
  body: string;
  targetMode: BroadcastTargetMode;
  targetId?: string;
  roleSelection?: string;
  attendanceMode?: "present" | "absent";
  actionUrl?: string;
  iconUrl?: string;
}

export function buildBroadcastRequest(
  input: BuildBroadcastInput
):
  | { payload: BroadcastNotificationRequest; error?: undefined }
  | { error: string; payload?: undefined } {
  const title = input.title.trim();
  const body = input.body.trim();

  if (!title) {
    return { error: "Title is required." };
  }
  if (!body) {
    return { error: "Message body is required." };
  }

  const targetId = input.targetId?.trim() || "";
  const roleSelection = input.roleSelection?.trim() || "participant";
  const attendanceMode = input.attendanceMode ?? "present";

  if (
    (input.targetMode === "competition" ||
      input.targetMode === "team" ||
      input.targetMode === "user" ||
      input.targetMode === "attendance") &&
    !targetId
  ) {
    const label =
      input.targetMode === "attendance"
        ? "an attendance session"
        : `a ${input.targetMode}`;
    return { error: `Please select ${label} from the list.` };
  }

  const effectiveTargetType: BroadcastNotificationRequest["target_type"] =
    input.targetMode === "attendance"
      ? `attendance_${attendanceMode}`
      : input.targetMode;

  const effectiveTargetId =
    input.targetMode === "role"
      ? roleSelection
      : input.targetMode === "broadcast"
      ? undefined
      : targetId || undefined;

  return {
    payload: {
      title,
      body,
      target_type: effectiveTargetType,
      target_id: effectiveTargetId,
      action_url: input.actionUrl?.trim() || undefined,
      icon_url: input.iconUrl?.trim() || undefined,
    },
  };
}

export function formatBroadcastTargetSummary(params: {
  targetMode: BroadcastTargetMode;
  roleSelection?: string;
  attendanceMode?: "present" | "absent";
  competitionName?: string;
  teamName?: string;
  userName?: string;
  sessionName?: string;
  targetId?: string;
}): string {
  switch (params.targetMode) {
    case "broadcast":
      return "Everyone in event";
    case "competition":
      return `Competition: ${params.competitionName || params.targetId || "Not selected"}`;
    case "role":
      return `Role: ${params.roleSelection || "participant"}`;
    case "team":
      return `Squad: ${params.teamName || params.targetId || "Not selected"}`;
    case "user":
      return `User: ${params.userName || params.targetId || "Not selected"}`;
    case "attendance":
      return `Attendance (${params.attendanceMode ?? "present"}): ${
        params.sessionName || params.targetId || "Not selected"
      }`;
  }
}
