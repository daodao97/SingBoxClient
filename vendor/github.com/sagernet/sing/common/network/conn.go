package network

import (
	"context"
	"io"
	"net"
	"time"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"
)

type PacketReader interface {
	ReadPacket(buffer *buf.Buffer) (destination M.Socksaddr, err error)
}

type TimeoutPacketReader interface {
	PacketReader
	SetReadDeadline(t time.Time) error
}

type NetPacketReader interface {
	ReadFrom(p []byte) (n int, addr net.Addr, err error)
}

type NetPacketWriter interface {
	WriteTo(p []byte, addr net.Addr) (n int, err error)
}

type PacketWriter interface {
	WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error
}

type PacketConn interface {
	PacketReader
	PacketWriter

	Close() error
	LocalAddr() net.Addr
	SetDeadline(t time.Time) error
	SetReadDeadline(t time.Time) error
	SetWriteDeadline(t time.Time) error
}

type ExtendedReader interface {
	io.Reader
	ReadBuffer(buffer *buf.Buffer) error
}

type ExtendedWriter interface {
	io.Writer
	WriteBuffer(buffer *buf.Buffer) error
}

type ExtendedConn interface {
	ExtendedReader
	ExtendedWriter
	net.Conn
}

type TCPConnectionHandler interface {
	NewConnection(ctx context.Context, conn net.Conn, metadata M.Metadata) error
}

type NetPacketConn interface {
	PacketConn
	NetPacketReader
	NetPacketWriter
}

type BindPacketConn interface {
	NetPacketConn
	net.Conn
}

type UDPHandler interface {
	NewPacket(ctx context.Context, conn PacketConn, buffer *buf.Buffer, metadata M.Metadata) error
}

type UDPConnectionHandler interface {
	NewPacketConnection(ctx context.Context, conn PacketConn, metadata M.Metadata) error
}

type CachedReader interface {
	ReadCached() *buf.Buffer
}

type CachedPacketReader interface {
	ReadCachedPacket() (destination M.Socksaddr, buffer *buf.Buffer)
}

type WithUpstreamReader interface {
	UpstreamReader() any
}

type WithUpstreamWriter interface {
	UpstreamWriter() any
}

type ReaderWithUpstream interface {
	ReaderReplaceable() bool
}

type WriterWithUpstream interface {
	WriterReplaceable() bool
}

func UnwrapReader(reader io.Reader) io.Reader {
	if u, ok := reader.(ReaderWithUpstream); !ok || !u.ReaderReplaceable() {
		return reader
	}
	if u, ok := reader.(WithUpstreamReader); ok {
		return UnwrapReader(u.UpstreamReader().(io.Reader))
	}
	if u, ok := reader.(common.WithUpstream); ok {
		return UnwrapReader(u.Upstream().(io.Reader))
	}
	panic("bad reader")
}

func UnwrapPacketReader(reader PacketReader) PacketReader {
	if u, ok := reader.(ReaderWithUpstream); !ok || !u.ReaderReplaceable() {
		return reader
	}
	if u, ok := reader.(WithUpstreamReader); ok {
		return UnwrapPacketReader(u.UpstreamReader().(PacketReader))
	}
	if u, ok := reader.(common.WithUpstream); ok {
		return UnwrapPacketReader(u.Upstream().(PacketReader))
	}
	panic("bad reader")
}

func UnwrapWriter(writer io.Writer) io.Writer {
	if u, ok := writer.(WriterWithUpstream); !ok || !u.WriterReplaceable() {
		return writer
	}
	if u, ok := writer.(WithUpstreamWriter); ok {
		return UnwrapWriter(u.UpstreamWriter().(io.Writer))
	}
	if u, ok := writer.(common.WithUpstream); ok {
		return UnwrapWriter(u.Upstream().(io.Writer))
	}
	panic("bad writer")
}

func UnwrapPacketWriter(writer PacketWriter) PacketWriter {
	if u, ok := writer.(WriterWithUpstream); !ok || !u.WriterReplaceable() {
		return writer
	}
	if u, ok := writer.(WithUpstreamWriter); ok {
		return UnwrapPacketWriter(u.UpstreamWriter().(PacketWriter))
	}
	if u, ok := writer.(common.WithUpstream); ok {
		return UnwrapPacketWriter(u.Upstream().(PacketWriter))
	}
	panic("bad writer")
}
