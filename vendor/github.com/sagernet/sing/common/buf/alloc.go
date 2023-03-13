package buf

// Inspired by https://github.com/xtaci/smux/blob/master/alloc.go

import (
	"errors"
	"math/bits"
	"strconv"
	"sync"
)

var DefaultAllocator = newDefaultAllocer()

type Allocator interface {
	Get(size int) []byte
	Put(buf []byte) error
}

// defaultAllocator for incoming frames, optimized to prevent overwriting after zeroing
type defaultAllocator struct {
	buffers []sync.Pool
}

// NewAllocator initiates a []byte allocator for frames less than 65536 bytes,
// the waste(memory fragmentation) of space allocation is guaranteed to be
// no more than 50%.
func newDefaultAllocer() Allocator {
	alloc := new(defaultAllocator)
	alloc.buffers = make([]sync.Pool, 17) // 1B -> 64K
	for k := range alloc.buffers {
		i := k
		alloc.buffers[k].New = func() any {
			return make([]byte, 1<<uint32(i))
		}
	}
	return alloc
}

// Get a []byte from pool with most appropriate cap
func (alloc *defaultAllocator) Get(size int) []byte {
	if size <= 0 || size > 65536 {
		panic("alloc bad size: " + strconv.Itoa(size))
	}

	bits := msb(size)
	if size == 1<<bits {
		return alloc.buffers[bits].Get().([]byte)[:size]
	}

	return alloc.buffers[bits+1].Get().([]byte)[:size]
}

// Put returns a []byte to pool for future use,
// which the cap must be exactly 2^n
func (alloc *defaultAllocator) Put(buf []byte) error {
	bits := msb(cap(buf))
	if cap(buf) == 0 || cap(buf) > 65536 || cap(buf) != 1<<bits {
		return errors.New("allocator Put() incorrect buffer size")
	}

	//nolint
	//lint:ignore SA6002 ignore temporarily
	alloc.buffers[bits].Put(buf)
	return nil
}

// msb return the pos of most significant bit
func msb(size int) uint16 {
	return uint16(bits.Len32(uint32(size)) - 1)
}
