package events

import (
	"context"

	"devclubxnst.com/cadence/internal/modules/auth"
)

type contextKey string

const eventContextKey contextKey = "event_context"

// Context stores the resolved event and the calling user's role in it.
type Context struct {
	EventID string `json:"event_id"`
	Slug    string `json:"slug"`
	Name    string `json:"name"`
	UserID  string `json:"user_id"`
	Role    string `json:"role"`
}

// WithEventContext stores event context into context.Context and registers the active tenant role.
func WithEventContext(ctx context.Context, eventCtx *Context) context.Context {
	ctx = context.WithValue(ctx, eventContextKey, eventCtx)
	if eventCtx != nil && eventCtx.Role != "" {
		ctx = auth.WithActiveTenantRole(ctx, eventCtx.Role)
	}
	return ctx
}

// FromContext extracts the EventContext from request context.
func FromContext(ctx context.Context) (*Context, bool) {
	c, ok := ctx.Value(eventContextKey).(*Context)
	return c, ok
}
