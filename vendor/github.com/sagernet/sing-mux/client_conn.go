package mux

import (
	"encoding/binary"
	"io"
	"net"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type clientConn struct {
	net.Conn
	destination    M.Socksaddr
	requestWritten bool
	responseRead   bool
}

func (c *clientConn) readResponse() error {
	response, err := ReadStreamResponse(c.Conn)
	if err != nil {
		return err
	}
	if response.Status == statusError {
		return E.New("remote error: ", response.Message)
	}
	return nil
}

func (c *clientConn) Read(b []byte) (n int, err error) {
	if !c.responseRead {
		err = c.readResponse()
		if err != nil {
			return
		}
		c.responseRead = true
	}
	return c.Conn.Read(b)
}

func (c *clientConn) Write(b []byte) (n int, err error) {
	if c.requestWritten {
		return c.Conn.Write(b)
	}
	request := StreamRequest{
		Network:     N.NetworkTCP,
		Destination: c.destination,
	}
	_buffer := buf.StackNewSize(streamRequestLen(request) + len(b))
	defer common.KeepAlive(_buffer)
	buffer := common.Dup(_buffer)
	defer buffer.Release()
	EncodeStreamRequest(request, buffer)
	buffer.Write(b)
	_, err = c.Conn.Write(buffer.Bytes())
	if err != nil {
		return
	}
	c.requestWritten = true
	return len(b), nil
}

func (c *clientConn) ReadFrom(r io.Reader) (n int64, err error) {
	if !c.requestWritten {
		return bufio.ReadFrom0(c, r)
	}
	return bufio.Copy(c.Conn, r)
}

func (c *clientConn) WriteTo(w io.Writer) (n int64, err error) {
	if !c.responseRead {
		return bufio.WriteTo0(c, w)
	}
	return bufio.Copy(w, c.Conn)
}

func (c *clientConn) LocalAddr() net.Addr {
	return c.Conn.LocalAddr()
}

func (c *clientConn) RemoteAddr() net.Addr {
	return c.destination.TCPAddr()
}

func (c *clientConn) ReaderReplaceable() bool {
	return c.responseRead
}

func (c *clientConn) WriterReplaceable() bool {
	return c.requestWritten
}

func (c *clientConn) NeedAdditionalReadDeadline() bool {
	return true
}

func (c *clientConn) Upstream() any {
	return c.Conn
}

type clientPacketConn struct {
	N.ExtendedConn
	destination    M.Socksaddr
	requestWritten bool
	responseRead   bool
}

func (c *clientPacketConn) readResponse() error {
	response, err := ReadStreamResponse(c.ExtendedConn)
	if err != nil {
		return err
	}
	if response.Status == statusError {
		return E.New("remote error: ", response.Message)
	}
	return nil
}

func (c *clientPacketConn) Read(b []byte) (n int, err error) {
	if !c.responseRead {
		err = c.readResponse()
		if err != nil {
			return
		}
		c.responseRead = true
	}
	var length uint16
	err = binary.Read(c.ExtendedConn, binary.BigEndian, &length)
	if err != nil {
		return
	}
	if cap(b) < int(length) {
		return 0, io.ErrShortBuffer
	}
	return io.ReadFull(c.ExtendedConn, b[:length])
}

func (c *clientPacketConn) writeRequest(payload []byte) (n int, err error) {
	request := StreamRequest{
		Network:     N.NetworkUDP,
		Destination: c.destination,
	}
	rLen := streamRequestLen(request)
	if len(payload) > 0 {
		rLen += 2 + len(payload)
	}
	_buffer := buf.StackNewSize(rLen)
	defer common.KeepAlive(_buffer)
	buffer := common.Dup(_buffer)
	defer buffer.Release()
	EncodeStreamRequest(request, buffer)
	if len(payload) > 0 {
		common.Must(
			binary.Write(buffer, binary.BigEndian, uint16(len(payload))),
			common.Error(buffer.Write(payload)),
		)
	}
	_, err = c.ExtendedConn.Write(buffer.Bytes())
	if err != nil {
		return
	}
	c.requestWritten = true
	return len(payload), nil
}

func (c *clientPacketConn) Write(b []byte) (n int, err error) {
	if !c.requestWritten {
		return c.writeRequest(b)
	}
	err = binary.Write(c.ExtendedConn, binary.BigEndian, uint16(len(b)))
	if err != nil {
		return
	}
	return c.ExtendedConn.Write(b)
}

func (c *clientPacketConn) ReadBuffer(buffer *buf.Buffer) (err error) {
	if !c.responseRead {
		err = c.readResponse()
		if err != nil {
			return
		}
		c.responseRead = true
	}
	var length uint16
	err = binary.Read(c.ExtendedConn, binary.BigEndian, &length)
	if err != nil {
		return
	}
	_, err = buffer.ReadFullFrom(c.ExtendedConn, int(length))
	return
}

func (c *clientPacketConn) WriteBuffer(buffer *buf.Buffer) error {
	if !c.requestWritten {
		defer buffer.Release()
		return common.Error(c.writeRequest(buffer.Bytes()))
	}
	bLen := buffer.Len()
	binary.BigEndian.PutUint16(buffer.ExtendHeader(2), uint16(bLen))
	return c.ExtendedConn.WriteBuffer(buffer)
}

func (c *clientPacketConn) FrontHeadroom() int {
	return 2
}

func (c *clientPacketConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	if !c.responseRead {
		err = c.readResponse()
		if err != nil {
			return
		}
		c.responseRead = true
	}
	var length uint16
	err = binary.Read(c.ExtendedConn, binary.BigEndian, &length)
	if err != nil {
		return
	}
	if cap(p) < int(length) {
		return 0, nil, io.ErrShortBuffer
	}
	n, err = io.ReadFull(c.ExtendedConn, p[:length])
	return
}

func (c *clientPacketConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	if !c.requestWritten {
		return c.writeRequest(p)
	}
	err = binary.Write(c.ExtendedConn, binary.BigEndian, uint16(len(p)))
	if err != nil {
		return
	}
	return c.ExtendedConn.Write(p)
}

func (c *clientPacketConn) ReadPacket(buffer *buf.Buffer) (destination M.Socksaddr, err error) {
	err = c.ReadBuffer(buffer)
	return
}

func (c *clientPacketConn) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	return c.WriteBuffer(buffer)
}

func (c *clientPacketConn) LocalAddr() net.Addr {
	return c.ExtendedConn.LocalAddr()
}

func (c *clientPacketConn) RemoteAddr() net.Addr {
	return c.destination.UDPAddr()
}

func (c *clientPacketConn) NeedAdditionalReadDeadline() bool {
	return true
}

func (c *clientPacketConn) Upstream() any {
	return c.ExtendedConn
}

var _ N.NetPacketConn = (*clientPacketAddrConn)(nil)

type clientPacketAddrConn struct {
	N.ExtendedConn
	destination    M.Socksaddr
	requestWritten bool
	responseRead   bool
}

func (c *clientPacketAddrConn) readResponse() error {
	response, err := ReadStreamResponse(c.ExtendedConn)
	if err != nil {
		return err
	}
	if response.Status == statusError {
		return E.New("remote error: ", response.Message)
	}
	return nil
}

func (c *clientPacketAddrConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	if !c.responseRead {
		err = c.readResponse()
		if err != nil {
			return
		}
		c.responseRead = true
	}
	destination, err := M.SocksaddrSerializer.ReadAddrPort(c.ExtendedConn)
	if err != nil {
		return
	}
	if destination.IsFqdn() {
		addr = destination
	} else {
		addr = destination.UDPAddr()
	}
	var length uint16
	err = binary.Read(c.ExtendedConn, binary.BigEndian, &length)
	if err != nil {
		return
	}
	if cap(p) < int(length) {
		return 0, nil, io.ErrShortBuffer
	}
	n, err = io.ReadFull(c.ExtendedConn, p[:length])
	return
}

func (c *clientPacketAddrConn) writeRequest(payload []byte, destination M.Socksaddr) (n int, err error) {
	request := StreamRequest{
		Network:     N.NetworkUDP,
		Destination: c.destination,
		PacketAddr:  true,
	}
	rLen := streamRequestLen(request)
	if len(payload) > 0 {
		rLen += M.SocksaddrSerializer.AddrPortLen(destination) + 2 + len(payload)
	}
	_buffer := buf.StackNewSize(rLen)
	defer common.KeepAlive(_buffer)
	buffer := common.Dup(_buffer)
	defer buffer.Release()
	EncodeStreamRequest(request, buffer)
	if len(payload) > 0 {
		common.Must(
			M.SocksaddrSerializer.WriteAddrPort(buffer, destination),
			binary.Write(buffer, binary.BigEndian, uint16(len(payload))),
			common.Error(buffer.Write(payload)),
		)
	}
	_, err = c.ExtendedConn.Write(buffer.Bytes())
	if err != nil {
		return
	}
	c.requestWritten = true
	return len(payload), nil
}

func (c *clientPacketAddrConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	if !c.requestWritten {
		return c.writeRequest(p, M.SocksaddrFromNet(addr))
	}
	err = M.SocksaddrSerializer.WriteAddrPort(c.ExtendedConn, M.SocksaddrFromNet(addr))
	if err != nil {
		return
	}
	err = binary.Write(c.ExtendedConn, binary.BigEndian, uint16(len(p)))
	if err != nil {
		return
	}
	return c.ExtendedConn.Write(p)
}

func (c *clientPacketAddrConn) ReadPacket(buffer *buf.Buffer) (destination M.Socksaddr, err error) {
	if !c.responseRead {
		err = c.readResponse()
		if err != nil {
			return
		}
		c.responseRead = true
	}
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
	return
}

func (c *clientPacketAddrConn) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	if !c.requestWritten {
		defer buffer.Release()
		return common.Error(c.writeRequest(buffer.Bytes(), destination))
	}
	bLen := buffer.Len()
	header := buf.With(buffer.ExtendHeader(M.SocksaddrSerializer.AddrPortLen(destination) + 2))
	common.Must(
		M.SocksaddrSerializer.WriteAddrPort(header, destination),
		binary.Write(header, binary.BigEndian, uint16(bLen)),
	)
	return c.ExtendedConn.WriteBuffer(buffer)
}

func (c *clientPacketAddrConn) LocalAddr() net.Addr {
	return c.ExtendedConn.LocalAddr()
}

func (c *clientPacketAddrConn) FrontHeadroom() int {
	return 2 + M.MaxSocksaddrLength
}

func (c *clientPacketAddrConn) NeedAdditionalReadDeadline() bool {
	return true
}

func (c *clientPacketAddrConn) Upstream() any {
	return c.ExtendedConn
}
