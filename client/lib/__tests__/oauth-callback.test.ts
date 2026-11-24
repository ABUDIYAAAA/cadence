import { describe, it } from "node:test";
import assert from "node:assert/strict";
import {
  buildGoogleLoginUrl,
  parseOAuthCallbackParams,
} from "../oauth-callback.ts";

describe("Seam 2: parseOAuthCallbackParams & buildGoogleLoginUrl", () => {
  it("parses a successful OAuth callback with serialized user events and default /events destination", () => {
    const eventsPayload = JSON.stringify([
      {
        event_id: "00000000-0000-0000-0000-000000000001",
        event_slug: "devclub-event",
        event_name: "DevClub Event",
        role: "admin",
      },
    ]);
    const params = new URLSearchParams({
      success: "true",
      events: eventsPayload,
    });

    const result = parseOAuthCallbackParams(params);
    assert.equal(result.status, "success");
    if (result.status === "success") {
      assert.equal(result.redirectPath, "/events");
      assert.equal(result.events.length, 1);
      assert.equal(result.events[0].event_slug, "devclub-event");
      assert.equal(result.events[0].role, "admin");
    }
  });

  it("tolerates malformed or missing events JSON on success without throwing", () => {
    const params = new URLSearchParams({
      success: "true",
      events: "{bad-json",
    });

    const result = parseOAuthCallbackParams(params);
    assert.equal(result.status, "success");
    if (result.status === "success") {
      assert.deepEqual(result.events, []);
    }
  });

  it("sanitizes unsafe open-redirect and self-redirect (/auth/callback, /login) paths back to /events", () => {
    const unsafe1 = parseOAuthCallbackParams(
      new URLSearchParams({
        success: "true",
        redirect: "//evil.example.com/phish",
      })
    );
    const unsafe2 = parseOAuthCallbackParams(
      new URLSearchParams({
        success: "true",
        redirect: "https://evil.example.com",
      })
    );
    const unsafeBackslash = parseOAuthCallbackParams(
      new URLSearchParams({
        success: "true",
        redirect: "/\\evil.example.com",
      })
    );
    const selfLoop = parseOAuthCallbackParams(
      new URLSearchParams({
        success: "true",
        redirect: "/auth/callback",
      })
    );
    const loginLoop = parseOAuthCallbackParams(
      new URLSearchParams({
        success: "true",
        redirect: "/login",
      })
    );
    const safeCustom = parseOAuthCallbackParams(
      new URLSearchParams({
        success: "true",
        redirect: "/events",
      })
    );

    assert.equal(unsafe1.status === "success" && unsafe1.redirectPath, "/events");
    assert.equal(unsafe2.status === "success" && unsafe2.redirectPath, "/events");
    assert.equal(
      unsafeBackslash.status === "success" && unsafeBackslash.redirectPath,
      "/events"
    );
    assert.equal(selfLoop.status === "success" && selfLoop.redirectPath, "/events");
    assert.equal(loginLoop.status === "success" && loginLoop.redirectPath, "/events");
    assert.equal(
      safeCustom.status === "success" && safeCustom.redirectPath,
      "/events"
    );
  });

  it("treats missing success=true parameter as an error when visiting /auth/callback directly", () => {
    const empty = parseOAuthCallbackParams(new URLSearchParams());
    assert.equal(empty.status, "error");
    if (empty.status === "error") {
      assert.equal(empty.code, "missing_state_or_code");
    }
  });

  it("maps backend OAuth error codes to human-readable failure messages", () => {
    const denied = parseOAuthCallbackParams(
      new URLSearchParams({
        success: "false",
        error: "access_denied",
        events: "[]",
      })
    );
    assert.equal(denied.status, "error");
    if (denied.status === "error") {
      assert.equal(denied.code, "access_denied");
      assert.match(denied.message, /cancelled|denied/i);
    }

    const deactivated = parseOAuthCallbackParams(
      new URLSearchParams({
        success: "false",
        error: "account_deactivated",
      })
    );
    assert.equal(deactivated.status, "error");
    if (deactivated.status === "error") {
      assert.equal(deactivated.code, "account_deactivated");
      assert.match(deactivated.message, /blacklisted/i);
    }
  });

  it("builds Google OAuth login URL targeting /auth/callback by default so it works even if FRONTEND_OAUTH_REDIRECT_URL is unset", () => {
    assert.equal(
      buildGoogleLoginUrl("http://localhost:8080/api/v2/"),
      "http://localhost:8080/api/v2/auth/google/login?redirect=%2Fauth%2Fcallback"
    );
    assert.equal(
      buildGoogleLoginUrl("http://localhost:8080/api/v2", "//evil.com"),
      "http://localhost:8080/api/v2/auth/google/login?redirect=%2Fauth%2Fcallback"
    );
    assert.equal(
      buildGoogleLoginUrl("http://localhost:8080/api/v2", "/events"),
      "http://localhost:8080/api/v2/auth/google/login?redirect=%2Fevents"
    );
  });
});
