package uot

import (
	"encoding/binary"
	"io"
	"net"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"
)

type ServerConn struct {
	net.PacketConn
	inputReader, outputReader *io.PipeReader
	inputWriter, outputWriter *io.PipeWriter
}

func NewServerConn(packetConn net.PacketConn) net.Conn {
	c := &ServerConn{
		PacketConn: packetConn,
	}
	c.inputReader, c.inputWriter = io.Pipe()
	c.outputReader, c.outputWriter = io.Pipe()
	go c.loopInput()
	go c.loopOutput()
	return c
}

func (c *ServerConn) Read(b []byte) (n int, err error) {
	return c.outputReader.Read(b)
}

func (c *ServerConn) Write(b []byte) (n int, err error) {
	return c.inputWriter.Write(b)
}

type pipeAddr struct{}

func (pipeAddr) Network() string { return "pipe" }
func (pipeAddr) String() string  { return "pipe" }

func (c *ServerConn) RemoteAddr() net.Addr {
	return pipeAddr{}
}

//warn:unsafe
func (c *ServerConn) loopInput() {
	_buffer := buf.StackNew()
	defer common.KeepAlive(_buffer)
	buffer := common.Dup(_buffer)
	defer buffer.Release()
	for {
		destination, err := AddrParser.ReadAddrPort(c.inputReader)
		if err != nil {
			break
		}
		if destination.IsFqdn() {
			addr, err := net.ResolveUDPAddr("udp", destination.String())
			if err != nil {
				continue
			}
			destination = M.SocksaddrFromNet(addr)
		}
		var length uint16
		err = binary.Read(c.inputReader, binary.BigEndian, &length)
		if err != nil {
			break
		}
		buffer.FullReset()
		_, err = buffer.ReadFullFrom(c.inputReader, int(length))
		if err != nil {
			break
		}
		_, err = c.WriteTo(buffer.Bytes(), destination.UDPAddr())
		if err != nil {
			break
		}
	}
	c.Close()
}

//warn:unsafe
func (c *ServerConn) loopOutput() {
	_buffer := buf.StackNew()
	defer common.KeepAlive(_buffer)
	buffer := common.Dup(_buffer)
	defer buffer.Release()
	for {
		buffer.FullReset()
		n, addr, err := buffer.ReadPacketFrom(c)
		if err != nil {
			break
		}
		err = AddrParser.WriteAddrPort(c.outputWriter, M.SocksaddrFromNet(addr))
		if err != nil {
			break
		}
		err = binary.Write(c.outputWriter, binary.BigEndian, uint16(n))
		if err != nil {
			break
		}
		_, err = buffer.WriteTo(c.outputWriter)
		if err != nil {
			break
		}
	}
	c.Close()
}

func (c *ServerConn) Close() error {
	c.inputReader.Close()
	c.inputWriter.Close()
	c.outputReader.Close()
	c.outputWriter.Close()
	c.PacketConn.Close()
	return nil
}
