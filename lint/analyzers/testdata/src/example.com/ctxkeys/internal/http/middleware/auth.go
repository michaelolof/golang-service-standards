package middleware

import (
	"context"

	"example.com/ctxkeys/internal/domain"
)

type requestKey struct{}

func withRequest(ctx context.Context) context.Context {
	return context.WithValue(ctx, requestKey{}, 1) // want `context.WithValue appears only in context.go, inside a WithX accessor \(CTX-2\)`
}

func tenant(ctx context.Context) any {
	return ctx.Value(domain.TenantKey{}) // want `a context key is an unexported struct{} type`
}

var _, _ = withRequest, tenant
