import { describe, it } from "node:test";
import assert from "node:assert/strict";
import {
  getAuthenticatedHomeRoute,
  getDashboardRoute,
  resolveEffectivePermissions,
} from "../auth-routes.ts";
import type { UserResponse } from "@/types/auth";

describe("Seam 3: resolveEffectivePermissions & getDashboardRoute", () => {
  it("maps owner and admin roles to /dashboard/admin", () => {
    assert.equal(getDashboardRoute("owner"), "/dashboard/admin");
    assert.equal(getDashboardRoute("admin"), "/dashboard/admin");
    assert.equal(getDashboardRoute("judge"), "/dashboard/judge");
    assert.equal(getDashboardRoute("volunteer"), "/dashboard/volunteer");
    assert.equal(getDashboardRoute("participant"), "/dashboard/participant");
  });

  it("grants global dev full owner and admin permissions across any event", () => {
    const devUser: UserResponse = {
      id: "u-dev",
      email: "dev@cadence.com",
      name: "Dev User",
      global_role: "dev",
      role: "owner",
      events: [],
      created_at: "2026-11-24T00:00:00Z",
    };

    const perms = resolveEffectivePermissions(devUser, "any-event");
    assert.equal(perms.isDev, true);
    assert.equal(perms.effectiveRole, "owner");
    assert.equal(perms.isOwner, true);
    assert.equal(perms.isAdmin, true);
    assert.equal(perms.isJudge, false);

    // Even if a dev user also has a lower-role membership row in user.events,
    // backend RequireMembership always grants dev effective role "owner".
    const devWithParticipantRow: UserResponse = {
      ...devUser,
      events: [
        {
          event_id: "e-1",
          event_slug: "any-event",
          event_name: "Any Event",
          role: "participant",
        },
      ],
    };
    const devPerms = resolveEffectivePermissions(devWithParticipantRow, "any-event");
    assert.equal(devPerms.effectiveRole, "owner");
    assert.equal(devPerms.isOwner, true);
    assert.equal(devPerms.isAdmin, true);
  });

  it("treats an event owner as satisfying both isOwner and isAdmin in that event", () => {
    const user: UserResponse = {
      id: "u-1",
      email: "owner@event.com",
      name: "Event Owner",
      global_role: "user",
      role: "participant",
      events: [
        {
          event_id: "e-1",
          event_slug: "hack-2026",
          event_name: "Hack 2026",
          role: "owner",
        },
        {
          event_id: "e-2",
          event_slug: "other-fest",
          event_name: "Other Fest",
          role: "participant",
        },
      ],
      created_at: "2026-11-24T00:00:00Z",
    };

    const ownerPerms = resolveEffectivePermissions(user, "hack-2026");
    assert.equal(ownerPerms.isDev, false);
    assert.equal(ownerPerms.effectiveRole, "owner");
    assert.equal(ownerPerms.isOwner, true);
    assert.equal(ownerPerms.isAdmin, true);

    const participantPerms = resolveEffectivePermissions(user, "other-fest");
    assert.equal(participantPerms.effectiveRole, "participant");
    assert.equal(participantPerms.isOwner, false);
    assert.equal(participantPerms.isAdmin, false);
    assert.equal(participantPerms.isParticipant, true);
  });

  it("returns null effectiveRole when user is not enrolled in the active event or no event is selected", () => {
    const user: UserResponse = {
      id: "u-2",
      email: "judge@event.com",
      name: "Judge",
      global_role: "user",
      role: "judge",
      events: [
        {
          event_id: "e-1",
          event_slug: "hack-2026",
          event_name: "Hack 2026",
          role: "judge",
        },
      ],
      created_at: "2026-11-24T00:00:00Z",
    };

    const unSelected = resolveEffectivePermissions(user, null);
    assert.equal(unSelected.effectiveRole, null);
    assert.equal(unSelected.isJudge, false);

    const wrongEvent = resolveEffectivePermissions(user, "unjoined-event");
    assert.equal(wrongEvent.effectiveRole, null);
    assert.equal(wrongEvent.isAdmin, false);
  });

  it("routes authenticated users without an active event to /events, and with an active event to their role dashboard", () => {
    const user: UserResponse = {
      id: "u-3",
      email: "admin@event.com",
      name: "Admin",
      global_role: "user",
      role: "admin",
      events: [
        {
          event_id: "e-1",
          event_slug: "hack-2026",
          event_name: "Hack 2026",
          role: "owner",
        },
      ],
      created_at: "2026-11-24T00:00:00Z",
    };

    assert.equal(getAuthenticatedHomeRoute(null, "hack-2026"), "/login");
    assert.equal(getAuthenticatedHomeRoute(user, null), "/events");
    assert.equal(getAuthenticatedHomeRoute(user, "unjoined"), "/events");
    assert.equal(getAuthenticatedHomeRoute(user, "hack-2026"), "/dashboard/admin");
  });
});
