package tun

import (
	"context"
	"net/netip"
	"sync"
	"time"
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
	LastActive  time.Time
}

func NewNat(ctx context.Context, timeout time.Duration) *TCPNat {
	natMap := &TCPNat{
		portIndex: 10000,
		addrMap:   make(map[netip.AddrPort]uint16),
		portMap:   make(map[uint16]*TCPSession),
	}
	go natMap.loopCheckTimeout(ctx, timeout)
	return natMap
}

func (n *TCPNat) loopCheckTimeout(ctx context.Context, timeout time.Duration) {
	ticker := time.NewTicker(timeout)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			n.checkTimeout(timeout)
		case <-ctx.Done():
			return
		}
	}
}

func (n *TCPNat) checkTimeout(timeout time.Duration) {
	now := time.Now()
	n.portAccess.Lock()
	defer n.portAccess.Unlock()
	n.addrAccess.Lock()
	defer n.addrAccess.Unlock()
	for natPort, session := range n.portMap {
		if now.Sub(session.LastActive) > timeout {
			delete(n.addrMap, session.Source)
			delete(n.portMap, natPort)
		}
	}
}

func (n *TCPNat) LookupBack(port uint16) *TCPSession {
	n.portAccess.RLock()
	session := n.portMap[port]
	n.portAccess.RUnlock()
	if session != nil {
		session.LastActive = time.Now()
	}
	return session
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
		LastActive:  time.Now(),
	}
	n.portAccess.Unlock()
	return nextPort
}
