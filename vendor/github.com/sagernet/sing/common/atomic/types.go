//go:build go1.19

package atomic

import "sync/atomic"

type (
	Bool    = atomic.Bool
	Int32   = atomic.Int32
	Int64   = atomic.Int64
	Uint32  = atomic.Uint32
	Uint64  = atomic.Uint64
	Uintptr = atomic.Uintptr
	Value   = atomic.Value
)

type Pointer[T any] struct {
	atomic.Pointer[T]
}
