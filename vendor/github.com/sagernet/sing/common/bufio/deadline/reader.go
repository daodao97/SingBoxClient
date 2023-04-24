package deadline

import (
	"io"
	"os"
	"time"

	"github.com/sagernet/sing/common/atomic"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	N "github.com/sagernet/sing/common/network"
)

type TimeoutReader interface {
	io.Reader
	SetReadDeadline(t time.Time) error
}

type Reader interface {
	N.ExtendedReader
	TimeoutReader
	N.WithUpstreamReader
	N.ReaderWithUpstream
}

type reader struct {
	N.ExtendedReader
	timeoutReader TimeoutReader
	deadline      atomic.TypedValue[time.Time]
	pipeDeadline  pipeDeadline
	result        chan *readResult
	done          chan struct{}
}

type readResult struct {
	buffer *buf.Buffer
	err    error
}

func NewReader(timeoutReader TimeoutReader) Reader {
	return &reader{
		ExtendedReader: bufio.NewExtendedReader(timeoutReader),
		timeoutReader:  timeoutReader,
		pipeDeadline:   makePipeDeadline(),
		result:         make(chan *readResult, 1),
		done:           makeFilledChan(),
	}
}

func (r *reader) Read(p []byte) (n int, err error) {
	select {
	case result := <-r.result:
		return r.pipeReturn(result, p)
	default:
	}
	select {
	case <-r.done:
		go r.pipeRead(len(p))
	default:
	}
	return r.read(p)
}

func (r *reader) read(p []byte) (n int, err error) {
	select {
	case result := <-r.result:
		return r.pipeReturn(result, p)
	case <-r.pipeDeadline.wait():
		return 0, os.ErrDeadlineExceeded
	}
}

func (r *reader) pipeReturn(result *readResult, p []byte) (n int, err error) {
	n = copy(p, result.buffer.Bytes())
	result.buffer.Advance(n)
	if result.buffer.IsEmpty() {
		result.buffer.Release()
		err = result.err
	} else {
		r.result <- result
	}
	return
}

func (r *reader) pipeRead(pLen int) {
	buffer := buf.NewSize(pLen)
	_, err := buffer.ReadOnceFrom(r.ExtendedReader)
	r.result <- &readResult{
		buffer: buffer,
		err:    err,
	}
	r.done <- struct{}{}
}

func (r *reader) ReadBuffer(buffer *buf.Buffer) error {
	select {
	case result := <-r.result:
		return r.pipeReturnBuffer(result, buffer)
	default:
	}
	select {
	case <-r.done:
		go r.pipeReadBuffer(buffer.Cap(), buffer.Start())
	default:
	}
	return r.readBuffer(buffer)
}

func (r *reader) readBuffer(buffer *buf.Buffer) error {
	select {
	case result := <-r.result:
		return r.pipeReturnBuffer(result, buffer)
	case <-r.pipeDeadline.wait():
		return os.ErrDeadlineExceeded
	}
}

func (r *reader) pipeReturnBuffer(result *readResult, buffer *buf.Buffer) error {
	buffer.Resize(result.buffer.Start(), 0)
	n := copy(buffer.FreeBytes(), result.buffer.Bytes())
	buffer.Truncate(n)
	result.buffer.Advance(n)
	if !result.buffer.IsEmpty() {
		r.result <- result
		return nil
	} else {
		result.buffer.Release()
		return result.err
	}
}

func (r *reader) pipeReadBuffer(bufLen int, bufStart int) {
	cacheBuffer := buf.NewSize(bufLen)
	cacheBuffer.Advance(bufStart)
	err := r.ExtendedReader.ReadBuffer(cacheBuffer)
	r.result <- &readResult{
		buffer: cacheBuffer,
		err:    err,
	}
	r.done <- struct{}{}
}

func (r *reader) SetReadDeadline(t time.Time) error {
	r.deadline.Store(t)
	r.pipeDeadline.set(t)
	return nil
}

func (r *reader) ReaderReplaceable() bool {
	select {
	case <-r.done:
		r.done <- struct{}{}
	default:
		return false
	}
	select {
	case result := <-r.result:
		r.result <- result
		return false
	default:
	}
	return r.deadline.Load().IsZero()
}

func (r *reader) UpstreamReader() any {
	return r.ExtendedReader
}
