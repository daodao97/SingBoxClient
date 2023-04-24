package atomic

import (
	"sync/atomic"

	"github.com/sagernet/sing/common"
)

type TypedValue[T any] struct {
	value atomic.Value
}

func (t *TypedValue[T]) Load() T {
	value := t.value.Load()
	if value == nil {
		return common.DefaultValue[T]()
	}
	return value.(T)
}

func (t *TypedValue[T]) Store(value T) {
	t.value.Store(value)
}

func (t *TypedValue[T]) Swap(new T) T {
	old := t.value.Swap(new)
	if old == nil {
		return common.DefaultValue[T]()
	}
	return old.(T)
}

func (t *TypedValue[T]) CompareAndSwap(old, new T) bool {
	return t.value.CompareAndSwap(old, new)
}
