package bufio

import (
	"net"
	"net/netip"

	"github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type NATPacketConn struct {
	N.NetPacketConn
	origin      M.Socksaddr
	destination M.Socksaddr
}

func NewNATPacketConn(conn N.NetPacketConn, origin M.Socksaddr, destination M.Socksaddr) *NATPacketConn {
	return &NATPacketConn{
		NetPacketConn: conn,
		origin:        origin,
		destination:   destination,
	}
}

func (c *NATPacketConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	n, addr, err = c.NetPacketConn.ReadFrom(p)
	if err == nil && M.SocksaddrFromNet(addr) == c.origin {
		addr = c.destination.UDPAddr()
	}
	return
}

func (c *NATPacketConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	if M.SocksaddrFromNet(addr) == c.destination {
		addr = c.origin.UDPAddr()
	}
	return c.NetPacketConn.WriteTo(p, addr)
}

func (c *NATPacketConn) ReadPacket(buffer *buf.Buffer) (destination M.Socksaddr, err error) {
	destination, err = c.NetPacketConn.ReadPacket(buffer)
	if destination == c.origin {
		destination = c.destination
	}
	return
}

func (c *NATPacketConn) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	if destination == c.destination {
		destination = c.origin
	}
	return c.NetPacketConn.WritePacket(buffer, destination)
}

func (c *NATPacketConn) UpdateDestination(destinationAddress netip.Addr) {
	c.destination = M.SocksaddrFrom(destinationAddress, c.destination.Port)
}

func (c *NATPacketConn) Upstream() any {
	return c.NetPacketConn
}
