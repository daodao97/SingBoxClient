package dns

import (
	"context"
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

const FixedPacketSize = 16384

var _ Transport = (*UDPTransport)(nil)

func init() {
	RegisterTransport([]string{"udp", ""}, CreateUDPTransport)
}

func CreateUDPTransport(name string, ctx context.Context, logger logger.ContextLogger, dialer N.Dialer, link string) (Transport, error) {
	serverURL, err := url.Parse(link)
	if err != nil || serverURL.Scheme == "" {
		return NewUDPTransport(name, ctx, dialer, M.ParseSocksaddr(link))
	}
	return NewUDPTransport(name, ctx, dialer, M.ParseSocksaddr(serverURL.Host))
}

type UDPTransport struct {
	myTransportAdapter
}

func NewUDPTransport(name string, ctx context.Context, dialer N.Dialer, serverAddr M.Socksaddr) (*UDPTransport, error) {
	if !serverAddr.IsValid() {
		return nil, E.New("invalid server address")
	}
	if serverAddr.Port == 0 {
		serverAddr.Port = 53
	}
	transport := &UDPTransport{
		newAdapter(name, ctx, dialer, serverAddr),
	}
	transport.handler = transport
	return transport, nil
}

func (t *UDPTransport) DialContext(ctx context.Context, queryCtx context.Context) (net.Conn, error) {
	return t.dialer.DialContext(ctx, "udp", t.serverAddr)
}

func (t *UDPTransport) ReadMessage(conn net.Conn) (*dns.Msg, error) {
	_buffer := buf.StackNewSize(FixedPacketSize)
	defer common.KeepAlive(_buffer)
	buffer := common.Dup(_buffer)
	defer buffer.Release()
	_, err := buffer.ReadOnceFrom(conn)
	if err != nil {
		return nil, err
	}
	var message dns.Msg
	err = message.Unpack(buffer.Bytes())
	return &message, err
}

func (t *UDPTransport) WriteMessage(conn net.Conn, message *dns.Msg) error {
	rawMessage, err := message.Pack()
	if err != nil {
		return err
	}
	return common.Error(conn.Write(rawMessage))
}
