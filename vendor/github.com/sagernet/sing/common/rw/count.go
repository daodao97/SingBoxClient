package rw

import (
	"io"
	"sync/atomic"
)

type ReadCounter struct {
	io.Reader
	count int64
}

func (r *ReadCounter) Read(p []byte) (n int, err error) {
	n, err = r.Reader.Read(p)
	if n > 0 {
		atomic.AddInt64(&r.count, int64(n))
	}
	return
}

func (r *ReadCounter) Count() int64 {
	return r.count
}

func (r *ReadCounter) Reset() {
	atomic.StoreInt64(&r.count, 0)
}
