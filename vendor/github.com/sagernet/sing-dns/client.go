package dns

import (
	"context"
	"net"
	"net/netip"
	"time"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/cache"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/task"

	"github.com/miekg/dns"
)

const DefaultTTL = 600

var (
	ErrNoRawSupport = E.New("no raw query support by current transport")
	ErrNotCached    = E.New("not cached")
)

type Client struct {
	disableCache  bool
	disableExpire bool
	logger        logger.ContextLogger

	cache *cache.LruCache[dns.Question, *dns.Msg]
}

func NewClient(disableCache bool, disableExpire bool, logger logger.ContextLogger) *Client {
	client := &Client{
		disableCache:  disableCache,
		disableExpire: disableExpire,
		logger:        logger,
	}
	if !disableCache {
		client.cache = cache.New[dns.Question, *dns.Msg]()
	}
	return client
}

func (c *Client) Exchange(ctx context.Context, transport Transport, message *dns.Msg, strategy DomainStrategy) (*dns.Msg, error) {
	response, err := c.exchange(ctx, transport, message, strategy)
	if err != nil {
		return nil, err
	}
	if rewriteTTL, loaded := RewriteTTLFromContext(ctx); loaded {
		for _, recordList := range [][]dns.RR{response.Answer, response.Ns, response.Extra} {
			for _, record := range recordList {
				record.Header().Ttl = rewriteTTL
			}
		}
	}
	return response, nil
}

func (c *Client) exchange(ctx context.Context, transport Transport, message *dns.Msg, strategy DomainStrategy) (*dns.Msg, error) {
	if len(message.Question) != 1 {
		if c.logger != nil {
			c.logger.WarnContext(ctx, "bad question size: ", len(message.Question))
		}
		responseMessage := dns.Msg{
			MsgHdr: dns.MsgHdr{
				Id:       message.Id,
				Response: true,
				Rcode:    dns.RcodeFormatError,
			},
			Question: message.Question,
		}
		return &responseMessage, nil
	}
	question := message.Question[0]
	disableCache := c.disableCache || DisableCacheFromContext(ctx)
	if !disableCache {
		response, ttl := c.loadResponse(question)
		if response != nil {
			logCachedResponse(c.logger, ctx, response, ttl)
			response.Id = message.Id
			return response, nil
		}
	}
	if question.Qtype == dns.TypeA && strategy == DomainStrategyUseIPv6 || question.Qtype == dns.TypeAAAA && strategy == DomainStrategyUseIPv4 {
		responseMessage := dns.Msg{
			MsgHdr: dns.MsgHdr{
				Id:       message.Id,
				Response: true,
				Rcode:    dns.RcodeSuccess,
			},
			Question: []dns.Question{question},
		}
		if c.logger != nil {
			c.logger.DebugContext(ctx, "strategy rejected")
		}
		return &responseMessage, nil
	}
	if !transport.Raw() {
		if question.Qtype == dns.TypeA || question.Qtype == dns.TypeAAAA {
			return c.exchangeToLookup(ctx, transport, message, question)
		}
		return nil, ErrNoRawSupport
	}
	messageId := message.Id
	contextTransport, loaded := transportNameFromContext(ctx)
	if loaded && transport.Name() == contextTransport {
		return nil, E.New("DNS query loopback in transport[", contextTransport, "]")
	}
	ctx = contextWithTransportName(ctx, transport.Name())
	response, err := transport.Exchange(ctx, message)
	if err != nil {
		return nil, err
	}
	var timeToLive int
	for _, recordList := range [][]dns.RR{response.Answer, response.Ns, response.Extra} {
		for _, record := range recordList {
			if timeToLive == 0 || record.Header().Ttl > 0 && int(record.Header().Ttl) < timeToLive {
				timeToLive = int(record.Header().Ttl)
			}
		}
	}
	if timeToLive == 0 {
		timeToLive = DefaultTTL
	}
	logExchangedResponse(c.logger, ctx, response, timeToLive)

	response.Id = messageId
	if !disableCache {
		c.storeCache(question, response, timeToLive)
	}

	return response, err
}

func (c *Client) Lookup(ctx context.Context, transport Transport, domain string, strategy DomainStrategy) ([]netip.Addr, error) {
	if dns.IsFqdn(domain) {
		domain = domain[:len(domain)-1]
	}
	dnsName := dns.Fqdn(domain)
	if transport.Raw() {
		if strategy == DomainStrategyUseIPv4 {
			return c.lookupToExchange(ctx, transport, dnsName, dns.TypeA, strategy)
		} else if strategy == DomainStrategyUseIPv6 {
			return c.lookupToExchange(ctx, transport, dnsName, dns.TypeAAAA, strategy)
		}
		var response4 []netip.Addr
		var response6 []netip.Addr
		var group task.Group
		group.Append("exchange4", func(ctx context.Context) error {
			response, err := c.lookupToExchange(ctx, transport, dnsName, dns.TypeA, strategy)
			if err != nil {
				return err
			}
			response4 = response
			return nil
		})
		group.Append("exchange6", func(ctx context.Context) error {
			response, err := c.lookupToExchange(ctx, transport, dnsName, dns.TypeAAAA, strategy)
			if err != nil {
				return err
			}
			response6 = response
			return nil
		})
		err := group.Run(ctx)
		if len(response4) == 0 && len(response6) == 0 {
			return nil, err
		}
		return sortAddresses(response4, response6, strategy), nil
	}
	disableCache := c.disableCache || DisableCacheFromContext(ctx)
	if !disableCache {
		if strategy == DomainStrategyUseIPv4 {
			response, err := c.questionCache(dns.Question{
				Name:   dnsName,
				Qtype:  dns.TypeA,
				Qclass: dns.ClassINET,
			})
			if err != ErrNotCached {
				return response, err
			}
		} else if strategy == DomainStrategyUseIPv6 {
			response, err := c.questionCache(dns.Question{
				Name:   dnsName,
				Qtype:  dns.TypeAAAA,
				Qclass: dns.ClassINET,
			})
			if err != ErrNotCached {
				return response, err
			}
		} else {
			response4, _ := c.questionCache(dns.Question{
				Name:   dnsName,
				Qtype:  dns.TypeA,
				Qclass: dns.ClassINET,
			})
			response6, _ := c.questionCache(dns.Question{
				Name:   dnsName,
				Qtype:  dns.TypeAAAA,
				Qclass: dns.ClassINET,
			})
			if len(response4) > 0 || len(response6) > 0 {
				return sortAddresses(response4, response6, strategy), nil
			}
		}
	}
	var rCode int
	response, err := transport.Lookup(ctx, domain, strategy)
	if err != nil {
		err = wrapError(err)
		if rCodeError, isRCodeError := err.(RCodeError); !isRCodeError {
			return nil, err
		} else {
			rCode = int(rCodeError)
		}
		if disableCache {
			return nil, err
		}
	}
	header := dns.MsgHdr{
		Response: true,
		Rcode:    rCode,
	}
	if !disableCache {
		if strategy != DomainStrategyUseIPv6 {
			question4 := dns.Question{
				Name:   dnsName,
				Qtype:  dns.TypeA,
				Qclass: dns.ClassINET,
			}
			response4 := common.Filter(response, func(addr netip.Addr) bool {
				return addr.Is4() || addr.Is4In6()
			})
			message4 := &dns.Msg{
				MsgHdr:   header,
				Question: []dns.Question{question4},
			}
			if len(response4) > 0 {
				for _, address := range response4 {
					message4.Answer = append(message4.Answer, &dns.A{
						Hdr: dns.RR_Header{
							Name:   question4.Name,
							Rrtype: dns.TypeA,
							Class:  dns.ClassINET,
							Ttl:    DefaultTTL,
						},
						A: address.AsSlice(),
					})
				}
			}
			c.storeCache(question4, message4, DefaultTTL)
		}
		if strategy != DomainStrategyUseIPv4 {
			question6 := dns.Question{
				Name:   dnsName,
				Qtype:  dns.TypeAAAA,
				Qclass: dns.ClassINET,
			}
			response6 := common.Filter(response, func(addr netip.Addr) bool {
				return addr.Is6() && !addr.Is4In6()
			})
			message6 := &dns.Msg{
				MsgHdr:   header,
				Question: []dns.Question{question6},
			}
			if len(response6) > 0 {
				for _, address := range response6 {
					message6.Answer = append(message6.Answer, &dns.AAAA{
						Hdr: dns.RR_Header{
							Name:   question6.Name,
							Rrtype: dns.TypeAAAA,
							Class:  dns.ClassINET,
							Ttl:    DefaultTTL,
						},
						AAAA: address.AsSlice(),
					})
				}
			}
			c.storeCache(question6, message6, DefaultTTL)
		}
	}
	return response, err
}

func sortAddresses(response4 []netip.Addr, response6 []netip.Addr, strategy DomainStrategy) []netip.Addr {
	if strategy == DomainStrategyPreferIPv6 {
		return append(response6, response4...)
	} else {
		return append(response4, response6...)
	}
}

func (c *Client) storeCache(question dns.Question, message *dns.Msg, timeToLive int) {
	if c.disableExpire {
		c.cache.Store(question, message)
		return
	}
	expireAt := time.Now().Add(time.Second * time.Duration(timeToLive))
	c.cache.StoreWithExpire(question, message, expireAt)
}

func (c *Client) exchangeToLookup(ctx context.Context, transport Transport, message *dns.Msg, question dns.Question) (*dns.Msg, error) {
	domain := question.Name
	var strategy DomainStrategy
	if question.Qtype == dns.TypeA {
		strategy = DomainStrategyUseIPv4
	} else {
		strategy = DomainStrategyUseIPv6
	}
	var rCode int
	result, err := c.Lookup(ctx, transport, domain, strategy)
	if err != nil {
		err = wrapError(err)
		if rCodeError, isRCodeError := err.(RCodeError); !isRCodeError {
			return nil, err
		} else {
			rCode = int(rCodeError)
		}
	}
	response := dns.Msg{
		MsgHdr: dns.MsgHdr{
			Id:       message.Id,
			Rcode:    rCode,
			Response: true,
		},
		Question: message.Question,
	}
	var timeToLive uint32
	if rewriteTTL, loaded := RewriteTTLFromContext(ctx); loaded {
		timeToLive = rewriteTTL
	} else {
		timeToLive = DefaultTTL
	}
	for _, address := range result {
		if address.Is4In6() {
			address = netip.AddrFrom4(address.As4())
		}
		if address.Is4() {
			response.Answer = append(response.Answer, &dns.A{
				Hdr: dns.RR_Header{
					Name:   question.Name,
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    timeToLive,
				},
				A: address.AsSlice(),
			})
		} else {
			response.Answer = append(response.Answer, &dns.AAAA{
				Hdr: dns.RR_Header{
					Name:   question.Name,
					Rrtype: dns.TypeAAAA,
					Class:  dns.ClassINET,
					Ttl:    timeToLive,
				},
				AAAA: address.AsSlice(),
			})
		}
	}
	return &response, nil
}

func (c *Client) lookupToExchange(ctx context.Context, transport Transport, name string, qType uint16, strategy DomainStrategy) ([]netip.Addr, error) {
	question := dns.Question{
		Name:   name,
		Qtype:  qType,
		Qclass: dns.ClassINET,
	}
	disableCache := c.disableCache || DisableCacheFromContext(ctx)
	if !disableCache {
		cachedAddresses, err := c.questionCache(question)
		if err != ErrNotCached {
			return cachedAddresses, err
		}
	}
	message := dns.Msg{
		MsgHdr: dns.MsgHdr{
			RecursionDesired: true,
		},
		Question: []dns.Question{question},
	}
	response, err := c.Exchange(ctx, transport, &message, strategy)
	if err != nil {
		return nil, err
	}
	return messageToAddresses(response)
}

func (c *Client) questionCache(question dns.Question) ([]netip.Addr, error) {
	response, _ := c.loadResponse(question)
	if response == nil {
		return nil, ErrNotCached
	}
	return messageToAddresses(response)
}

func (c *Client) loadResponse(question dns.Question) (*dns.Msg, int) {
	if c.disableExpire {
		response, loaded := c.cache.Load(question)
		if !loaded {
			return nil, 0
		}
		return response.Copy(), 0
	} else {
		cachedAnswer, expireAt, loaded := c.cache.LoadWithExpire(question)
		if !loaded {
			return nil, 0
		}
		timeNow := time.Now()
		if timeNow.After(expireAt) {
			c.cache.Delete(question)
			return nil, 0
		}
		var originTTL int
		for _, recordList := range [][]dns.RR{cachedAnswer.Answer, cachedAnswer.Ns, cachedAnswer.Extra} {
			for _, record := range recordList {
				if originTTL == 0 || record.Header().Ttl > 0 && int(record.Header().Ttl) < originTTL {
					originTTL = int(record.Header().Ttl)
				}
			}
		}
		nowTTL := int(expireAt.Sub(timeNow).Seconds())
		if nowTTL < 0 {
			nowTTL = 0
		}
		response := cachedAnswer.Copy()
		if originTTL > 0 {
			duration := uint32(originTTL - nowTTL)
			for _, recordList := range [][]dns.RR{response.Answer, response.Ns, response.Extra} {
				for _, record := range recordList {
					newTTL := record.Header().Ttl - duration
					if newTTL < 0 {
						newTTL = 0
					}
					record.Header().Ttl = newTTL
				}
			}
		} else {
			for _, recordList := range [][]dns.RR{response.Answer, response.Ns, response.Extra} {
				for _, record := range recordList {
					record.Header().Ttl = uint32(nowTTL)
				}
			}
		}
		return response, nowTTL
	}
}

func messageToAddresses(response *dns.Msg) ([]netip.Addr, error) {
	if response.Rcode != dns.RcodeSuccess {
		return nil, RCodeError(response.Rcode)
	} else if len(response.Answer) == 0 {
		return nil, RCodeSuccess
	}
	addresses := make([]netip.Addr, 0, len(response.Answer))
	for _, rawAnswer := range response.Answer {
		switch answer := rawAnswer.(type) {
		case *dns.A:
			addresses = append(addresses, M.AddrFromIP(answer.A))
		case *dns.AAAA:
			addresses = append(addresses, M.AddrFromIP(answer.AAAA))
		}
	}
	return addresses, nil
}

func wrapError(err error) error {
	switch dnsErr := err.(type) {
	case *net.DNSError:
		if dnsErr.IsNotFound {
			return RCodeNameError
		}
	case *net.AddrError:
		return RCodeNameError
	}
	return err
}
