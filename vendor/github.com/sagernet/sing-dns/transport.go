package dns

import (
	"context"
	"net/netip"
	"net/url"

	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	N "github.com/sagernet/sing/common/network"

	"github.com/miekg/dns"
)

type TransportConstructor = func(name string, ctx context.Context, logger logger.ContextLogger, dialer N.Dialer, link string) (Transport, error)

type Transport interface {
	Name() string
	Start() error
	Close() error
	Raw() bool
	Exchange(ctx context.Context, message *dns.Msg) (*dns.Msg, error)
	Lookup(ctx context.Context, domain string, strategy DomainStrategy) ([]netip.Addr, error)
}

var transports map[string]TransportConstructor

func RegisterTransport(schemes []string, constructor TransportConstructor) {
	if transports == nil {
		transports = make(map[string]TransportConstructor)
	}
	for _, scheme := range schemes {
		transports[scheme] = constructor
	}
}

func CreateTransport(name string, ctx context.Context, logger logger.ContextLogger, dialer N.Dialer, address string) (Transport, error) {
	constructor := transports[address]
	if constructor == nil {
		serverURL, _ := url.Parse(address)
		var scheme string
		if serverURL != nil {
			scheme = serverURL.Scheme
		}
		constructor = transports[scheme]
	}
	if constructor == nil {
		return nil, E.New("unknown DNS server format: " + address)
	}
	return constructor(name, contextWithTransportName(ctx, name), logger, dialer, address)
}
