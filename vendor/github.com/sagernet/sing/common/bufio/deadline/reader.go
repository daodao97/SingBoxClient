package deadline

import (
	"io"
	"os"
	"sync"
	"time"

	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	N "github.com/sagernet/sing/common/network"
)

type Reader struct {
	N.ExtendedReader
	deadline     time.Time
	pipeDeadline pipeDeadline
	cacheAccess  sync.RWMutex
	cached       bool
	cachedBuffer *buf.Buffer
	cachedErr    error
}

func NewReader(reader io.Reader) *Reader {
	return &Reader{ExtendedReader: bufio.NewExtendedReader(reader), pipeDeadline: makePipeDeadline()}
}

func (r *Reader) Read(p []byte) (n int, err error) {
	r.cacheAccess.Lock()
	if r.cached {
		n = copy(p, r.cachedBuffer.Bytes())
		err = r.cachedErr
		r.cachedBuffer.Advance(n)
		if r.cachedBuffer.IsEmpty() {
			r.cachedBuffer.Release()
			r.cached = false
		}
		r.cacheAccess.Unlock()
		return
	}
	r.cacheAccess.Unlock()
	done := make(chan struct{})
	go func() {
		n, err = r.pipeRead(p, r.pipeDeadline.wait())
		close(done)
	}()
	select {
	case <-done:
		return
	case <-r.pipeDeadline.wait():
		return 0, os.ErrDeadlineExceeded
	}
}

func (r *Reader) pipeRead(p []byte, cancel chan struct{}) (n int, err error) {
	r.cacheAccess.Lock()
	defer r.cacheAccess.Unlock()
	buffer := buf.NewSize(len(p))
	n, err = buffer.ReadOnceFrom(r.ExtendedReader)
	if isClosedChan(cancel) {
		r.cached = true
		r.cachedBuffer = buffer
		r.cachedErr = err
	} else {
		n = copy(p, buffer.Bytes())
		buffer.Release()
	}
	return
}

func (r *Reader) ReadBuffer(buffer *buf.Buffer) error {
	r.cacheAccess.Lock()
	if r.cached {
		n := copy(buffer.FreeBytes(), r.cachedBuffer.Bytes())
		err := r.cachedErr
		buffer.Resize(buffer.Start(), n)
		r.cachedBuffer.Advance(n)
		if r.cachedBuffer.IsEmpty() {
			r.cachedBuffer.Release()
			r.cached = false
		}
		r.cacheAccess.Unlock()
		return err
	}
	r.cacheAccess.Unlock()
	done := make(chan struct{})
	var err error
	go func() {
		err = r.pipeReadBuffer(buffer, r.pipeDeadline.wait())
		close(done)
	}()
	select {
	case <-done:
		return err
	case <-r.pipeDeadline.wait():
		return os.ErrDeadlineExceeded
	}
}

func (r *Reader) pipeReadBuffer(buffer *buf.Buffer, cancel chan struct{}) error {
	r.cacheAccess.Lock()
	defer r.cacheAccess.Unlock()
	cacheBuffer := buf.NewSize(buffer.FreeLen())
	err := r.ExtendedReader.ReadBuffer(cacheBuffer)
	if isClosedChan(cancel) {
		r.cached = true
		r.cachedBuffer = cacheBuffer
		r.cachedErr = err
	} else {
		buffer.ReadOnceFrom(cacheBuffer)
		cacheBuffer.Release()
	}
	return err
}

func (r *Reader) SetReadDeadline(t time.Time) error {
	r.deadline = t
	r.pipeDeadline.set(t)
	return nil
}

func (r *Reader) ReaderReplaceable() bool {
	return r.deadline.IsZero()
}

func (r *Reader) UpstreamReader() any {
	return r.ExtendedReader
}
