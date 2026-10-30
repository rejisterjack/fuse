package proxy

import (
	"context"

	"github.com/rejisterjack/fuse/internal/budget"
)

type scopeKey struct{}

func withScope(ctx context.Context, s budget.Scope) context.Context {
	return context.WithValue(ctx, scopeKey{}, s)
}

func scopeFromContext(ctx context.Context) (budget.Scope, bool) {
	s, ok := ctx.Value(scopeKey{}).(budget.Scope)
	return s, ok
}
