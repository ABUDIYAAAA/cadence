import type { EventRole, UserResponse } from "@/types/auth";

export interface EffectivePermissions {
  effectiveRole: EventRole | null;
  isDev: boolean;
  isOwner: boolean;
  isAdmin: boolean;
  isJudge: boolean;
  isVolunteer: boolean;
  isParticipant: boolean;
}

export function getDashboardRoute(role?: EventRole | string | null): string {
  switch (role) {
    case "owner":
    case "admin":
      return "/dashboard/admin";
    case "judge":
      return "/dashboard/judge";
    case "volunteer":
      return "/dashboard/volunteer";
    case "participant":
      return "/dashboard/participant";
    default:
      return "/dashboard";
  }
}

export function getAuthenticatedHomeRoute(
  user: UserResponse | null | undefined,
  activeEventSlug?: string | null
): string {
  if (!user) return "/login";
  if (!activeEventSlug) return "/events";

  const { effectiveRole } = resolveEffectivePermissions(user, activeEventSlug);
  if (!effectiveRole) return "/events";

  return getDashboardRoute(effectiveRole);
}

export function resolveEffectivePermissions(
  user: UserResponse | null | undefined,
  activeEventSlug: string | null | undefined
): EffectivePermissions {
  if (!user) {
    return {
      effectiveRole: null,
      isDev: false,
      isOwner: false,
      isAdmin: false,
      isJudge: false,
      isVolunteer: false,
      isParticipant: false,
    };
  }

  const isDev = user.global_role === "dev";
  let effectiveRole: EventRole | null = null;

  if (isDev) {
    effectiveRole = "owner";
  } else if (activeEventSlug) {
    if (Array.isArray(user.events)) {
      effectiveRole =
        user.events.find((e) => e.event_slug === activeEventSlug)?.role ?? null;
    } else {
      effectiveRole = user.role ?? null;
    }
  } else if (!Array.isArray(user.events)) {
    effectiveRole = user.role ?? null;
  }

  const isOwner = isDev || effectiveRole === "owner";
  const isAdmin = isOwner || effectiveRole === "admin";

  return {
    effectiveRole,
    isDev,
    isOwner,
    isAdmin,
    isJudge: effectiveRole === "judge",
    isVolunteer: effectiveRole === "volunteer",
    isParticipant: effectiveRole === "participant",
  };
}
