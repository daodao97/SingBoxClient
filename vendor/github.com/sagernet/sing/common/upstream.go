package common

type WithUpstream interface {
	Upstream() any
}

func Cast[T any](obj any) (T, bool) {
	if c, ok := obj.(T); ok {
		return c, true
	}
	if u, ok := obj.(WithUpstream); ok {
		return Cast[T](u.Upstream())
	}
	return DefaultValue[T](), false
}

func MustCast[T any](obj any) T {
	value, ok := Cast[T](obj)
	if !ok {
		// make panic
		return obj.(T)
	}
	return value
}
