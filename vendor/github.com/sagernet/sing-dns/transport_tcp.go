package dns

import (
	"context"
	"encoding/binary"
	"net"
	"net/url"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"github.com/miekg/dns"
)

var _ Transport = (*TCPTransport)(nil)

func init() {
	RegisterTransport([]string{"tcp"}, CreateTCPTransport)
}

func CreateTCPTransport(name string, ctx context.Context, logger logger.ContextLogger, dialer N.Dialer, link string) (Transport, error) {
	serverURL, err := url.Parse(link)
	if err != nil {
		return nil, err
	}
	return NewTCPTransport(name, ctx, dialer, M.ParseSocksaddr(serverURL.Host))
}

type TCPTransport struct {
	myTransportAdapter
}

func NewTCPTransport(name string, ctx context.Context, dialer N.Dialer, serverAddr M.Socksaddr) (*TCPTransport, error) {
	if !serverAddr.IsValid() {
		return nil, E.New("invalid server address")
	}
	if serverAddr.Port == 0 {
		serverAddr.Port = 53
	}
	transport := &TCPTransport{
		newAdapter(name, ctx, dialer, serverAddr),
	}
	transport.handler = transport
	return transport, nil
}

func (t *TCPTransport) DialContext(ctx context.Context, queryCtx context.Context) (net.Conn, error) {
	return t.dialer.DialContext(ctx, N.NetworkTCP, t.serverAddr)
}

func (t *TCPTransport) ReadMessage(conn net.Conn) (*dns.Msg, error) {
	var length uint16
	err := binary.Read(conn, binary.BigEndian, &length)
	if err != nil {
		return nil, err
	}
	if length < 10 {
		return nil, dns.ErrShortRead
	}
	_buffer := buf.StackNewSize(int(length))
	defer common.KeepAlive(_buffer)
	buffer := common.Dup(_buffer)
	defer buffer.Release()
	_, err = buffer.ReadFullFrom(conn, int(length))
	if err != nil {
		return nil, err
	}
	var message dns.Msg
	err = message.Unpack(buffer.Bytes())
	return &message, err
}

func (t *TCPTransport) WriteMessage(conn net.Conn, message *dns.Msg) error {
	rawMessage, err := message.Pack()
	if err != nil {
		return err
	}
	_buffer := buf.StackNewSize(2 + len(rawMessage))
	defer common.KeepAlive(_buffer)
	buffer := common.Dup(_buffer)
	defer buffer.Release()
	common.Must(binary.Write(buffer, binary.BigEndian, uint16(len(rawMessage))))
	common.Must1(buffer.Write(rawMessage))
	return common.Error(conn.Write(buffer.Bytes()))
}
