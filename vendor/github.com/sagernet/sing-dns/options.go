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
