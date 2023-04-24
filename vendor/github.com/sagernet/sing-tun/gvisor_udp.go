//go:build with_gvisor

package tun

import (
	"context"
	"math"
	"net"
	"net/netip"

	"github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/udpnat"

	"gvisor.dev/gvisor/pkg/bufferv2"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/checksum"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

type UDPForwarder struct {
	ctx    context.Context
	stack  *stack.Stack
	udpNat *udpnat.Service[netip.AddrPort]
}

func NewUDPForwarder(ctx context.Context, stack *stack.Stack, handler Handler, udpTimeout int64) *UDPForwarder {
	return &UDPForwarder{
		ctx:    ctx,
		stack:  stack,
		udpNat: udpnat.New[netip.AddrPort](udpTimeout, handler),
	}
}

func (f *UDPForwarder) HandlePacket(id stack.TransportEndpointID, pkt stack.PacketBufferPtr) bool {
	var upstreamMetadata M.Metadata
	upstreamMetadata.Source = M.SocksaddrFrom(M.AddrFromIP(net.IP(id.RemoteAddress)), id.RemotePort)
	upstreamMetadata.Destination = M.SocksaddrFrom(M.AddrFromIP(net.IP(id.LocalAddress)), id.LocalPort)
	var netProto tcpip.NetworkProtocolNumber
	if upstreamMetadata.Source.IsIPv4() {
		netProto = header.IPv4ProtocolNumber
	} else {
		netProto = header.IPv6ProtocolNumber
	}
	f.udpNat.NewPacket(
		f.ctx,
		upstreamMetadata.Source.AddrPort(),
		buf.As(pkt.Data().AsRange().ToSlice()),
		upstreamMetadata,
		func(natConn N.PacketConn) N.PacketWriter {
			return &UDPBackWriter{f.stack, id.RemoteAddress, id.RemotePort, netProto}
		},
	)
	return true
}

type UDPBackWriter struct {
	stack         *stack.Stack
	source        tcpip.Address
	sourcePort    uint16
	sourceNetwork tcpip.NetworkProtocolNumber
}

func (w *UDPBackWriter) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	defer buffer.Release()

	route, err := w.stack.FindRoute(
		defaultNIC,
		tcpip.Address(destination.Addr.AsSlice()),
		w.source,
		w.sourceNetwork,
		false,
	)
	if err != nil {
		return wrapStackError(err)
	}
	defer route.Release()

	packet := stack.NewPacketBuffer(stack.PacketBufferOptions{
		ReserveHeaderBytes: header.UDPMinimumSize + int(route.MaxHeaderLength()),
		Payload:            bufferv2.MakeWithData(buffer.Bytes()),
	})
	defer packet.DecRef()

	packet.TransportProtocolNumber = header.UDPProtocolNumber
	udpHdr := header.UDP(packet.TransportHeader().Push(header.UDPMinimumSize))
	pLen := uint16(packet.Size())
	udpHdr.Encode(&header.UDPFields{
		SrcPort: destination.Port,
		DstPort: w.sourcePort,
		Length:  pLen,
	})

	if route.RequiresTXTransportChecksum() && w.sourceNetwork == header.IPv6ProtocolNumber {
		xsum := udpHdr.CalculateChecksum(checksum.Combine(
			route.PseudoHeaderChecksum(header.UDPProtocolNumber, pLen),
			packet.Data().Checksum(),
		))
		if xsum != math.MaxUint16 {
			xsum = ^xsum
		}
		udpHdr.SetChecksum(xsum)
	}

	err = route.WritePacket(stack.NetworkHeaderParams{
		Protocol: header.UDPProtocolNumber,
		TTL:      route.DefaultTTL(),
		TOS:      0,
	}, packet)

	if err != nil {
		route.Stats().UDP.PacketSendErrors.Increment()
		return wrapStackError(err)
	}

	route.Stats().UDP.PacketsSent.Increment()
	return nil
}
