package competitions

import "context"

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

// WithCompetitionContext stores competition context into context.Context.
func WithCompetitionContext(ctx context.Context, compCtx *Context) context.Context {
	return context.WithValue(ctx, competitionContextKey, compCtx)
}

// FromContext extracts the CompetitionContext from request context.
func FromContext(ctx context.Context) (*Context, bool) {
	c, ok := ctx.Value(competitionContextKey).(*Context)
	return c, ok
}
