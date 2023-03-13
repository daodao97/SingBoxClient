package quic

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"os"

	"github.com/sagernet/quic-go"
	"github.com/sagernet/quic-go/http3"
	"github.com/sagernet/sing-dns"
	"github.com/sagernet/sing/common/bufio"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	mDNS "github.com/miekg/dns"
)

var _ dns.Transport = (*HTTP3Transport)(nil)

func init() {
	dns.RegisterTransport([]string{"h3"}, CreateHTTP3Transport)
}

func CreateHTTP3Transport(ctx context.Context, logger logger.ContextLogger, dialer N.Dialer, link string) (dns.Transport, error) {
	linkURL, err := url.Parse(link)
	if err != nil {
		return nil, err
	}
	linkURL.Scheme = "https"
	return NewHTTP3Transport(dialer, linkURL.String()), nil
}

type HTTP3Transport struct {
	destination string
	transport   *http3.RoundTripper
}

func NewHTTP3Transport(dialer N.Dialer, serverURL string) *HTTP3Transport {
	return &HTTP3Transport{
		destination: serverURL,
		transport: &http3.RoundTripper{
			Dial: func(ctx context.Context, addr string, tlsCfg *tls.Config, cfg *quic.Config) (quic.EarlyConnection, error) {
				destinationAddr := M.ParseSocksaddr(addr)
				conn, err := dialer.DialContext(ctx, N.NetworkUDP, destinationAddr)
				if err != nil {
					return nil, err
				}
				return quic.DialEarlyContext(ctx, bufio.NewUnbindPacketConn(conn), conn.RemoteAddr(), destinationAddr.AddrString(), tlsCfg, cfg)
			},
			TLSClientConfig: &tls.Config{
				NextProtos: []string{"dns"},
			},
		},
	}
}

func (t *HTTP3Transport) Start() error {
	return nil
}

func (t *HTTP3Transport) Close() error {
	return t.transport.Close()
}

func (t *HTTP3Transport) Raw() bool {
	return true
}

func (t *HTTP3Transport) Exchange(ctx context.Context, message *mDNS.Msg) (*mDNS.Msg, error) {
	message.Id = 0
	rawMessage, err := message.Pack()
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, t.destination, bytes.NewReader(rawMessage))
	if err != nil {
		return nil, err
	}
	request.Header.Set("content-type", dns.MimeType)
	request.Header.Set("accept", dns.MimeType)

	client := &http.Client{Transport: t.transport}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	rawMessage, err = io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	var responseMessage mDNS.Msg
	err = responseMessage.Unpack(rawMessage)
	if err != nil {
		return nil, err
	}
	return &responseMessage, nil
}

func (t *HTTP3Transport) Lookup(ctx context.Context, domain string, strategy dns.DomainStrategy) ([]netip.Addr, error) {
	return nil, os.ErrInvalid
}
