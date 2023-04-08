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
