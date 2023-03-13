package bufio

import (
	"io"
	"os"
	"sync"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	N "github.com/sagernet/sing/common/network"
)

type BufferedReader struct {
	upstream N.ExtendedReader
	buffer   *buf.Buffer
}

func NewBufferedReader(upstream io.Reader, buffer *buf.Buffer) *BufferedReader {
	return &BufferedReader{
		upstream: NewExtendedReader(upstream),
		buffer:   buffer,
	}
}

func (r *BufferedReader) Read(p []byte) (n int, err error) {
	if r.buffer.Closed() {
		return 0, os.ErrClosed
	}
	if r.buffer.IsEmpty() {
		r.buffer.Reset()
		err = r.upstream.ReadBuffer(r.buffer)
		if err != nil {
			r.buffer.Release()
			return
		}
	}
	return r.buffer.Read(p)
}

func (r *BufferedReader) ReadBuffer(buffer *buf.Buffer) error {
	if r.buffer.Closed() {
		return os.ErrClosed
	}
	var err error
	if r.buffer.IsEmpty() {
		r.buffer.Reset()
		err = r.upstream.ReadBuffer(r.buffer)
		if err != nil {
			r.buffer.Release()
			return err
		}
	}
	if r.buffer.Len() > buffer.FreeLen() {
		err = common.Error(buffer.ReadFullFrom(r.buffer, buffer.FreeLen()))
	} else {
		err = common.Error(buffer.ReadFullFrom(r.buffer, r.buffer.Len()))
	}
	if err != nil {
		r.buffer.Release()
	}
	return err
}

func (r *BufferedReader) WriteTo(w io.Writer) (n int64, err error) {
	if r.buffer.Closed() {
		return 0, os.ErrClosed
	}
	defer r.buffer.Release()
	return CopyExtendedBuffer(NewExtendedWriter(w), NewExtendedReader(r.upstream), r.buffer)
}

func (r *BufferedReader) Upstream() any {
	return r.upstream
}

type BufferedWriter struct {
	upstream io.Writer
	buffer   *buf.Buffer
	access   sync.Mutex
}

func NewBufferedWriter(upstream io.Writer, buffer *buf.Buffer) *BufferedWriter {
	return &BufferedWriter{
		upstream: upstream,
		buffer:   buffer,
	}
}

func (w *BufferedWriter) Write(p []byte) (n int, err error) {
	w.access.Lock()
	defer w.access.Unlock()
	if w.buffer == nil {
		return w.upstream.Write(p)
	}
	for {
		var writeN int
		writeN, err = w.buffer.Write(p[n:])
		n += writeN
		if n == len(p) {
			return
		}
		_, err = w.upstream.Write(w.buffer.Bytes())
		if err != nil {
			return
		}
		w.buffer.FullReset()
	}
}

func (w *BufferedWriter) Fallthrough() error {
	w.access.Lock()
	defer w.access.Unlock()
	if w.buffer == nil {
		return nil
	}
	if !w.buffer.IsEmpty() {
		_, err := w.upstream.Write(w.buffer.Bytes())
		if err != nil {
			return err
		}
	}
	w.buffer.Release()
	w.buffer = nil
	return nil
}

func (w *BufferedWriter) ReadFrom(r io.Reader) (n int64, err error) {
	if w.buffer == nil {
		return Copy(w.upstream, r)
	}
	return CopyExtendedBuffer(NewExtendedWriter(w), NewExtendedReader(r), w.buffer)
}

func (w *BufferedWriter) WriterReplaceable() bool {
	return w.buffer == nil
}
