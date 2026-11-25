package audit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"devclubxnst.com/cadence/internal/modules/events"
)

func TestParseFilterEventScoping(t *testing.T) {
	h := NewHandler(nil)

	t.Run("global dev route honors explicit event_id query param", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/audit-logs?event_id=ev-123&competition_id=comp-456", nil)
		f := h.parseFilter(req)
		if f.EventID != "ev-123" || f.CompetitionID != "comp-456" {
			t.Fatalf("expected event_id=ev-123 competition_id=comp-456, got %+v", f)
		}
	})

	t.Run("global dev route honors platform sentinel", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/audit-logs?event_id=platform", nil)
		f := h.parseFilter(req)
		if f.EventID != "platform" {
			t.Fatalf("expected event_id=platform, got %q", f.EventID)
		}
	})

	t.Run("event-scoped route locks event_id to context and ignores query override", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/e/org-a/audit-logs?event_id=other-org", nil)
		ctx := events.WithEventContext(context.Background(), &events.Context{
			EventID: "org-a-id",
			Slug:    "org-a",
		})
		f := h.parseFilter(req.WithContext(ctx))
		if f.EventID != "org-a-id" {
			t.Fatalf("expected locked event_id=org-a-id, got %q", f.EventID)
		}
	})
}
