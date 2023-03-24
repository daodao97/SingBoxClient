package tun

import (
	"net/netip"
	"sync"

	"github.com/sagernet/sing-tun/internal/clashtcpip"
)

type NatMapping struct {
	access    sync.RWMutex
	sessions  map[RouteSession]RouteContext
	ipRewrite bool
}

func NewNatMapping(ipRewrite bool) *NatMapping {
	return &NatMapping{
		sessions:  make(map[RouteSession]RouteContext),
		ipRewrite: ipRewrite,
	}
}

func (m *NatMapping) CreateSession(session RouteSession, context RouteContext) {
	if m.ipRewrite {
		session.Source = netip.AddrPort{}
	}
	m.access.Lock()
	m.sessions[session] = context
	m.access.Unlock()
}

func (m *NatMapping) DeleteSession(session RouteSession) {
	if m.ipRewrite {
		session.Source = netip.AddrPort{}
	}
	m.access.Lock()
	delete(m.sessions, session)
	m.access.Unlock()
}

func (m *NatMapping) WritePacket(packet []byte) (bool, error) {
	var routeSession RouteSession
	var ipHdr clashtcpip.IP
	switch ipVersion := packet[0] >> 4; ipVersion {
	case 4:
		routeSession.IPVersion = 4
		ipHdr = clashtcpip.IPv4Packet(packet)
	case 6:
		routeSession.IPVersion = 6
		ipHdr = clashtcpip.IPv6Packet(packet)
	default:
		return false, nil
	}
	routeSession.Network = ipHdr.Protocol()
	switch routeSession.Network {
	case clashtcpip.TCP:
		tcpHdr := clashtcpip.TCPPacket(ipHdr.Payload())
		routeSession.Destination = netip.AddrPortFrom(ipHdr.SourceIP(), tcpHdr.SourcePort())
		if !m.ipRewrite {
			routeSession.Source = netip.AddrPortFrom(ipHdr.DestinationIP(), tcpHdr.DestinationPort())
		}
	case clashtcpip.UDP:
		udpHdr := clashtcpip.UDPPacket(ipHdr.Payload())
		routeSession.Destination = netip.AddrPortFrom(ipHdr.SourceIP(), udpHdr.SourcePort())
		if !m.ipRewrite {
			routeSession.Source = netip.AddrPortFrom(ipHdr.DestinationIP(), udpHdr.DestinationPort())
		}
	default:
		routeSession.Destination = netip.AddrPortFrom(ipHdr.SourceIP(), 0)
		if !m.ipRewrite {
			routeSession.Source = netip.AddrPortFrom(ipHdr.DestinationIP(), 0)
		}
	}
	m.access.RLock()
	context, loaded := m.sessions[routeSession]
	m.access.RUnlock()
	if !loaded {
		return false, nil
	}
	return true, context.WritePacket(packet)
}

type NatWriter struct {
	inet4Address netip.Addr
	inet6Address netip.Addr
}

func NewNatWriter(inet4Address netip.Addr, inet6Address netip.Addr) *NatWriter {
	return &NatWriter{
		inet4Address: inet4Address,
		inet6Address: inet6Address,
	}
}

func (w *NatWriter) RewritePacket(packet []byte) {
	var ipHdr clashtcpip.IP
	var bindAddr netip.Addr
	switch ipVersion := packet[0] >> 4; ipVersion {
	case 4:
		ipHdr = clashtcpip.IPv4Packet(packet)
		bindAddr = w.inet4Address
	case 6:
		ipHdr = clashtcpip.IPv6Packet(packet)
		bindAddr = w.inet6Address
	default:
		return
	}
	ipHdr.SetSourceIP(bindAddr)
	switch ipHdr.Protocol() {
	case clashtcpip.TCP:
		tcpHdr := clashtcpip.TCPPacket(ipHdr.Payload())
		tcpHdr.ResetChecksum(ipHdr.PseudoSum())
	case clashtcpip.UDP:
		udpHdr := clashtcpip.UDPPacket(ipHdr.Payload())
		udpHdr.ResetChecksum(ipHdr.PseudoSum())
	default:
	}
	ipHdr.ResetChecksum()
}
