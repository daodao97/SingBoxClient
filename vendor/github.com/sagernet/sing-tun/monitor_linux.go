package tun

import (
	"os"
	"sync"

	"github.com/sagernet/netlink"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/x/list"
)

type networkUpdateMonitor struct {
	routeUpdate  chan netlink.RouteUpdate
	linkUpdate   chan netlink.LinkUpdate
	close        chan struct{}
	errorHandler E.Handler

	access    sync.Mutex
	callbacks list.List[NetworkUpdateCallback]
}

func NewNetworkUpdateMonitor(errorHandler E.Handler) (NetworkUpdateMonitor, error) {
	return &networkUpdateMonitor{
		routeUpdate:  make(chan netlink.RouteUpdate, 2),
		linkUpdate:   make(chan netlink.LinkUpdate, 2),
		close:        make(chan struct{}),
		errorHandler: errorHandler,
	}, nil
}

func (m *networkUpdateMonitor) Start() error {
	err := netlink.RouteSubscribe(m.routeUpdate, m.close)
	if err != nil {
		return err
	}
	err = netlink.LinkSubscribe(m.linkUpdate, m.close)
	if err != nil {
		return err
	}
	go m.loopUpdate()
	return nil
}

func (m *networkUpdateMonitor) loopUpdate() {
	for {
		select {
		case <-m.close:
			return
		case <-m.routeUpdate:
		case <-m.linkUpdate:
		}
		m.emit()
	}
}

func (m *networkUpdateMonitor) Close() error {
	select {
	case <-m.close:
		return os.ErrClosed
	default:
	}
	close(m.close)
	return nil
}
