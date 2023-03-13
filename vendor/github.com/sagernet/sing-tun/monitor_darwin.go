package tun

import (
	"context"
	"net"
	"net/netip"
	"os"
	"sync"
	"syscall"

	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/x/list"

	"golang.org/x/net/route"
	"golang.org/x/sys/unix"
)

type networkUpdateMonitor struct {
	errorHandler E.Handler

	access      sync.Mutex
	callbacks   list.List[NetworkUpdateCallback]
	routeSocket *os.File
}

func NewNetworkUpdateMonitor(errorHandler E.Handler) (NetworkUpdateMonitor, error) {
	return &networkUpdateMonitor{
		errorHandler: errorHandler,
	}, nil
}

func (m *networkUpdateMonitor) Start() error {
	routeSocket, err := unix.Socket(unix.AF_ROUTE, unix.SOCK_RAW, 0)
	if err != nil {
		return err
	}
	err = unix.SetNonblock(routeSocket, true)
	if err != nil {
		return err
	}
	m.routeSocket = os.NewFile(uintptr(routeSocket), "route")
	go m.loopUpdate()
	return nil
}

func (m *networkUpdateMonitor) loopUpdate() {
	rawConn, err := m.routeSocket.SyscallConn()
	if err != nil {
		m.errorHandler.NewError(context.Background(), E.Cause(err, "create raw route connection"))
		return
	}
	for {
		var innerErr error
		err = rawConn.Read(func(fd uintptr) (done bool) {
			var msg [2048]byte
			_, innerErr = unix.Read(int(fd), msg[:])
			return innerErr != unix.EWOULDBLOCK
		})
		if innerErr != nil {
			err = innerErr
		}
		if err != nil {
			break
		}
		m.emit()
	}
	if err != syscall.EAGAIN {
		m.errorHandler.NewError(context.Background(), E.Cause(err, "read route message"))
	}
}

func (m *networkUpdateMonitor) Close() error {
	return common.Close(common.PtrOrNil(m.routeSocket))
}

func (m *defaultInterfaceMonitor) checkUpdate() error {
	ribMessage, err := route.FetchRIB(unix.AF_UNSPEC, route.RIBTypeRoute, 0)
	if err != nil {
		return err
	}
	routeMessages, err := route.ParseRIB(route.RIBTypeRoute, ribMessage)
	if err != nil {
		return err
	}
	for _, rawRouteMessage := range routeMessages {
		routeMessage := rawRouteMessage.(*route.RouteMessage)
		if common.Any(common.FilterIsInstance(routeMessage.Addrs, func(it route.Addr) (*route.Inet4Addr, bool) {
			addr, loaded := it.(*route.Inet4Addr)
			return addr, loaded
		}), func(addr *route.Inet4Addr) bool {
			return addr.IP == netip.IPv4Unspecified().As4()
		}) {
			oldInterface := m.defaultInterfaceName
			oldIndex := m.defaultInterfaceIndex

			m.defaultInterfaceIndex = routeMessage.Index
			defaultInterface, err := net.InterfaceByIndex(routeMessage.Index)
			if err != nil {
				return err
			}
			m.defaultInterfaceName = defaultInterface.Name
			if oldInterface == m.defaultInterfaceName && oldIndex == m.defaultInterfaceIndex {
				return nil
			}
			m.emit(EventInterfaceUpdate)
			return nil
		}
	}
	return ErrNoRoute
}
