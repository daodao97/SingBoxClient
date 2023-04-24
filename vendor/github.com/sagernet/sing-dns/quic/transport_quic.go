package quic

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"net/netip"
	"net/url"
	"os"
	"sync"

	"github.com/sagernet/quic-go"
	"github.com/sagernet/sing-dns"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	mDNS "github.com/miekg/dns"
)

var _ dns.Transport = (*Transport)(nil)

func init() {
	dns.RegisterTransport([]string{"quic"}, CreateTransport)
}

func CreateTransport(name string, ctx context.Context, logger logger.ContextLogger, dialer N.Dialer, link string) (dns.Transport, error) {
	serverURL, err := url.Parse(link)
	if err != nil {
		return nil, err
	}
	return NewTransport(name, ctx, dialer, M.ParseSocksaddr(serverURL.Host))
}

type Transport struct {
	name       string
	ctx        context.Context
	dialer     N.Dialer
	serverAddr M.Socksaddr

	access     sync.Mutex
	connection quic.EarlyConnection
}

func NewTransport(name string, ctx context.Context, dialer N.Dialer, serverAddr M.Socksaddr) (*Transport, error) {
	if !serverAddr.IsValid() {
		return nil, E.New("invalid server address")
	}
	if serverAddr.Port == 0 {
		serverAddr.Port = 853
	}
	return &Transport{
		name:       name,
		ctx:        ctx,
		dialer:     dialer,
		serverAddr: serverAddr,
	}, nil
}

func (t *Transport) Name() string {
	return t.name
}

func (t *Transport) Start() error {
	return nil
}

func (t *Transport) Close() error {
	connection := t.connection
	if connection != nil {
		connection.CloseWithError(0, "")
	}
	return nil
}

func (t *Transport) Raw() bool {
	return true
}

func (t *Transport) openConnection() (quic.EarlyConnection, error) {
	connection := t.connection
	if connection != nil && !common.Done(connection.Context()) {
		return connection, nil
	}
	t.access.Lock()
	defer t.access.Unlock()
	connection = t.connection
	if connection != nil && !common.Done(connection.Context()) {
		return connection, nil
	}
	conn, err := t.dialer.DialContext(t.ctx, N.NetworkUDP, t.serverAddr)
	if err != nil {
		return nil, err
	}
	earlyConnection, err := quic.DialEarly(
		bufio.NewUnbindPacketConn(conn),
		t.serverAddr.UDPAddr(),
		t.serverAddr.AddrString(),
		&tls.Config{NextProtos: []string{"doq"}},
		nil,
	)
	if err != nil {
		return nil, err
	}
	t.connection = earlyConnection
	return earlyConnection, nil
}

func (t *Transport) Exchange(ctx context.Context, message *mDNS.Msg) (*mDNS.Msg, error) {
	var (
		conn     quic.Connection
		err      error
		response *mDNS.Msg
	)
	for i := 0; i < 2; i++ {
		conn, err = t.openConnection()
		if conn == nil {
			return nil, err
		}
		response, err = t.exchange(ctx, message, conn)
		if err == nil {
			return response, nil
		} else if !isQUICRetryError(err) {
			return nil, err
		} else {
			conn.CloseWithError(quic.ApplicationErrorCode(0), "")
			continue
		}
	}
	return nil, err
}

func (t *Transport) exchange(ctx context.Context, message *mDNS.Msg, conn quic.Connection) (*mDNS.Msg, error) {
	message.Id = 0
	rawMessage, err := message.Pack()
	if err != nil {
		return nil, err
	}
	_buffer := buf.StackNewSize(2 + len(rawMessage))
	defer common.KeepAlive(_buffer)
	buffer := common.Dup(_buffer)
	defer buffer.Release()
	common.Must(binary.Write(buffer, binary.BigEndian, uint16(len(rawMessage))))
	common.Must1(buffer.Write(rawMessage))
	stream, err := conn.OpenStreamSync(ctx)
	if err != nil {
		return nil, err
	}
	defer stream.Close()
	defer stream.CancelRead(0)
	_, err = stream.Write(buffer.Bytes())
	if err != nil {
		return nil, err
	}
	buffer.FullReset()
	_, err = buffer.ReadFullFrom(stream, 2)
	if err != nil {
		return nil, err
	}
	responseLen := int(binary.BigEndian.Uint16(buffer.Bytes()))
	buffer.FullReset()
	if buffer.FreeLen() < responseLen {
		buffer.Release()
		_buffer = buf.StackNewSize(responseLen)
		buffer = common.Dup(_buffer)
	}
	_, err = buffer.ReadFullFrom(stream, responseLen)
	if err != nil {
		return nil, err
	}
	var responseMessage mDNS.Msg
	err = responseMessage.Unpack(buffer.Bytes())
	if err != nil {
		return nil, err
	}
	return &responseMessage, nil
}

func (t *Transport) Lookup(ctx context.Context, domain string, strategy dns.DomainStrategy) ([]netip.Addr, error) {
	return nil, os.ErrInvalid
}

// https://github.com/AdguardTeam/dnsproxy/blob/fd1868577652c639cce3da00e12ca548f421baf1/upstream/upstream_quic.go#L394
func isQUICRetryError(err error) (ok bool) {
	var qAppErr *quic.ApplicationError
	if errors.As(err, &qAppErr) && qAppErr.ErrorCode == 0 {
		return true
	}

	var qIdleErr *quic.IdleTimeoutError
	if errors.As(err, &qIdleErr) {
		return true
	}

	var resetErr *quic.StatelessResetError
	if errors.As(err, &resetErr) {
		return true
	}

	var qTransportError *quic.TransportError
	if errors.As(err, &qTransportError) && qTransportError.ErrorCode == quic.NoError {
		return true
	}

	if errors.Is(err, quic.Err0RTTRejected) {
		return true
	}

	return false
}
