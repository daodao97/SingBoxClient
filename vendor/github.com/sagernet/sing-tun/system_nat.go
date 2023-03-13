package tun

import (
	"net/netip"
	"sync"
)

type TCPNat struct {
	portIndex  uint16
	portAccess sync.RWMutex
	addrAccess sync.RWMutex
	addrMap    map[netip.AddrPort]uint16
	portMap    map[uint16]*TCPSession
}

type TCPSession struct {
	Source      netip.AddrPort
	Destination netip.AddrPort
}

func NewNat() *TCPNat {
	return &TCPNat{
		portIndex: 10000,
		addrMap:   make(map[netip.AddrPort]uint16),
		portMap:   make(map[uint16]*TCPSession),
	}
}

func (n *TCPNat) LookupBack(port uint16) *TCPSession {
	n.portAccess.RLock()
	defer n.portAccess.RUnlock()
	return n.portMap[port]
}

func (n *TCPNat) Lookup(source netip.AddrPort, destination netip.AddrPort) uint16 {
	n.addrAccess.RLock()
	port, loaded := n.addrMap[source]
	n.addrAccess.RUnlock()
	if loaded {
		return port
	}
	n.addrAccess.Lock()
	nextPort := n.portIndex
	if nextPort == 0 {
		nextPort = 10000
		n.portIndex = 10001
	} else {
		n.portIndex++
	}
	n.addrMap[source] = nextPort
	n.addrAccess.Unlock()
	n.portAccess.Lock()
	n.portMap[nextPort] = &TCPSession{
		Source:      source,
		Destination: destination,
	}
	n.portAccess.Unlock()
	return nextPort
}

func (n *TCPNat) Revoke(natPort uint16, session *TCPSession) {
	n.addrAccess.Lock()
	delete(n.addrMap, session.Source)
	n.addrAccess.Unlock()
	n.portAccess.Lock()
	delete(n.portMap, natPort)
	n.portAccess.Unlock()
}
