package socks

import (
	"bytes"
	"net"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

// +----+------+------+----------+----------+----------+
// |RSV | FRAG | ATYP | DST.ADDR | DST.PORT |   DATA   |
// +----+------+------+----------+----------+----------+
// | 2  |  1   |  1   | Variable |    2     | Variable |
// +----+------+------+----------+----------+----------+

type AssociatePacketConn struct {
	N.NetPacketConn
	remoteAddr M.Socksaddr
	underlying net.Conn
}

func NewAssociatePacketConn(conn net.PacketConn, remoteAddr M.Socksaddr, underlying net.Conn) *AssociatePacketConn {
	return &AssociatePacketConn{
		NetPacketConn: bufio.NewPacketConn(conn),
		remoteAddr:    remoteAddr,
		underlying:    underlying,
	}
}

func NewAssociateConn(conn net.Conn, remoteAddr M.Socksaddr, underlying net.Conn) *AssociatePacketConn {
	return &AssociatePacketConn{
		NetPacketConn: bufio.NewUnbindPacketConn(conn),
		remoteAddr:    remoteAddr,
		underlying:    underlying,
	}
}

func (c *AssociatePacketConn) RemoteAddr() net.Addr {
	return c.remoteAddr.UDPAddr()
}

//warn:unsafe
func (c *AssociatePacketConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	n, addr, err = c.NetPacketConn.ReadFrom(p)
	if err != nil {
		return
	}
	c.remoteAddr = M.SocksaddrFromNet(addr)
	reader := bytes.NewReader(p[3:n])
	destination, err := M.SocksaddrSerializer.ReadAddrPort(reader)
	if err != nil {
		return
	}
	addr = destination.UDPAddr()
	index := 3 + int(reader.Size()) - reader.Len()
	n = copy(p, p[index:n])
	return
}

//warn:unsafe
func (c *AssociatePacketConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	destination := M.SocksaddrFromNet(addr)
	_buffer := buf.StackNewSize(3 + M.SocksaddrSerializer.AddrPortLen(destination) + len(p))
	defer common.KeepAlive(_buffer)
	buffer := common.Dup(_buffer)
	defer buffer.Release()
	common.Must(buffer.WriteZeroN(3))
	err = M.SocksaddrSerializer.WriteAddrPort(buffer, destination)
	if err != nil {
		return
	}
	_, err = buffer.Write(p)
	if err != nil {
		return
	}
	return bufio.WritePacketBuffer(c.NetPacketConn, buffer, c.remoteAddr)
}

func (c *AssociatePacketConn) Read(b []byte) (n int, err error) {
	n, _, err = c.ReadFrom(b)
	return
}

func (c *AssociatePacketConn) Write(b []byte) (n int, err error) {
	return c.WriteTo(b, c.remoteAddr)
}

func (c *AssociatePacketConn) ReadPacket(buffer *buf.Buffer) (M.Socksaddr, error) {
	destination, err := c.NetPacketConn.ReadPacket(buffer)
	if err != nil {
		return M.Socksaddr{}, err
	}
	c.remoteAddr = destination
	buffer.Advance(3)
	dest, err := M.SocksaddrSerializer.ReadAddrPort(buffer)
	return dest, err
}

func (c *AssociatePacketConn) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	header := buf.With(buffer.ExtendHeader(3 + M.SocksaddrSerializer.AddrPortLen(destination)))
	common.Must(header.WriteZeroN(3))
	common.Must(M.SocksaddrSerializer.WriteAddrPort(header, destination))
	return common.Error(bufio.WritePacketBuffer(c.NetPacketConn, buffer, c.remoteAddr))
}

func (c *AssociatePacketConn) Upstream() any {
	return c.NetPacketConn
}

func (c *AssociatePacketConn) FrontHeadroom() int {
	return 3 + M.MaxSocksaddrLength
}

func (c *AssociatePacketConn) Close() error {
	return common.Close(
		c.NetPacketConn,
		c.underlying,
	)
}
