package audit

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"devclubxnst.com/cadence/internal/modules/auth"
	"devclubxnst.com/cadence/internal/modules/competitions"
	"devclubxnst.com/cadence/internal/modules/events"
	"devclubxnst.com/cadence/internal/ratelimit"
	"devclubxnst.com/cadence/pkg/amqpqueue"
	pkgaudit "devclubxnst.com/cadence/pkg/audit"
	"devclubxnst.com/cadence/pkg/utils"
)

type captureBroker struct {
	payloads [][]byte
}

func (b *captureBroker) Publish(_ context.Context, body []byte, _ ...amqpqueue.PublishOption) error {
	b.payloads = append(b.payloads, append([]byte(nil), body...))
	return nil
}

func (b *captureBroker) StartConsumers(_ context.Context, _ int, _ amqpqueue.Handler) {}
func (b *captureBroker) IsHealthy() bool                                             { return true }
func (b *captureBroker) Close() error                                                { return nil }

func TestQueueRecordActionEnrichesAllContextFields(t *testing.T) {
	broker := &captureBroker{}
	q := NewQueueWithBroker(broker, nil, nil)

	const (
		eventID  = "11111111-1111-1111-1111-111111111111"
		compID   = "22222222-2222-2222-2222-222222222222"
		actorID  = "33333333-3333-3333-3333-333333333333"
		targetID = "44444444-4444-4444-4444-444444444444"
		deviceID = "dev-cookie-abc123"
	)

	ctx := context.Background()
	ctx = auth.WithUserClaims(ctx, &utils.AccessTokenClaims{
		UserID: actorID,
		Email:  "judge@cadence.local",
		Role:   "user",
	})
	ctx = auth.WithActiveTenantRole(ctx, "judge")
	ctx = events.WithEventContext(ctx, &events.Context{
		EventID: eventID,
		Slug:    "spring-hack",
		UserID:  actorID,
		Role:    "judge",
	})
	ctx = competitions.WithCompetitionContext(ctx, &competitions.Context{
		CompetitionID: compID,
		EventID:       eventID,
		Slug:          "ai-track",
		UserID:        actorID,
		Role:          "judge",
	})
	ctx = WithRequestMeta(ctx, RequestMeta{
		IPAddress: "203.0.113.42",
		UserAgent: "CadenceWeb/2.0",
	})
	ctx = context.WithValue(ctx, ratelimit.DeviceIDContextKey, deviceID)

	tid := targetID
	q.RecordAction(ctx, pkgaudit.ActionJudgingScoreLock, pkgaudit.TargetJudgingSlot, &tid, "Locked scorecard", map[string]any{"slot_id": tid})

	if len(broker.payloads) != 1 {
		t.Fatalf("expected 1 published audit message, got %d", len(broker.payloads))
	}

	var got Event
	if err := json.Unmarshal(broker.payloads[0], &got); err != nil {
		t.Fatalf("unmarshal published event: %v", err)
	}

	assertPtr := func(field string, ptr *string, want string) {
		t.Helper()
		if ptr == nil || *ptr != want {
			var actual string
			if ptr != nil {
				actual = *ptr
			}
			t.Errorf("%s: got %q (nil=%v), want %q", field, actual, ptr == nil, want)
		}
	}

	assertPtr("EventID", got.EventID, eventID)
	assertPtr("CompetitionID", got.CompetitionID, compID)
	assertPtr("ActorID", got.ActorID, actorID)
	assertPtr("ActorEmail", got.ActorEmail, "judge@cadence.local")
	assertPtr("ActorRole", got.ActorRole, "judge")
	assertPtr("IPAddress", got.IPAddress, "203.0.113.42")
	assertPtr("UserAgent", got.UserAgent, "CadenceWeb/2.0")
	assertPtr("DeviceID", got.DeviceID, deviceID)
	assertPtr("TargetID", got.TargetID, targetID)
	if got.Action != pkgaudit.ActionJudgingScoreLock {
		t.Errorf("Action: got %q, want %q", got.Action, pkgaudit.ActionJudgingScoreLock)
	}
	if got.TargetType != pkgaudit.TargetJudgingSlot {
		t.Errorf("TargetType: got %q, want %q", got.TargetType, pkgaudit.TargetJudgingSlot)
	}
}

func TestQueueRecordPreservesCallerOverridesAndSanitizesContext(t *testing.T) {
	broker := &captureBroker{}
	q := NewQueueWithBroker(broker, nil, nil)

	const (
		ctxEventID      = "11111111-1111-1111-1111-111111111111"
		overrideEventID = "99999999-9999-9999-9999-999999999999"
		overrideActorID = "88888888-8888-8888-8888-888888888888"
	)

	ctx := events.WithEventContext(context.Background(), &events.Context{EventID: ctxEventID})
	ctx = WithRequestMeta(ctx, RequestMeta{
		IPAddress: "198.51.100.7",
		UserAgent: "Evil\x00Agent/" + strings.Repeat("a", pkgaudit.MaxUserAgentLen+50),
	})

	evID := overrideEventID
	aID := overrideActorID
	email := "override@cadence.local"
	q.Record(ctx, Event{
		EventID:     &evID,
		ActorID:     &aID,
		ActorEmail:  &email,
		Action:      pkgaudit.ActionEventCreate,
		TargetType:  pkgaudit.TargetEvent,
		TargetID:    &evID,
		Description: "Created new event",
	})

	if len(broker.payloads) != 1 {
		t.Fatalf("expected 1 published message, got %d", len(broker.payloads))
	}
	var got Event
	if err := json.Unmarshal(broker.payloads[0], &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.EventID == nil || *got.EventID != overrideEventID {
		t.Fatalf("expected caller EventID override %q, got %v", overrideEventID, got.EventID)
	}
	if got.ActorID == nil || *got.ActorID != overrideActorID {
		t.Fatalf("expected caller ActorID override %q, got %v", overrideActorID, got.ActorID)
	}
	if got.IPAddress == nil || *got.IPAddress != "198.51.100.7" {
		t.Fatalf("expected IPAddress from context, got %v", got.IPAddress)
	}
	if got.UserAgent == nil || strings.Contains(*got.UserAgent, "\x00") || len(*got.UserAgent) > pkgaudit.MaxUserAgentLen {
		t.Fatalf("expected sanitized UserAgent within bounds, got len=%d", len(*got.UserAgent))
	}
}

func TestRequestMetaMiddlewareStripsPortAndBindsUA(t *testing.T) {
	var captured RequestMeta
	var ok bool
	h := RequestMetaMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured, ok = RequestMetaFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.0.2.10:54321"
	req.Header.Set("User-Agent", "Mozilla/5.0 Test")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if !ok {
		t.Fatal("expected RequestMeta in context")
	}
	if captured.IPAddress != "192.0.2.10" {
		t.Errorf("IPAddress: got %q, want %q", captured.IPAddress, "192.0.2.10")
	}
	if captured.UserAgent != "Mozilla/5.0 Test" {
		t.Errorf("UserAgent: got %q, want %q", captured.UserAgent, "Mozilla/5.0 Test")
	}
}
