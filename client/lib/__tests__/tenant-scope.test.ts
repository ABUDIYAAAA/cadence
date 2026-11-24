import { describe, it, beforeEach } from "node:test";
import assert from "node:assert/strict";
import {
  resolveScopedPath,
  getTenantScope,
  setTenantScope,
  clearTenantScope,
  normalizeEventUsersPage,
} from "../tenant-scope.ts";
import {
  isValidCompetitionSlug,
  validateCompetitionInput,
  resolveNextActiveCompetitionSlug,
  encodeTeamAssignmentFilter,
  parseTeamAssignmentFilter,
} from "../competition-utils.ts";

describe("Seam 1: Competition Slug Validation & Tenant Scope Resolution", () => {
  beforeEach(() => {
    clearTenantScope();
  });

  it("accepts valid competition slugs matching backend rules (2-64 chars, lowercase alphanumeric with single hyphens)", () => {
    for (const slug of [
      "main",
      "hack-2026",
      "devclub-hackathon",
      "ai-challenge-1",
      "h1",
      "a".repeat(64),
    ]) {
      assert.equal(
        isValidCompetitionSlug(slug),
        true,
        `expected ${slug} to be valid`
      );
    }
  });

  it("rejects invalid competition slugs (too short, too long, uppercase, underscores, leading/trailing/double hyphens)", () => {
    for (const slug of [
      "",
      "a",
      "a".repeat(65),
      "Hack-2026",
      "hack_2026",
      "-hack",
      "hack-",
      "hack--2026",
      "hack 2026",
      "hack.2026",
    ]) {
      assert.equal(
        isValidCompetitionSlug(slug),
        false,
        `expected ${slug} to be invalid`
      );
    }
  });

  it("validates competition name, slug, and description limits", () => {
    assert.equal(
      validateCompetitionInput({ name: "AI & Robotics", slug: "ai-robotics" }),
      null
    );
    assert.equal(
      validateCompetitionInput({ name: "A", slug: "ai-robotics" }),
      "Competition name must be at least 2 characters."
    );
    assert.equal(
      validateCompetitionInput({
        name: "A".repeat(256),
        slug: "ai-robotics",
      }),
      "Competition name cannot exceed 255 characters."
    );
    assert.equal(
      validateCompetitionInput({ name: "AI Track", slug: "a" }),
      "Competition slug must be between 2 and 64 characters."
    );
    assert.equal(
      validateCompetitionInput({ name: "AI Track", slug: "ai_track" }),
      "Slug must contain only lowercase letters, numbers, and single hyphens."
    );
    assert.equal(
      validateCompetitionInput({
        name: "AI Track",
        slug: "ai-track",
        description: "x".repeat(2001),
      }),
      "Description cannot exceed 2000 characters."
    );
  });

  it("rewrites event-scoped modules (/tickets, /notifications, /audit-logs, /competitions) to /e/{eventSlug}", () => {
    const scope = { eventSlug: "devclub-2026", competitionSlug: "hack-track" };
    assert.equal(
      resolveScopedPath("/tickets/me", scope),
      "/e/devclub-2026/tickets/me"
    );
    assert.equal(
      resolveScopedPath("/competitions", scope),
      "/e/devclub-2026/competitions"
    );
  });

  it("rewrites competition-scoped modules (/teams, /judging) to /e/{eventSlug}/c/{competitionSlug}", () => {
    const scope = { eventSlug: "devclub-2026", competitionSlug: "hack-track" };
    assert.equal(
      resolveScopedPath("/teams/my-team", scope),
      "/e/devclub-2026/c/hack-track/teams/my-team"
    );
    assert.equal(
      resolveScopedPath("/judging/rounds/r1/leaderboard", scope),
      "/e/devclub-2026/c/hack-track/judging/rounds/r1/leaderboard"
    );

    const noCompScope = { eventSlug: "devclub-2026", competitionSlug: null };
    assert.equal(resolveScopedPath("/teams/my-team", noCompScope), "/teams/my-team");
    assert.equal(
      resolveScopedPath("/judging/volunteer/schedule", noCompScope),
      "/judging/volunteer/schedule"
    );
  });

  it("stores, updates, and clears active tenant scope in memory", () => {
    assert.deepEqual(getTenantScope(), { eventSlug: null, competitionSlug: null });
    setTenantScope({ eventSlug: "fest-26", competitionSlug: "ai-cup" });
    assert.deepEqual(getTenantScope(), {
      eventSlug: "fest-26",
      competitionSlug: "ai-cup",
    });
    setTenantScope({ competitionSlug: "robotics-cup" });
    assert.deepEqual(getTenantScope(), {
      eventSlug: "fest-26",
      competitionSlug: "robotics-cup",
    });
    clearTenantScope();
    assert.deepEqual(getTenantScope(), { eventSlug: null, competitionSlug: null });
  });

  it("normalizes event-scoped user pages into PaginatedUsersResponse", () => {
    const page = normalizeEventUsersPage({
      users: [
        {
          id: "u-1",
          email: "alice@event.com",
          name: "Alice",
          role: "participant",
          is_banned: true,
          banned_at: "2026-11-24T10:00:00Z",
          team_id: "t-1",
          team_name: "Alpha",
          is_leader: true,
          created_at: "2026-11-24T09:00:00Z",
          checkin_status: "checked_in",
        },
      ],
      total: 51,
      page: 2,
      limit: 25,
    });

    assert.equal(page.total_count, 51);
    assert.equal(page.total_pages, 3);
    assert.equal(page.data[0].is_active, false);
    assert.equal(page.data[0].is_banned, true);
  });

  it("computes the next active competition slug when a competition is renamed or deleted", () => {
    const comps = [
      { slug: "main" },
      { slug: "ai-robotics" },
      { slug: "cyber-ctf" },
    ];

    assert.equal(
      resolveNextActiveCompetitionSlug({
        currentActiveSlug: "ai-robotics",
        competitions: comps,
        editedOldSlug: "ai-robotics",
        editedNewSlug: "ai-ml",
      }),
      "ai-ml"
    );

    assert.equal(
      resolveNextActiveCompetitionSlug({
        currentActiveSlug: "main",
        competitions: comps,
        editedOldSlug: "ai-robotics",
        editedNewSlug: "ai-ml",
      }),
      "main"
    );

    assert.equal(
      resolveNextActiveCompetitionSlug({
        currentActiveSlug: "main",
        competitions: comps,
        deletedSlug: "main",
      }),
      "ai-robotics"
    );

    assert.equal(
      resolveNextActiveCompetitionSlug({
        currentActiveSlug: "main",
        competitions: [{ slug: "main" }],
        deletedSlug: "main",
      }),
      null
    );
  });

  it("encodes and parses event user competition team-assignment filter values", () => {
    assert.equal(encodeTeamAssignmentFilter({}), "");
    assert.equal(
      encodeTeamAssignmentFilter({ no_team: true }),
      "unassigned:any"
    );
    assert.equal(
      encodeTeamAssignmentFilter({
        no_team: true,
        competition_id: "comp-uuid-1",
      }),
      "unassigned:comp-uuid-1"
    );

    assert.deepEqual(parseTeamAssignmentFilter(""), {
      no_team: undefined,
      competition_id: undefined,
    });
    assert.deepEqual(parseTeamAssignmentFilter("unassigned:any"), {
      no_team: true,
      competition_id: undefined,
    });
    assert.deepEqual(parseTeamAssignmentFilter("unassigned:comp-uuid-1"), {
      no_team: true,
      competition_id: "comp-uuid-1",
    });
  });
});
