package bufio

import (
	"io"
	"net"
	"sync"
	"syscall"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

func NewVectorisedWriter(writer io.Writer) N.VectorisedWriter {
	if vectorisedWriter, ok := CreateVectorisedWriter(writer); ok {
		return vectorisedWriter
	}
	return &SerialVectorisedWriter{upstream: writer}
}

func CreateVectorisedWriter(writer any) (N.VectorisedWriter, bool) {
	switch w := writer.(type) {
	case N.VectorisedWriter:
		return w, true
	case *net.TCPConn:
		return &NetVectorisedWriterWrapper{w}, true
	case *net.UDPConn:
		return &NetVectorisedWriterWrapper{w}, true
	case *net.IPConn:
		return &NetVectorisedWriterWrapper{w}, true
	case *net.UnixConn:
		return &NetVectorisedWriterWrapper{w}, true
	case syscall.Conn:
		rawConn, err := w.SyscallConn()
		if err == nil {
			return &SyscallVectorisedWriter{writer, rawConn}, true
		}
	case syscall.RawConn:
		return &SyscallVectorisedWriter{writer, w}, true
	}
	return nil, false
}

func CreateVectorisedPacketWriter(writer any) (N.VectorisedPacketWriter, bool) {
	switch w := writer.(type) {
	case N.VectorisedPacketWriter:
		return w, true
	case syscall.Conn:
		rawConn, err := w.SyscallConn()
		if err == nil {
			return &SyscallVectorisedPacketWriter{writer, rawConn}, true
		}
	case syscall.RawConn:
		return &SyscallVectorisedPacketWriter{writer, w}, true
	}
	return nil, false
}

var _ N.VectorisedWriter = (*SerialVectorisedWriter)(nil)

type SerialVectorisedWriter struct {
	upstream io.Writer
	access   sync.Mutex
}

func (w *SerialVectorisedWriter) WriteVectorised(buffers []*buf.Buffer) error {
	w.access.Lock()
	defer w.access.Unlock()
	for _, buffer := range buffers {
		_, err := w.upstream.Write(buffer.Bytes())
		if err != nil {
			return err
		}
	}
	return nil
}

func (w *SerialVectorisedWriter) Upstream() any {
	return w.upstream
}

var _ N.VectorisedWriter = (*NetVectorisedWriterWrapper)(nil)

type NetVectorisedWriterWrapper struct {
	upstream io.Writer
}

func (w *NetVectorisedWriterWrapper) WriteVectorised(buffers []*buf.Buffer) error {
	defer buf.ReleaseMulti(buffers)
	netBuffers := net.Buffers(buf.ToSliceMulti(buffers))
	return common.Error(netBuffers.WriteTo(w.upstream))
}

func (w *NetVectorisedWriterWrapper) Upstream() any {
	return w.upstream
}

func (w *NetVectorisedWriterWrapper) WriterReplaceable() bool {
	return true
}

var _ N.VectorisedWriter = (*SyscallVectorisedWriter)(nil)

type SyscallVectorisedWriter struct {
	upstream any
	rawConn  syscall.RawConn
}

func (w *SyscallVectorisedWriter) Upstream() any {
	return w.upstream
}

func (w *SyscallVectorisedWriter) WriterReplaceable() bool {
	return true
}

var _ N.VectorisedPacketWriter = (*SyscallVectorisedPacketWriter)(nil)

type SyscallVectorisedPacketWriter struct {
	upstream any
	rawConn  syscall.RawConn
}

func (w *SyscallVectorisedPacketWriter) Upstream() any {
	return w.upstream
}

var _ N.VectorisedPacketWriter = (*UnbindVectorisedPacketWriter)(nil)

type UnbindVectorisedPacketWriter struct {
	N.VectorisedWriter
}

func (w *UnbindVectorisedPacketWriter) WriteVectorisedPacket(buffers []*buf.Buffer, _ M.Socksaddr) error {
	return w.WriteVectorised(buffers)
}
