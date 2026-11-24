import { describe, it, beforeEach } from "node:test";
import assert from "node:assert/strict";
import {
  resolveScopedPath,
  getTenantScope,
  setTenantScope,
  clearTenantScope,
  normalizeEventUsersPage,
} from "../tenant-scope.ts";

describe("Seam 1: resolveScopedPath & tenantScope", () => {
  beforeEach(() => {
    clearTenantScope();
  });

  it("leaves global routes untouched even when event and competition are active", () => {
    const scope = { eventSlug: "devclub-2026", competitionSlug: "hack-track" };
    assert.equal(resolveScopedPath("/auth/me", scope), "/auth/me");
    assert.equal(resolveScopedPath("/auth/google/login", scope), "/auth/google/login");
    assert.equal(resolveScopedPath("/events", scope), "/events");
    assert.equal(resolveScopedPath("/users", scope), "/users");
    assert.equal(resolveScopedPath("/users/123/blacklist", scope), "/users/123/blacklist");
  });

  it("does not double-prefix paths already scoped under /e/{eventSlug}", () => {
    const scope = { eventSlug: "devclub-2026", competitionSlug: "hack-track" };
    assert.equal(
      resolveScopedPath("/e/devclub-2026/users", scope),
      "/e/devclub-2026/users"
    );
    assert.equal(
      resolveScopedPath("/e/other-event/c/track-1/teams", scope),
      "/e/other-event/c/track-1/teams"
    );
  });

  it("rewrites event-scoped modules (/tickets, /notifications, /audit-logs, /competitions) to /e/{eventSlug}", () => {
    const scope = { eventSlug: "devclub-2026", competitionSlug: "hack-track" };
    assert.equal(
      resolveScopedPath("/tickets/me", scope),
      "/e/devclub-2026/tickets/me"
    );
    assert.equal(
      resolveScopedPath("/notifications/inbox", scope),
      "/e/devclub-2026/notifications/inbox"
    );
    assert.equal(
      resolveScopedPath("/audit-logs/export", scope),
      "/e/devclub-2026/audit-logs/export"
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
  });

  it("falls back to event-scoped /e/{eventSlug}/judging when eventSlug is set without competitionSlug", () => {
    const scope = { eventSlug: "devclub-2026", competitionSlug: null };
    assert.equal(
      resolveScopedPath("/judging/rounds", scope),
      "/e/devclub-2026/judging/rounds"
    );
  });

  it("stores, updates, and clears active tenant scope in memory", () => {
    assert.deepEqual(getTenantScope(), { eventSlug: null, competitionSlug: null });
    setTenantScope({ eventSlug: "fest-26", competitionSlug: "ai-cup" });
    assert.deepEqual(getTenantScope(), {
      eventSlug: "fest-26",
      competitionSlug: "ai-cup",
    });
    clearTenantScope();
    assert.deepEqual(getTenantScope(), { eventSlug: null, competitionSlug: null });
  });

  it("normalizes event-scoped user pages into PaginatedUsersResponse with active/ban flags and total_pages", () => {
    const page = normalizeEventUsersPage({
      users: [
        {
          id: "u-1",
          email: "alice@event.com",
          name: "Alice",
          global_role: "user",
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
    assert.equal(page.page, 2);
    assert.equal(page.limit, 25);
    assert.equal(page.total_pages, 3);
    assert.equal(page.data.length, 1);
    assert.equal(page.data[0].is_active, false);
    assert.equal(page.data[0].is_banned, true);
    assert.equal(page.data[0].is_leader, true);
  });
});
