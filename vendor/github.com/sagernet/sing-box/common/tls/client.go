package tls

import (
	"context"
	"crypto/tls"
	"net"
	"os"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/badtls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

func NewDialerFromOptions(router adapter.Router, dialer N.Dialer, serverAddress string, options option.OutboundTLSOptions) (N.Dialer, error) {
	if !options.Enabled {
		return dialer, nil
	}
	config, err := NewClient(router, serverAddress, options)
	if err != nil {
		return nil, err
	}
	return NewDialer(dialer, config), nil
}

func NewClient(router adapter.Router, serverAddress string, options option.OutboundTLSOptions) (Config, error) {
	if !options.Enabled {
		return nil, nil
	}
	if options.ECH != nil && options.ECH.Enabled {
		return NewECHClient(router, serverAddress, options)
	} else if options.Reality != nil && options.Reality.Enabled {
		return NewRealityClient(router, serverAddress, options)
	} else if options.UTLS != nil && options.UTLS.Enabled {
		return NewUTLSClient(router, serverAddress, options)
	} else {
		return NewSTDClient(router, serverAddress, options)
	}
}

func ClientHandshake(ctx context.Context, conn net.Conn, config Config) (Conn, error) {
	ctx, cancel := context.WithTimeout(ctx, C.TCPTimeout)
	defer cancel()
	tlsConn, err := config.Client(conn)
	if err != nil {
		return nil, err
	}
	err = tlsConn.HandshakeContext(ctx)
	if err != nil {
		return nil, err
	}
	if stdConn, isSTD := tlsConn.(*tls.Conn); isSTD {
		var badConn badtls.TLSConn
		badConn, err = badtls.Create(stdConn)
		if err == nil {
			return badConn, nil
		}
	}
	return tlsConn, nil
}

type Dialer struct {
	dialer N.Dialer
	config Config
}

func NewDialer(dialer N.Dialer, config Config) N.Dialer {
	return &Dialer{dialer, config}
}

func (d *Dialer) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	if network != N.NetworkTCP {
		return nil, os.ErrInvalid
	}
	conn, err := d.dialer.DialContext(ctx, network, destination)
	if err != nil {
		return nil, err
	}
	return ClientHandshake(ctx, conn, d.config)
}

func (d *Dialer) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	return nil, os.ErrInvalid
}
