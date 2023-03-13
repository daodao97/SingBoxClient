package tun

import (
	"net/netip"

	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/x/list"
)

var ErrNoRoute = E.New("no route to internet")

type (
	NetworkUpdateCallback          = func() error
	DefaultInterfaceUpdateCallback = func(event int) error
)

const (
	EventInterfaceUpdate  = 1
	EventAndroidVPNUpdate = 2
)

type NetworkUpdateMonitor interface {
	Start() error
	Close() error
	RegisterCallback(callback NetworkUpdateCallback) *list.Element[NetworkUpdateCallback]
	UnregisterCallback(element *list.Element[NetworkUpdateCallback])
	E.Handler
}

type DefaultInterfaceMonitor interface {
	Start() error
	Close() error
	DefaultInterfaceName(destination netip.Addr) string
	DefaultInterfaceIndex(destination netip.Addr) int
	OverrideAndroidVPN() bool
	AndroidVPNEnabled() bool
	RegisterCallback(callback DefaultInterfaceUpdateCallback) *list.Element[DefaultInterfaceUpdateCallback]
	UnregisterCallback(element *list.Element[DefaultInterfaceUpdateCallback])
}

type DefaultInterfaceMonitorOptions struct {
	OverrideAndroidVPN bool
}
