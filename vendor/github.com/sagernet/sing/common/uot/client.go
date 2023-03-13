package uot

import (
	"bufio"
	"encoding/binary"
	"io"
	"net"
	"sync"

	"github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"
)

type ClientConn struct {
	net.Conn
	reader      *bufio.Reader
	writer      *bufio.Writer
	readAccess  sync.Mutex
	writeAccess sync.Mutex
}

func NewClientConn(conn net.Conn) *ClientConn {
	return &ClientConn{
		Conn:   conn,
		reader: bufio.NewReader(conn),
		writer: bufio.NewWriter(conn),
	}
}

func (c *ClientConn) ReadPacket(buffer *buf.Buffer) (M.Socksaddr, error) {
	c.readAccess.Lock()
	defer c.readAccess.Unlock()

	destination, err := AddrParser.ReadAddrPort(c.reader)
	if err != nil {
		return M.Socksaddr{}, err
	}
	var length uint16
	err = binary.Read(c.reader, binary.BigEndian, &length)
	if err != nil {
		return M.Socksaddr{}, err
	}
	_, err = buffer.ReadFullFrom(c.reader, int(length))
	if err != nil {
		return M.Socksaddr{}, err
	}
	return destination, nil
}

func (c *ClientConn) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	c.writeAccess.Lock()
	defer c.writeAccess.Unlock()

	defer buffer.Release()
	err := AddrParser.WriteAddrPort(c.writer, destination)
	if err != nil {
		return err
	}
	err = binary.Write(c.writer, binary.BigEndian, uint16(buffer.Len()))
	if err != nil {
		return err
	}
	_, err = c.writer.Write(buffer.Bytes())
	if err != nil {
		return err
	}
	return c.writer.Flush()
}

func (c *ClientConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	c.readAccess.Lock()
	defer c.readAccess.Unlock()

	addrPort, err := AddrParser.ReadAddrPort(c.reader)
	if err != nil {
		return 0, nil, err
	}
	var length uint16
	err = binary.Read(c.reader, binary.BigEndian, &length)
	if err != nil {
		return 0, nil, err
	}
	if len(p) < int(length) {
		return 0, nil, io.ErrShortBuffer
	}
	n, err = io.ReadFull(c.reader, p[:length])
	if err != nil {
		return 0, nil, err
	}
	addr = addrPort.UDPAddr()
	return
}

func (c *ClientConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	c.writeAccess.Lock()
	defer c.writeAccess.Unlock()

	err = AddrParser.WriteAddrPort(c.writer, M.SocksaddrFromNet(addr))
	if err != nil {
		return
	}
	err = binary.Write(c.writer, binary.BigEndian, uint16(len(p)))
	if err != nil {
		return
	}
	_, err = c.Write(p)
	if err != nil {
		return
	}
	err = c.writer.Flush()
	if err != nil {
		return
	}
	return len(p), nil
}
