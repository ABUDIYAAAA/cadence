package competitions

import (
	"context"

	"devclubxnst.com/cadence/internal/modules/auth"
)

type contextKey string

const competitionContextKey contextKey = "competition_context"

// Context stores the resolved competition and the calling user's role in its parent event.
type Context struct {
	CompetitionID string `json:"competition_id"`
	EventID       string `json:"event_id"`
	Slug          string `json:"slug"`
	Name          string `json:"name"`
	UserID        string `json:"user_id"`
	Role          string `json:"role"`
}

// WithCompetitionContext stores competition context into context.Context and registers the active tenant role.
func WithCompetitionContext(ctx context.Context, compCtx *Context) context.Context {
	ctx = context.WithValue(ctx, competitionContextKey, compCtx)
	if compCtx != nil && compCtx.Role != "" {
		ctx = auth.WithActiveTenantRole(ctx, compCtx.Role)
	}
	return ctx
}

// FromContext extracts the CompetitionContext from request context.
func FromContext(ctx context.Context) (*Context, bool) {
	c, ok := ctx.Value(competitionContextKey).(*Context)
	return c, ok
}
