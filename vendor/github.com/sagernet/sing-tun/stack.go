package tun

import (
	"context"
	"net/netip"

	"github.com/sagernet/sing/common/control"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
)

type Stack interface {
	Start() error
	Close() error
}

type StackOptions struct {
	Context                context.Context
	Tun                    Tun
	Name                   string
	MTU                    uint32
	Inet4Address           []netip.Prefix
	Inet6Address           []netip.Prefix
	EndpointIndependentNat bool
	UDPTimeout             int64
	Router                 Router
	Handler                Handler
	Logger                 logger.Logger
	ForwarderBindInterface bool
	InterfaceFinder        control.InterfaceFinder
}

func NewStack(
	stack string,
	options StackOptions,
) (Stack, error) {
	switch stack {
	case "":
		return NewSystem(options)
	case "gvisor":
		return NewGVisor(options)
	case "system":
		return NewSystem(options)
	case "lwip":
		return NewLWIP(options)
	default:
		return nil, E.New("unknown stack: ", stack)
	}
}
