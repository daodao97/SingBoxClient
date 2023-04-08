package dns

import (
	"context"
	"net/netip"
	"net/url"
	"os"

	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	N "github.com/sagernet/sing/common/network"

	"github.com/miekg/dns"
)

var _ Transport = (*RCodeTransport)(nil)

func init() {
	RegisterTransport([]string{"rcode"}, CreateRCodeTransport)
}

func CreateRCodeTransport(name string, ctx context.Context, logger logger.ContextLogger, dialer N.Dialer, link string) (Transport, error) {
	serverURL, err := url.Parse(link)
	if err != nil {
		return nil, err
	}
	return NewRCodeTransport(name, serverURL.Host)
}

type RCodeTransport struct {
	name string
	code RCodeError
}

func NewRCodeTransport(name string, code string) (*RCodeTransport, error) {
	switch code {
	case "success":
		return &RCodeTransport{name, RCodeSuccess}, nil
	case "format_error":
		return &RCodeTransport{name, RCodeFormatError}, nil
	case "server_failure":
		return &RCodeTransport{name, RCodeServerFailure}, nil
	case "name_error":
		return &RCodeTransport{name, RCodeNameError}, nil
	case "not_implemented":
		return &RCodeTransport{name, RCodeNotImplemented}, nil
	case "refused":
		return &RCodeTransport{name, RCodeRefused}, nil
	default:
		return nil, E.New("unknown rcode: " + code)
	}
}

func (t *RCodeTransport) Name() string {
	return t.name
}

func (t *RCodeTransport) Start() error {
	return nil
}

func (t *RCodeTransport) Close() error {
	return nil
}

func (t *RCodeTransport) Raw() bool {
	return true
}

func (t *RCodeTransport) Exchange(ctx context.Context, message *dns.Msg) (*dns.Msg, error) {
	message.Response = true
	message.Rcode = int(t.code)
	return message, nil
}

func (t *RCodeTransport) Lookup(ctx context.Context, domain string, strategy DomainStrategy) ([]netip.Addr, error) {
	return nil, os.ErrInvalid
}
