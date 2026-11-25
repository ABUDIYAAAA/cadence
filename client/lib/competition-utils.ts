import type { UserFilter } from "../types/users.ts";

const COMPETITION_SLUG_REGEX = /^[a-z0-9]+(-[a-z0-9]+)*$/;

export function isValidCompetitionSlug(slug: string): boolean {
  return (
    typeof slug === "string" &&
    slug.length >= 2 &&
    slug.length <= 64 &&
    COMPETITION_SLUG_REGEX.test(slug)
  );
}

export function validateCompetitionInput(input: {
  name: string;
  slug: string;
  description?: string | null;
  min_team_size?: number;
  max_team_size?: number;
}): string | null {
  const name = input.name.trim();
  const slug = input.slug.trim();
  const description = input.description?.trim() ?? "";

  if (name.length < 2) {
    return "Competition name must be at least 2 characters.";
  }
  if (name.length > 255) {
    return "Competition name cannot exceed 255 characters.";
  }
  if (slug.length < 2 || slug.length > 64) {
    return "Competition slug must be between 2 and 64 characters.";
  }
  if (!COMPETITION_SLUG_REGEX.test(slug)) {
    return "Slug must contain only lowercase letters, numbers, and single hyphens.";
  }
  if (description.length > 2000) {
    return "Description cannot exceed 2000 characters.";
  }
  if (
    input.min_team_size === undefined ||
    !Number.isInteger(input.min_team_size) ||
    input.min_team_size < 1 ||
    input.min_team_size > 100
  ) {
    return "Minimum team size must be a whole number from 1 to 100.";
  }
  if (
    input.max_team_size === undefined ||
    !Number.isInteger(input.max_team_size) ||
    input.max_team_size < 1 ||
    input.max_team_size > 100
  ) {
    return "Maximum team size must be a whole number from 1 to 100.";
  }
  if (input.min_team_size > input.max_team_size) {
    return "Minimum team size cannot exceed maximum team size.";
  }
  return null;
}

export function resolveNextActiveCompetitionSlug(params: {
  currentActiveSlug: string | null;
  competitions: ReadonlyArray<{ slug: string }>;
  editedOldSlug?: string;
  editedNewSlug?: string;
  deletedSlug?: string;
}): string | null {
  const {
    currentActiveSlug,
    competitions,
    editedOldSlug,
    editedNewSlug,
    deletedSlug,
  } = params;

  if (editedOldSlug && editedNewSlug) {
    return currentActiveSlug === editedOldSlug
      ? editedNewSlug
      : currentActiveSlug;
  }

  if (deletedSlug) {
    const remaining = competitions.filter((c) => c.slug !== deletedSlug);
    if (currentActiveSlug !== deletedSlug && currentActiveSlug) {
      return currentActiveSlug;
    }
    return remaining.length > 0 ? remaining[0].slug : null;
  }

  return currentActiveSlug;
}

export function encodeTeamAssignmentFilter(filters: {
  no_team?: boolean;
  competition_id?: string;
}): string {
  if (filters.no_team) {
    if (filters.competition_id) {
      return `unassigned:${filters.competition_id}`;
    }
    return "unassigned:any";
  }
  if (filters.competition_id) {
    return `competition:${filters.competition_id}`;
  }
  return "";
}

export function parseTeamAssignmentFilter(value: string): {
  no_team?: boolean;
  competition_id?: string;
} {
  if (value === "unassigned:any") {
    return { no_team: true, competition_id: undefined };
  }
  if (value.startsWith("unassigned:")) {
    const compId = value.slice("unassigned:".length).trim();
    if (compId) {
      return { no_team: true, competition_id: compId };
    }
  }
  if (value.startsWith("competition:")) {
    const compId = value.slice("competition:".length).trim();
    if (compId) {
      return { no_team: undefined, competition_id: compId };
    }
  }
  return { no_team: undefined, competition_id: undefined };
}

export function getUserTableColumnVisibility(mode: "event" | "platform"): {
  showTeam: boolean;
  showCheckIn: boolean;
} {
  const isEvent = mode === "event";
  return {
    showTeam: isEvent,
    showCheckIn: isEvent,
  };
}

/**
 * Injects the active competition ID into event user list filters when no
 * explicit competition filter or "unassigned in any competition" (`no_team: true`
 * with `competition_id: undefined`) filter is selected.
 */
export function resolveEventUserFilters<T extends UserFilter>(
  filters: T,
  activeCompetitionId?: string | null
): T {
  if (filters.competition_id) {
    return filters;
  }
  if (filters.no_team) {
    return filters;
  }
  if (activeCompetitionId) {
    return {
      ...filters,
      competition_id: activeCompetitionId,
    };
  }
  return filters;
}

export interface CompetitionRouteContext {
  competitionSlug: string;
  section: string | null;
}

/**
 * Extracts the competition slug and optional nested section (e.g. "teams", "judging")
 * from `/dashboard/competitions/<slug>[/<section>]` paths.
 */
export function getCompetitionRouteContext(
  pathname: string
): CompetitionRouteContext | null {
  const match = pathname.match(/^\/dashboard\/competitions\/([^/]+)(?:\/([^/]+))?/);
  if (!match) return null;
  let competitionSlug = match[1];
  try {
    competitionSlug = decodeURIComponent(competitionSlug);
  } catch {
    // keep raw segment if malformed
  }
  return {
    competitionSlug,
    section: match[2] ?? null,
  };
}

export function getCompetitionSectionHref(
  competitionSlug: string,
  section: "teams" | "judging"
): string {
  return `/dashboard/competitions/${encodeURIComponent(competitionSlug)}/${section}`;
}
