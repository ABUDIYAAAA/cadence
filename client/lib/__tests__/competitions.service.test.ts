import { describe, it } from "node:test";
import assert from "node:assert/strict";
import { createCompetitionsService } from "../../services/competitions.service.ts";

describe("Seam 2: competitionsService HTTP seam", () => {
  it("lists competitions for an event at GET /e/{eventSlug}/competitions and safely defaults to empty array", async () => {
    const calls: Array<{ method: string; url: string }> = [];
    const service = createCompetitionsService({
      get: async <T>(url: string) => {
        calls.push({ method: "GET", url });
        return {
          data: {
            competitions: [
              {
                id: "comp-1",
                event_id: "ev-1",
                slug: "main",
                name: "Main Track",
                description: "Primary track",
                created_at: "2026-11-24T00:00:00Z",
                updated_at: "2026-11-24T00:00:00Z",
              },
            ],
            total: 1,
          } as unknown as T,
        };
      },
      post: async () => ({ data: {} as never }),
      patch: async () => ({ data: {} as never }),
      delete: async () => ({ data: {} as never }),
    });

    const list = await service.listByEvent("devclub-2026");
    assert.equal(calls.length, 1);
    assert.deepEqual(calls[0], {
      method: "GET",
      url: "/e/devclub-2026/competitions",
    });
    assert.equal(list.length, 1);
    assert.equal(list[0].slug, "main");
  });

  it("fetches single competition metadata and caller role at GET /e/{eventSlug}/c/{competitionSlug}", async () => {
    const calls: Array<{ method: string; url: string }> = [];
    const service = createCompetitionsService({
      get: async <T>(url: string) => {
        calls.push({ method: "GET", url });
        return {
          data: {
            id: "comp-2",
            event_id: "ev-1",
            slug: "ai-robotics",
            name: "AI & Robotics",
            description: "Hardware and ML",
            my_role: "admin",
            created_at: "2026-11-24T00:00:00Z",
            updated_at: "2026-11-24T00:00:00Z",
          } as unknown as T,
        };
      },
      post: async () => ({ data: {} as never }),
      patch: async () => ({ data: {} as never }),
      delete: async () => ({ data: {} as never }),
    });

    const comp = await service.getCompetition("devclub-2026", "ai-robotics");
    assert.deepEqual(calls[0], {
      method: "GET",
      url: "/e/devclub-2026/c/ai-robotics",
    });
    assert.equal(comp.slug, "ai-robotics");
    assert.equal(comp.my_role, "admin");
  });

  it("creates, updates, and deletes competitions with URL-encoded event and competition slugs", async () => {
    const calls: Array<{ method: string; url: string; body?: unknown }> = [];
    const service = createCompetitionsService({
      get: async () => ({ data: {} as never }),
      post: async <T>(url: string, body?: unknown) => {
        calls.push({ method: "POST", url, body });
        return {
          data: {
            id: "comp-3",
            event_id: "ev-1",
            slug: "cyber-sec",
            name: "Cyber Security",
            description: "CTF",
            created_at: "2026-11-24T00:00:00Z",
            updated_at: "2026-11-24T00:00:00Z",
          } as unknown as T,
        };
      },
      patch: async <T>(url: string, body?: unknown) => {
        calls.push({ method: "PATCH", url, body });
        return {
          data: {
            id: "comp-3",
            event_id: "ev-1",
            slug: "cyber-ctf",
            name: "Cyber CTF",
            description: "Updated CTF",
            created_at: "2026-11-24T00:00:00Z",
            updated_at: "2026-11-24T01:00:00Z",
          } as unknown as T,
        };
      },
      delete: async <T>(url: string) => {
        calls.push({ method: "DELETE", url });
        return {
          data: { message: "competition deleted successfully" } as unknown as T,
        };
      },
    });

    const created = await service.createCompetition("fest-2026", {
      slug: "cyber-sec",
      name: "Cyber Security",
      description: "CTF",
      min_team_size: 2,
      max_team_size: 4,
    });
    assert.equal(created.slug, "cyber-sec");
    assert.deepEqual(calls[0], {
      method: "POST",
      url: "/e/fest-2026/competitions",
      body: {
        slug: "cyber-sec",
        name: "Cyber Security",
        description: "CTF",
        min_team_size: 2,
        max_team_size: 4,
      },
    });

    const updated = await service.updateCompetition("fest-2026", "cyber-sec", {
      slug: "cyber-ctf",
      name: "Cyber CTF",
    });
    assert.equal(updated.slug, "cyber-ctf");
    assert.deepEqual(calls[1], {
      method: "PATCH",
      url: "/e/fest-2026/c/cyber-sec",
      body: { slug: "cyber-ctf", name: "Cyber CTF" },
    });

    const deleted = await service.deleteCompetition("fest-2026", "cyber-ctf");
    assert.equal(deleted.message, "competition deleted successfully");
    assert.deepEqual(calls[2], {
      method: "DELETE",
      url: "/e/fest-2026/c/cyber-ctf",
    });
  });
});
