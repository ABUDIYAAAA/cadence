import type { EventRole, UserEventRole } from "@/types/auth";

export type OAuthCallbackResult =
  | {
      status: "success";
      events: UserEventRole[];
      redirectPath: string;
    }
  | {
      status: "error";
      code: string;
      message: string;
    };

const VALID_ROLES = new Set<EventRole>([
  "owner",
  "admin",
  "judge",
  "volunteer",
  "participant",
]);

const ERROR_MESSAGES: Record<string, string> = {
  access_denied: "Google sign-in was cancelled or denied.",
  missing_state_or_code:
    "Incomplete Google sign-in response. Please try signing in again.",
  invalid_oauth_state:
    "Your sign-in session expired or became invalid. Please try again.",
  account_deactivated:
    "Your user access has been blacklisted. Contact an administrator.",
  oauth_exchange_failed:
    "Could not verify your authorization token with Google. Please try again.",
  oauth_profile_failed:
    "Could not retrieve your user details from Google. Please try again.",
  authentication_failed:
    "Authentication failed due to a server error. Please try again.",
};

export function isSafeRelativeRedirect(
  path: string | null | undefined
): path is string {
  if (!path) return false;
  return (
    path.startsWith("/") &&
    !path.startsWith("//") &&
    !path.includes("\\")
  );
}

function isAuthLoopPath(path: string): boolean {
  return (
    path === "/" ||
    path.startsWith("/login") ||
    path.startsWith("/auth/login") ||
    path.startsWith("/auth/callback")
  );
}

export function resolvePostOAuthRedirect(
  rawRedirect: string | null | undefined
): string {
  if (!isSafeRelativeRedirect(rawRedirect) || isAuthLoopPath(rawRedirect)) {
    return "/events";
  }
  return rawRedirect;
}

export function buildGoogleLoginUrl(
  apiBaseUrl: string,
  redirectPath?: string | null
): string {
  const base = apiBaseUrl.replace(/\/+$/, "");
  const safeRedirect =
    isSafeRelativeRedirect(redirectPath) && !isAuthLoopPath(redirectPath)
      ? redirectPath
      : "/auth/callback";
  return `${base}/auth/google/login?redirect=${encodeURIComponent(safeRedirect)}`;
}

export function getOAuthErrorMessage(code?: string | null): string {
  if (!code) return ERROR_MESSAGES.authentication_failed;
  return (
    ERROR_MESSAGES[code] ||
    `Google authentication failed (${code.replace(/_/g, " ")}).`
  );
}

function parseEventsParam(raw: string | null): UserEventRole[] {
  if (!raw) return [];
  try {
    const parsed: unknown = JSON.parse(raw);
    if (!Array.isArray(parsed)) return [];
    return parsed
      .filter((item): item is Record<string, unknown> =>
        Boolean(item && typeof item === "object")
      )
      .map((item) => {
        const role =
          typeof item.role === "string" &&
          VALID_ROLES.has(item.role as EventRole)
            ? (item.role as EventRole)
            : "participant";
        return {
          event_id: typeof item.event_id === "string" ? item.event_id : "",
          event_slug:
            typeof item.event_slug === "string" ? item.event_slug : "",
          event_name:
            typeof item.event_name === "string" ? item.event_name : "",
          role,
        };
      })
      .filter((item) => item.event_slug !== "");
  } catch {
    return [];
  }
}

export function parseOAuthCallbackParams(
  params: URLSearchParams | { get(name: string): string | null }
): OAuthCallbackResult {
  const successParam = params.get("success");
  const errorParam = params.get("error");

  if (successParam !== "true") {
    const code =
      errorParam ||
      (successParam === "false" ? "authentication_failed" : "missing_state_or_code");
    return {
      status: "error",
      code,
      message: getOAuthErrorMessage(code),
    };
  }

  return {
    status: "success",
    events: parseEventsParam(params.get("events")),
    redirectPath: resolvePostOAuthRedirect(params.get("redirect")),
  };
}
