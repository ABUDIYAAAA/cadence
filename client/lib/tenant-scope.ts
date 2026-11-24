import type {
  EventUserDetailResponse,
  EventUserListResponse,
} from "@/types/events";
import type {
  PaginatedUsersResponse,
  UserDetailResponse,
} from "@/types/users";

export interface TenantScope {
  eventSlug: string | null;
  competitionSlug: string | null;
}

const EVENT_STORAGE_KEY = "cadence_active_event_slug";
const COMPETITION_STORAGE_KEY_PREFIX = "cadence_active_competition_slug:";

const EVENT_SCOPED_PREFIXES = [
  "/tickets",
  "/notifications",
  "/audit-logs",
  "/competitions",
] as const;

const COMPETITION_SCOPED_PREFIXES = ["/teams", "/judging"] as const;

let currentScope: TenantScope = loadInitialScope();
const listeners = new Set<(scope: TenantScope) => void>();

function loadInitialScope(): TenantScope {
  if (typeof window === "undefined") {
    return { eventSlug: null, competitionSlug: null };
  }
  try {
    const eventSlug = window.localStorage.getItem(EVENT_STORAGE_KEY) || null;
    const competitionSlug = eventSlug
      ? window.localStorage.getItem(`${COMPETITION_STORAGE_KEY_PREFIX}${eventSlug}`) || null
      : null;
    return { eventSlug, competitionSlug };
  } catch {
    return { eventSlug: null, competitionSlug: null };
  }
}

function persistScope(scope: TenantScope): void {
  if (typeof window === "undefined") return;
  try {
    if (scope.eventSlug) {
      window.localStorage.setItem(EVENT_STORAGE_KEY, scope.eventSlug);
      if (scope.competitionSlug) {
        window.localStorage.setItem(
          `${COMPETITION_STORAGE_KEY_PREFIX}${scope.eventSlug}`,
          scope.competitionSlug
        );
      } else {
        window.localStorage.removeItem(
          `${COMPETITION_STORAGE_KEY_PREFIX}${scope.eventSlug}`
        );
      }
    } else {
      window.localStorage.removeItem(EVENT_STORAGE_KEY);
    }
  } catch {}
}

export function getSavedCompetitionForEvent(eventSlug: string): string | null {
  if (typeof window === "undefined" || !eventSlug) return null;
  try {
    return (
      window.localStorage.getItem(`${COMPETITION_STORAGE_KEY_PREFIX}${eventSlug}`) ||
      null
    );
  } catch {
    return null;
  }
}

export function getTenantScope(): TenantScope {
  return currentScope;
}

export function setTenantScope(next: Partial<TenantScope>): TenantScope {
  const updated: TenantScope = {
    eventSlug:
      next.eventSlug !== undefined ? next.eventSlug : currentScope.eventSlug,
    competitionSlug:
      next.competitionSlug !== undefined
        ? next.competitionSlug
        : currentScope.competitionSlug,
  };
  currentScope = updated;
  persistScope(updated);
  listeners.forEach((listener) => listener(updated));
  return updated;
}

export function clearTenantScope(): void {
  currentScope = { eventSlug: null, competitionSlug: null };
  persistScope(currentScope);
  listeners.forEach((listener) => listener(currentScope));
}

export function subscribeTenantScope(
  listener: (scope: TenantScope) => void
): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

function matchesPrefix(pathname: string, prefix: string): boolean {
  return (
    pathname === prefix ||
    pathname.startsWith(`${prefix}/`) ||
    pathname.startsWith(`${prefix}?`)
  );
}

/**
 * Pure URL rewriter that maps relative feature endpoints onto the active
 * `/e/{eventSlug}` or `/e/{eventSlug}/c/{competitionSlug}` v2 hierarchy.
 */
export function resolveScopedPath(
  url: string,
  scope: TenantScope = currentScope
): string {
  if (!url || !url.startsWith("/") || url.startsWith("/e/")) {
    return url;
  }

  const { eventSlug, competitionSlug } = scope;
  if (!eventSlug) {
    return url;
  }

  const encEvent = encodeURIComponent(eventSlug);

  for (const prefix of COMPETITION_SCOPED_PREFIXES) {
    if (matchesPrefix(url, prefix)) {
      if (competitionSlug) {
        const encComp = encodeURIComponent(competitionSlug);
        return `/e/${encEvent}/c/${encComp}${url}`;
      }
      if (prefix === "/judging") {
        return `/e/${encEvent}${url}`;
      }
      return url;
    }
  }

  for (const prefix of EVENT_SCOPED_PREFIXES) {
    if (matchesPrefix(url, prefix)) {
      return `/e/${encEvent}${url}`;
    }
  }

  return url;
}

export function normalizeEventUser(
  u: EventUserDetailResponse
): UserDetailResponse {
  return {
    id: u.id,
    email: u.email,
    name: u.name,
    phone_number: u.phone_number,
    avatar_url: u.avatar_url,
    role: u.role,
    team_id: u.team_id,
    team_name: u.team_name,
    is_leader: Boolean(u.is_leader),
    is_active: !u.is_banned,
    is_banned: u.is_banned,
    banned_at: u.banned_at,
    created_at: u.created_at,
    checkin_status: u.checkin_status,
    checked_in_at: u.checked_in_at,
  };
}

export function normalizeEventUsersPage(
  res: EventUserListResponse
): PaginatedUsersResponse {
  const limit = res.limit > 0 ? res.limit : 25;
  const totalCount = res.total ?? 0;
  return {
    data: (res.users ?? []).map(normalizeEventUser),
    total_count: totalCount,
    page: res.page > 0 ? res.page : 1,
    limit,
    total_pages: Math.max(1, Math.ceil(totalCount / limit)),
  };
}
