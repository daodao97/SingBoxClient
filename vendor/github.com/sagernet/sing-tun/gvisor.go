//go:build with_gvisor

package tun

import (
	"context"
	"net"
	"syscall"
	"time"

	"github.com/sagernet/sing/common/bufio"
	"github.com/sagernet/sing/common/canceler"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"

	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv6"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/icmp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"
	"gvisor.dev/gvisor/pkg/waiter"
)

const WithGVisor = true

const defaultNIC tcpip.NICID = 1

type GVisor struct {
	ctx                    context.Context
	tun                    GVisorTun
	tunMtu                 uint32
	endpointIndependentNat bool
	udpTimeout             int64
	router                 Router
	handler                Handler
	logger                 logger.Logger
	stack                  *stack.Stack
	endpoint               stack.LinkEndpoint
	routeMapping           *RouteMapping
}

type GVisorTun interface {
	Tun
	NewEndpoint() (stack.LinkEndpoint, error)
}

func NewGVisor(
	options StackOptions,
) (Stack, error) {
	gTun, isGTun := options.Tun.(GVisorTun)
	if !isGTun {
		return nil, E.New("gVisor stack is unsupported on current platform")
	}

	return &GVisor{
		ctx:                    options.Context,
		tun:                    gTun,
		tunMtu:                 options.MTU,
		endpointIndependentNat: options.EndpointIndependentNat,
		udpTimeout:             options.UDPTimeout,
		router:                 options.Router,
		handler:                options.Handler,
		logger:                 options.Logger,
		routeMapping:           NewRouteMapping(options.UDPTimeout),
	}, nil
}

func (t *GVisor) Start() error {
	linkEndpoint, err := t.tun.NewEndpoint()
	if err != nil {
		return err
	}
	ipStack := stack.New(stack.Options{
		NetworkProtocols: []stack.NetworkProtocolFactory{
			ipv4.NewProtocol,
			ipv6.NewProtocol,
		},
		TransportProtocols: []stack.TransportProtocolFactory{
			tcp.NewProtocol,
			udp.NewProtocol,
			icmp.NewProtocol4,
			icmp.NewProtocol6,
		},
	})
	tErr := ipStack.CreateNIC(defaultNIC, linkEndpoint)
	if tErr != nil {
		return E.New("create nic: ", wrapStackError(tErr))
	}
	ipStack.SetRouteTable([]tcpip.Route{
		{Destination: header.IPv4EmptySubnet, NIC: defaultNIC},
		{Destination: header.IPv6EmptySubnet, NIC: defaultNIC},
	})
	ipStack.SetSpoofing(defaultNIC, true)
	ipStack.SetPromiscuousMode(defaultNIC, true)
	bufSize := 20 * 1024
	ipStack.SetTransportProtocolOption(tcp.ProtocolNumber, &tcpip.TCPReceiveBufferSizeRangeOption{
		Min:     1,
		Default: bufSize,
		Max:     bufSize,
	})
	ipStack.SetTransportProtocolOption(tcp.ProtocolNumber, &tcpip.TCPSendBufferSizeRangeOption{
		Min:     1,
		Default: bufSize,
		Max:     bufSize,
	})
	sOpt := tcpip.TCPSACKEnabled(true)
	ipStack.SetTransportProtocolOption(tcp.ProtocolNumber, &sOpt)
	mOpt := tcpip.TCPModerateReceiveBufferOption(true)
	ipStack.SetTransportProtocolOption(tcp.ProtocolNumber, &mOpt)

	tcpForwarder := tcp.NewForwarder(ipStack, 0, 1024, func(r *tcp.ForwarderRequest) {
		var wq waiter.Queue
		handshakeCtx, cancel := context.WithCancel(context.Background())
		go func() {
			select {
			case <-t.ctx.Done():
				wq.Notify(wq.Events())
			case <-handshakeCtx.Done():
			}
		}()
		endpoint, err := r.CreateEndpoint(&wq)
		cancel()
		if err != nil {
			r.Complete(true)
			return
		}
		r.Complete(false)
		endpoint.SocketOptions().SetKeepAlive(true)
		keepAliveIdle := tcpip.KeepaliveIdleOption(15 * time.Second)
		endpoint.SetSockOpt(&keepAliveIdle)
		keepAliveInterval := tcpip.KeepaliveIntervalOption(15 * time.Second)
		endpoint.SetSockOpt(&keepAliveInterval)
		tcpConn := gonet.NewTCPConn(&wq, endpoint)
		lAddr := tcpConn.RemoteAddr()
		rAddr := tcpConn.LocalAddr()
		if lAddr == nil || rAddr == nil {
			tcpConn.Close()
			return
		}
		go func() {
			var metadata M.Metadata
			metadata.Source = M.SocksaddrFromNet(lAddr)
			metadata.Destination = M.SocksaddrFromNet(rAddr)
			hErr := t.handler.NewConnection(t.ctx, &gTCPConn{tcpConn}, metadata)
			if hErr != nil {
				endpoint.Abort()
			}
		}()
	})
	ipStack.SetTransportProtocolHandler(tcp.ProtocolNumber, func(id stack.TransportEndpointID, buffer *stack.PacketBuffer) bool {
		if t.router != nil {
			var routeSession RouteSession
			routeSession.Network = syscall.IPPROTO_TCP
			var ipHdr header.Network
			if buffer.NetworkProtocolNumber == header.IPv4ProtocolNumber {
				routeSession.IPVersion = 4
				ipHdr = header.IPv4(buffer.NetworkHeader().Slice())
			} else {
				routeSession.IPVersion = 6
				ipHdr = header.IPv6(buffer.NetworkHeader().Slice())
			}
			tcpHdr := header.TCP(buffer.TransportHeader().Slice())
			routeSession.Source = M.AddrPortFrom(net.IP(ipHdr.SourceAddress()), tcpHdr.SourcePort())
			routeSession.Destination = M.AddrPortFrom(net.IP(ipHdr.DestinationAddress()), tcpHdr.DestinationPort())
			action := t.routeMapping.Lookup(routeSession, func() RouteAction {
				if routeSession.IPVersion == 4 {
					return t.router.RouteConnection(routeSession, &systemTCPDirectPacketWriter4{t.tun, routeSession.Source})
				} else {
					return t.router.RouteConnection(routeSession, &systemTCPDirectPacketWriter6{t.tun, routeSession.Source})
				}
			})
			switch actionType := action.(type) {
			case *ActionReject:
				// TODO: send icmp unreachable
				return true
			case *ActionDirect:
				buffer.IncRef()
				err = actionType.WritePacketBuffer(buffer)
				if err != nil {
					t.logger.Trace("route gvisor tcp packet: ", err)
				}
				return true
			}
		}
		return tcpForwarder.HandlePacket(id, buffer)
	})

	if !t.endpointIndependentNat {
		udpForwarder := udp.NewForwarder(ipStack, func(request *udp.ForwarderRequest) {
			var wq waiter.Queue
			endpoint, err := request.CreateEndpoint(&wq)
			if err != nil {
				return
			}
			udpConn := gonet.NewUDPConn(ipStack, &wq, endpoint)
			lAddr := udpConn.RemoteAddr()
			rAddr := udpConn.LocalAddr()
			if lAddr == nil || rAddr == nil {
				endpoint.Abort()
				return
			}
			go func() {
				var metadata M.Metadata
				metadata.Source = M.SocksaddrFromNet(lAddr)
				metadata.Destination = M.SocksaddrFromNet(rAddr)
				ctx, conn := canceler.NewPacketConn(t.ctx, bufio.NewPacketConn(&bufio.UnbindPacketConn{ExtendedConn: bufio.NewExtendedConn(&gUDPConn{udpConn}), Addr: M.SocksaddrFromNet(rAddr)}), time.Duration(t.udpTimeout)*time.Second)
				hErr := t.handler.NewPacketConnection(ctx, conn, metadata)
				if hErr != nil {
					endpoint.Abort()
				}
			}()
		})
		ipStack.SetTransportProtocolHandler(udp.ProtocolNumber, func(id stack.TransportEndpointID, buffer *stack.PacketBuffer) bool {
			if t.router != nil {
				var routeSession RouteSession
				routeSession.Network = syscall.IPPROTO_UDP
				var ipHdr header.Network
				if buffer.NetworkProtocolNumber == header.IPv4ProtocolNumber {
					routeSession.IPVersion = 4
					ipHdr = header.IPv4(buffer.NetworkHeader().Slice())
				} else {
					routeSession.IPVersion = 6
					ipHdr = header.IPv6(buffer.NetworkHeader().Slice())
				}
				udpHdr := header.UDP(buffer.TransportHeader().Slice())
				routeSession.Source = M.AddrPortFrom(net.IP(ipHdr.SourceAddress()), udpHdr.SourcePort())
				routeSession.Destination = M.AddrPortFrom(net.IP(ipHdr.DestinationAddress()), udpHdr.DestinationPort())
				action := t.routeMapping.Lookup(routeSession, func() RouteAction {
					if routeSession.IPVersion == 4 {
						return t.router.RouteConnection(routeSession, &systemUDPDirectPacketWriter4{t.tun, routeSession.Source})
					} else {
						return t.router.RouteConnection(routeSession, &systemUDPDirectPacketWriter6{t.tun, routeSession.Source})
					}
				})
				switch actionType := action.(type) {
				case *ActionReject:
					// TODO: send icmp unreachable
					return true
				case *ActionDirect:
					buffer.IncRef()
					err = actionType.WritePacketBuffer(buffer)
					if err != nil {
						t.logger.Trace("route gvisor udp packet: ", err)
					}
					return true
				}
			}
			return udpForwarder.HandlePacket(id, buffer)
		})
	} else {
		ipStack.SetTransportProtocolHandler(udp.ProtocolNumber, NewUDPForwarder(t.ctx, ipStack, t.handler, t.udpTimeout).HandlePacket)
	}

	t.stack = ipStack
	t.endpoint = linkEndpoint
	return nil
}

func (t *GVisor) Close() error {
	t.endpoint.Attach(nil)
	t.stack.Close()
	for _, endpoint := range t.stack.CleanupEndpoints() {
		endpoint.Abort()
	}
	return nil
}
