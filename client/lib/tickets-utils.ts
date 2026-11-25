import type { CreateScanSessionRequest, ScanKind } from "../types/tickets.ts";

export function normalizeCheckInCode(code?: string | null): string {
  return (code ?? "").trim().toUpperCase();
}

export function validateCustomCheckInCode(code?: string | null): string | null {
  const normalized = normalizeCheckInCode(code);
  if (!normalized) return null; // Blank means generate a random 6-char code
  if (normalized.length < 2 || normalized.length > 32) {
    return "Custom check-in code must be between 2 and 32 characters.";
  }
  return null;
}

export function buildCreateScanSessionPayload(params: {
  name: string;
  kind: ScanKind;
  markTeam: boolean;
  competitionId?: string;
  enableCode: boolean;
  customCode?: string;
}):
  | { payload: CreateScanSessionRequest; error?: undefined }
  | { error: string; payload?: undefined } {
  const trimmedName = params.name.trim();
  if (trimmedName.length < 2) {
    return { error: "Session name must be at least 2 characters." };
  }
  if (params.kind === "attendance" && params.markTeam && !params.competitionId) {
    return {
      error: "Select a competition when team-leader scanning is enabled.",
    };
  }

  const normalizedCode = params.enableCode
    ? normalizeCheckInCode(params.customCode)
    : "";
  if (normalizedCode) {
    const codeErr = validateCustomCheckInCode(normalizedCode);
    if (codeErr) {
      return { error: codeErr };
    }
  }

  const payload: CreateScanSessionRequest = {
    name: trimmedName,
    kind: params.kind,
    ...(params.kind === "attendance"
      ? {
          mark_team: params.markTeam,
          competition_id: params.competitionId || undefined,
        }
      : {}),
    ...(params.enableCode ? { generate_code: true } : {}),
    ...(normalizedCode ? { checkin_code: normalizedCode } : {}),
  };

  return { payload };
}
