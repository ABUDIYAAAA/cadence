import { describe, it } from "node:test";
import assert from "node:assert/strict";
import { createTicketsService } from "../../services/tickets.service.ts";

describe("Seam 3: ticketsService scan sessions, check-in codes, and ticket settings", () => {
  it("creates a competition-linked scan session with mark_team and initial check-in code generation", async () => {
    const calls: Array<{ method: string; url: string; body?: unknown }> = [];
    const service = createTicketsService({
      get: async () => ({ data: {} as never }),
      post: async <T>(url: string, body?: unknown) => {
        calls.push({ method: "POST", url, body });
        return {
          data: {
            id: "sess-1",
            event_id: "ev-1",
            competition_id: "comp-1",
            competition_slug: "main",
            competition_name: "Main Track",
            name: "Night 1 Attendance",
            kind: "attendance",
            is_default: false,
            is_open: true,
            mark_team: true,
            checkin_code: "K9P4X2",
            scanned_count: 0,
            created_at: "2026-11-24T20:00:00Z",
          } as unknown as T,
        };
      },
      patch: async () => ({ data: {} as never }),
      put: async () => ({ data: {} as never }),
      delete: async () => ({ data: {} as never }),
    });

    const created = await service.createSession({
      name: "Night 1 Attendance",
      kind: "attendance",
      mark_team: true,
      competition_id: "comp-1",
      generate_code: true,
    });

    assert.equal(created.competition_id, "comp-1");
    assert.equal(created.checkin_code, "K9P4X2");
    assert.deepEqual(calls[0], {
      method: "POST",
      url: "/tickets/sessions",
      body: {
        name: "Night 1 Attendance",
        kind: "attendance",
        mark_team: true,
        competition_id: "comp-1",
        generate_code: true,
      },
    });
  });

  it("generates/rotates and clears a scan session check-in code, and allows participant self-check-in by code", async () => {
    const calls: Array<{ method: string; url: string; body?: unknown }> = [];
    const service = createTicketsService({
      get: async () => ({ data: {} as never }),
      post: async <T>(url: string, body?: unknown) => {
        calls.push({ method: "POST", url, body });
        return {
          data: {
            id: "sess-1",
            checkin_code: "HACK26",
            success: true,
            message: "Checked in",
          } as unknown as T,
        };
      },
      patch: async () => ({ data: {} as never }),
      put: async () => ({ data: {} as never }),
      delete: async <T>(url: string) => {
        calls.push({ method: "DELETE", url });
        return {
          data: {
            id: "sess-1",
            checkin_code: null,
          } as unknown as T,
        };
      },
    });

    await service.generateSessionCode("sess-1", "HACK26");
    assert.deepEqual(calls[0], {
      method: "POST",
      url: "/tickets/sessions/sess-1/code",
      body: { code: "HACK26" },
    });

    await service.generateSessionCode("sess-1");
    assert.deepEqual(calls[1], {
      method: "POST",
      url: "/tickets/sessions/sess-1/code",
      body: {},
    });

    await service.clearSessionCode("sess-1");
    assert.deepEqual(calls[2], {
      method: "DELETE",
      url: "/tickets/sessions/sess-1/code",
    });

    await service.checkInByCode("HACK26");
    assert.deepEqual(calls[3], {
      method: "POST",
      url: "/tickets/check-in/code",
      body: { code: "HACK26" },
    });
  });

  it("updates ticket settings with internal domains, custom email subject/HTML, and release flag", async () => {
    const calls: Array<{ method: string; url: string; body?: unknown }> = [];
    const service = createTicketsService({
      get: async () => ({ data: {} as never }),
      post: async () => ({ data: {} as never }),
      patch: async () => ({ data: {} as never }),
      put: async <T>(url: string, body?: unknown) => {
        calls.push({ method: "PUT", url, body });
        return {
          data: {
            internal_email_domains: ["college.edu"],
            excluded_domains: ["college.edu"],
            email_subject: "Your Hack 2026 Pass",
            email_html: "<p>Welcome!</p>",
            tickets_released: true,
            source: "database",
          } as unknown as T,
        };
      },
      delete: async () => ({ data: {} as never }),
    });

    const res = await service.updateSettings({
      internal_email_domains: ["college.edu"],
      email_subject: "Your Hack 2026 Pass",
      email_html: "<p>Welcome!</p>",
      tickets_released: true,
    });

    assert.equal(res.email_subject, "Your Hack 2026 Pass");
    assert.deepEqual(calls[0], {
      method: "PUT",
      url: "/tickets/settings",
      body: {
        internal_email_domains: ["college.edu"],
        email_subject: "Your Hack 2026 Pass",
        email_html: "<p>Welcome!</p>",
        tickets_released: true,
      },
    });
  });
});
