package domain

import "context"

type principalKey struct{}

type Principal struct{ ID string }

func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

func PrincipalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok
}

type TenantKey struct{}

func WithTenant(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, TenantKey{}, id) // want `a context key is an unexported struct{} type declared beside its accessors: type xKey struct{} \(CTX-2\)`
}

type idKey int

func withID(ctx context.Context) context.Context {
	return context.WithValue(ctx, idKey(1), 1)
}

var _ = withID
