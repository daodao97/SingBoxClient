package bufio

import (
	"net"

	"github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

var _ N.NetPacketConn = (*FallbackPacketConn)(nil)

type FallbackPacketConn struct {
	N.PacketConn
}

func NewNetPacketConn(conn N.PacketConn) N.NetPacketConn {
	if packetConn, loaded := conn.(N.NetPacketConn); loaded {
		return packetConn
	}
	return &FallbackPacketConn{PacketConn: conn}
}

func (c *FallbackPacketConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	buffer := buf.With(p)
	destination, err := c.ReadPacket(buffer)
	if err != nil {
		return
	}
	n = buffer.Len()
	if buffer.Start() > 0 {
		copy(p, buffer.Bytes())
	}
	addr = destination.UDPAddr()
	return
}

func (c *FallbackPacketConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	err = c.WritePacket(buf.As(p), M.SocksaddrFromNet(addr))
	if err == nil {
		n = len(p)
	}
	return
}

func (c *FallbackPacketConn) ReaderReplaceable() bool {
	return true
}

func (c *FallbackPacketConn) WriterReplaceable() bool {
	return true
}

func (c *FallbackPacketConn) Upstream() any {
	return c.PacketConn
}
