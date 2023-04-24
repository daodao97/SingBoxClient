package deadline

import (
	"time"

	"github.com/sagernet/sing/common/atomic"
	"github.com/sagernet/sing/common/buf"
)

type fallbackReader struct {
	*reader
	disablePipe atomic.Bool
	inRead      atomic.Bool
}

func NewFallbackReader(timeoutReader TimeoutReader) Reader {
	return &fallbackReader{reader: NewReader(timeoutReader).(*reader)}
}

func (r *fallbackReader) Read(p []byte) (n int, err error) {
	select {
	case result := <-r.result:
		return r.pipeReturn(result, p)
	default:
	}
	if r.disablePipe.Load() {
		return r.ExtendedReader.Read(p)
	}
	select {
	case <-r.done:
		if r.deadline.Load().IsZero() {
			r.done <- struct{}{}
			r.inRead.Store(true)
			defer r.inRead.Store(false)
			n, err = r.ExtendedReader.Read(p)
			return
		}
		go r.pipeRead(len(p))
	default:
	}
	return r.reader.read(p)
}

func (r *fallbackReader) ReadBuffer(buffer *buf.Buffer) error {
	select {
	case result := <-r.result:
		return r.pipeReturnBuffer(result, buffer)
	default:
	}
	if r.disablePipe.Load() {
		return r.ExtendedReader.ReadBuffer(buffer)
	}
	select {
	case <-r.done:
		if r.deadline.Load().IsZero() {
			r.done <- struct{}{}
			r.inRead.Store(true)
			defer r.inRead.Store(false)
			return r.ExtendedReader.ReadBuffer(buffer)
		}
		go r.pipeReadBuffer(buffer.Cap(), buffer.Start())
	default:
	}
	return r.readBuffer(buffer)
}

func (r *fallbackReader) SetReadDeadline(t time.Time) error {
	if r.disablePipe.Load() {
		return r.timeoutReader.SetReadDeadline(t)
	} else if r.inRead.Load() {
		r.disablePipe.Store(true)
		return r.timeoutReader.SetReadDeadline(t)
	}
	return r.reader.SetReadDeadline(t)
}

func (r *fallbackReader) ReaderReplaceable() bool {
	return r.disablePipe.Load() || r.reader.ReaderReplaceable()
}

func (r *fallbackReader) UpstreamReader() any {
	return r.reader.UpstreamReader()
}
