package mux

import (
	"encoding/binary"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/rw"
)

var _ N.HandshakeConn = (*serverConn)(nil)

type serverConn struct {
	N.ExtendedConn
	responseWritten bool
}

func (c *serverConn) HandshakeFailure(err error) error {
	errMessage := err.Error()
	_buffer := buf.StackNewSize(1 + rw.UVariantLen(uint64(len(errMessage))) + len(errMessage))
	defer common.KeepAlive(_buffer)
	buffer := common.Dup(_buffer)
	defer buffer.Release()
	common.Must(
		buffer.WriteByte(statusError),
		rw.WriteVString(_buffer, errMessage),
	)
	return c.ExtendedConn.WriteBuffer(buffer)
}

func (c *serverConn) Write(b []byte) (n int, err error) {
	if c.responseWritten {
		return c.ExtendedConn.Write(b)
	}
	_buffer := buf.StackNewSize(1 + len(b))
	defer common.KeepAlive(_buffer)
	buffer := common.Dup(_buffer)
	defer buffer.Release()
	common.Must(
		buffer.WriteByte(statusSuccess),
		common.Error(buffer.Write(b)),
	)
	_, err = c.ExtendedConn.Write(buffer.Bytes())
	if err != nil {
		return
	}
	c.responseWritten = true
	return len(b), nil
}

func (c *serverConn) WriteBuffer(buffer *buf.Buffer) error {
	if c.responseWritten {
		return c.ExtendedConn.WriteBuffer(buffer)
	}
	buffer.ExtendHeader(1)[0] = statusSuccess
	c.responseWritten = true
	return c.ExtendedConn.WriteBuffer(buffer)
}

func (c *serverConn) FrontHeadroom() int {
	if !c.responseWritten {
		return 1
	}
	return 0
}

func (c *serverConn) NeedAdditionalReadDeadline() bool {
	return true
}

func (c *serverConn) Upstream() any {
	return c.ExtendedConn
}

var (
	_ N.HandshakeConn = (*serverPacketConn)(nil)
	_ N.PacketConn    = (*serverPacketConn)(nil)
)

type serverPacketConn struct {
	N.ExtendedConn
	destination     M.Socksaddr
	responseWritten bool
}

func (c *serverPacketConn) HandshakeFailure(err error) error {
	errMessage := err.Error()
	_buffer := buf.StackNewSize(1 + rw.UVariantLen(uint64(len(errMessage))) + len(errMessage))
	defer common.KeepAlive(_buffer)
	buffer := common.Dup(_buffer)
	defer buffer.Release()
	common.Must(
		buffer.WriteByte(statusError),
		rw.WriteVString(_buffer, errMessage),
	)
	return c.ExtendedConn.WriteBuffer(buffer)
}

func (c *serverPacketConn) ReadPacket(buffer *buf.Buffer) (destination M.Socksaddr, err error) {
	var length uint16
	err = binary.Read(c.ExtendedConn, binary.BigEndian, &length)
	if err != nil {
		return
	}
	_, err = buffer.ReadFullFrom(c.ExtendedConn, int(length))
	if err != nil {
		return
	}
	destination = c.destination
	return
}

func (c *serverPacketConn) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	pLen := buffer.Len()
	common.Must(binary.Write(buf.With(buffer.ExtendHeader(2)), binary.BigEndian, uint16(pLen)))
	if !c.responseWritten {
		buffer.ExtendHeader(1)[0] = statusSuccess
		c.responseWritten = true
	}
	return c.ExtendedConn.WriteBuffer(buffer)
}

func (c *serverPacketConn) NeedAdditionalReadDeadline() bool {
	return true
}

func (c *serverPacketConn) Upstream() any {
	return c.ExtendedConn
}

func (c *serverPacketConn) FrontHeadroom() int {
	if !c.responseWritten {
		return 3
	}
	return 2
}

var (
	_ N.HandshakeConn = (*serverPacketAddrConn)(nil)
	_ N.PacketConn    = (*serverPacketAddrConn)(nil)
)

type serverPacketAddrConn struct {
	N.ExtendedConn
	responseWritten bool
}

func (c *serverPacketAddrConn) HandshakeFailure(err error) error {
	errMessage := err.Error()
	_buffer := buf.StackNewSize(1 + rw.UVariantLen(uint64(len(errMessage))) + len(errMessage))
	defer common.KeepAlive(_buffer)
	buffer := common.Dup(_buffer)
	defer buffer.Release()
	common.Must(
		buffer.WriteByte(statusError),
		rw.WriteVString(_buffer, errMessage),
	)
	return c.ExtendedConn.WriteBuffer(buffer)
}

func (c *serverPacketAddrConn) ReadPacket(buffer *buf.Buffer) (destination M.Socksaddr, err error) {
	destination, err = M.SocksaddrSerializer.ReadAddrPort(c.ExtendedConn)
	if err != nil {
		return
	}
	var length uint16
	err = binary.Read(c.ExtendedConn, binary.BigEndian, &length)
	if err != nil {
		return
	}
	_, err = buffer.ReadFullFrom(c.ExtendedConn, int(length))
	if err != nil {
		return
	}
	return
}

func (c *serverPacketAddrConn) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	pLen := buffer.Len()
	common.Must(binary.Write(buf.With(buffer.ExtendHeader(2)), binary.BigEndian, uint16(pLen)))
	common.Must(M.SocksaddrSerializer.WriteAddrPort(buf.With(buffer.ExtendHeader(M.SocksaddrSerializer.AddrPortLen(destination))), destination))
	if !c.responseWritten {
		buffer.ExtendHeader(1)[0] = statusSuccess
		c.responseWritten = true
	}
	return c.ExtendedConn.WriteBuffer(buffer)
}

func (c *serverPacketAddrConn) NeedAdditionalReadDeadline() bool {
	return true
}

func (c *serverPacketAddrConn) Upstream() any {
	return c.ExtendedConn
}

func (c *serverPacketAddrConn) FrontHeadroom() int {
	if !c.responseWritten {
		return 3 + M.MaxSocksaddrLength
	}
	return 2 + M.MaxSocksaddrLength
}
