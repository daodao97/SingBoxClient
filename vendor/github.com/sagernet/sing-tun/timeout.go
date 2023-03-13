package tun

import "context"

type needTimeoutKey struct{}

func ContextWithNeedTimeout(ctx context.Context, need bool) context.Context {
	return context.WithValue(ctx, (*needTimeoutKey)(nil), need)
}

func NeedTimeoutFromContext(ctx context.Context) bool {
	need, _ := ctx.Value((*needTimeoutKey)(nil)).(bool)
	return need
}
