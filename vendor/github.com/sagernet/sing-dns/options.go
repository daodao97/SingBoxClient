package dns

import "context"

type disableCacheKey struct{}

func ContextWithDisableCache(ctx context.Context, val bool) context.Context {
	return context.WithValue(ctx, (*disableCacheKey)(nil), val)
}

func DisableCacheFromContext(ctx context.Context) bool {
	val := ctx.Value((*disableCacheKey)(nil))
	if val == nil {
		return false
	}
	return val.(bool)
}

type rewriteTTLKey struct{}

func ContextWithRewriteTTL(ctx context.Context, val uint32) context.Context {
	return context.WithValue(ctx, (*rewriteTTLKey)(nil), val)
}

func RewriteTTLFromContext(ctx context.Context) (uint32, bool) {
	val := ctx.Value((*rewriteTTLKey)(nil))
	if val == nil {
		return 0, false
	}
	return val.(uint32), true
}
