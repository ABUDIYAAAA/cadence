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
  if (!filters.no_team) return "";
  if (filters.competition_id) {
    return `unassigned:${filters.competition_id}`;
  }
  return "unassigned:any";
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
  return { no_team: undefined, competition_id: undefined };
}
