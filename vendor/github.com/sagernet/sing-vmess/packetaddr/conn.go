package packetaddr

import (
	"net"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type PacketConn struct {
	N.NetPacketConn
	bindAddr M.Socksaddr
}

func NewConn(conn net.PacketConn, bindAddr M.Socksaddr) *PacketConn {
	return &PacketConn{
		bufio.NewPacketConn(conn),
		bindAddr,
	}
}

func NewBindConn(conn net.Conn) *PacketConn {
	return &PacketConn{
		bufio.NewUnbindPacketConn(conn),
		M.Socksaddr{},
	}
}

func (c *PacketConn) RemoteAddr() net.Addr {
	return c.bindAddr
}

func (c *PacketConn) Read(b []byte) (n int, err error) {
	n, _, err = c.ReadFrom(b)
	return
}

func (c *PacketConn) Write(b []byte) (n int, err error) {
	return c.WriteTo(b, c.RemoteAddr())
}

func (c *PacketConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	buffer := buf.With(p)
	var destination M.Socksaddr
	destination, err = c.ReadPacket(buffer)
	if err != nil {
		return
	}
	n = copy(p, buffer.Bytes())
	if destination.IsFqdn() {
		addr = destination
	} else {
		addr = destination.UDPAddr()
	}
	return
}

func (c *PacketConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	destination := M.SocksaddrFromNet(addr)
	_buffer := buf.StackNewSize(AddressSerializer.AddrPortLen(destination) + len(p))
	defer common.KeepAlive(_buffer)
	buffer := common.Dup(_buffer)
	defer buffer.Release()
	common.Must(AddressSerializer.WriteAddrPort(buffer, destination))
	common.Must1(buffer.Write(p))
	return c.NetPacketConn.WriteTo(buffer.Bytes(), c.bindAddr.UDPAddr())
}

func (c *PacketConn) ReadPacket(buffer *buf.Buffer) (destination M.Socksaddr, err error) {
	_, err = c.NetPacketConn.ReadPacket(buffer)
	if err != nil {
		return
	}
	return AddressSerializer.ReadAddrPort(buffer)
}

func (c *PacketConn) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	if destination.IsFqdn() {
		return E.Extend(ErrFqdnUnsupported, destination.Fqdn)
	}
	header := buf.With(buffer.ExtendHeader(AddressSerializer.AddrPortLen(destination)))
	common.Must(AddressSerializer.WriteAddrPort(header, destination))
	return c.NetPacketConn.WritePacket(buffer, c.bindAddr)
}

func (c *PacketConn) FrontHeadroom() int {
	return M.MaxIPSocksaddrLength
}

func (c *PacketConn) Upstream() any {
	return c.NetPacketConn
}
