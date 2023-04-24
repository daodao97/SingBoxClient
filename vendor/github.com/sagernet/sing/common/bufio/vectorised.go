package bufio

import (
	"io"
	"net"
	"syscall"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

func NewVectorisedWriter(writer io.Writer) N.VectorisedWriter {
	if vectorisedWriter, ok := CreateVectorisedWriter(N.UnwrapWriter(writer)); ok {
		return vectorisedWriter
	}
	return &BufferedVectorisedWriter{upstream: writer}
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

var _ N.VectorisedWriter = (*BufferedVectorisedWriter)(nil)

type BufferedVectorisedWriter struct {
	upstream io.Writer
}

func (w *BufferedVectorisedWriter) WriteVectorised(buffers []*buf.Buffer) error {
	defer buf.ReleaseMulti(buffers)
	bufferLen := buf.LenMulti(buffers)
	if bufferLen == 0 {
		return common.Error(w.upstream.Write(nil))
	} else if len(buffers) == 1 {
		return common.Error(w.upstream.Write(buffers[0].Bytes()))
	}
	var bufferBytes []byte
	if bufferLen > 65535 {
		bufferBytes = make([]byte, bufferLen)
	} else {
		_buffer := buf.StackNewSize(bufferLen)
		defer common.KeepAlive(_buffer)
		buffer := common.Dup(_buffer)
		defer buffer.Release()
		bufferBytes = buffer.FreeBytes()
	}
	buf.CopyMulti(bufferBytes, buffers)
	return common.Error(w.upstream.Write(bufferBytes))
}

func (w *BufferedVectorisedWriter) Upstream() any {
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
