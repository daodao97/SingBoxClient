package dns

import "context"

type transportKey struct{}

func contextWithTransportName(ctx context.Context, transportName string) context.Context {
	return context.WithValue(ctx, transportKey{}, transportName)
}

func transportNameFromContext(ctx context.Context) (string, bool) {
	value, loaded := ctx.Value(transportKey{}).(string)
	return value, loaded
}
