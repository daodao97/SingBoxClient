package dns

import (
	"context"
	"net"
	"time"

	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type DialerWrapper struct {
	dialer        N.Dialer
	client        *Client
	transport     Transport
	strategy      DomainStrategy
	fallbackDelay time.Duration
}

func NewDialerWrapper(dialer N.Dialer, client *Client, transport Transport, strategy DomainStrategy, fallbackDelay time.Duration) N.Dialer {
	return &DialerWrapper{dialer, client, transport, strategy, fallbackDelay}
}

func (d *DialerWrapper) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	if destination.IsIP() {
		return d.dialer.DialContext(ctx, network, destination)
	}
	addresses, err := d.client.Lookup(ctx, d.transport, destination.Fqdn, d.strategy)
	if err != nil {
		return nil, err
	}
	return N.DialParallel(ctx, d.dialer, network, destination, addresses, d.strategy == DomainStrategyPreferIPv6, d.fallbackDelay)
}

func (d *DialerWrapper) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	if destination.IsIP() {
		return d.dialer.ListenPacket(ctx, destination)
	}
	addresses, err := d.client.Lookup(ctx, d.transport, destination.Fqdn, d.strategy)
	if err != nil {
		return nil, err
	}
	conn, _, err := N.ListenSerial(ctx, d.dialer, destination, addresses)
	return conn, err
}

func (d *DialerWrapper) Upstream() any {
	return d.dialer
}
