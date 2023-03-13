package tun

import (
	"io"
	"net"
	"net/netip"
	"runtime"
	"strconv"
	"strings"

	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/ranges"
)

type Handler interface {
	N.TCPConnectionHandler
	N.UDPConnectionHandler
	E.Handler
}

type Tun interface {
	io.ReadWriter
	Close() error
}

type WinTun interface {
	Tun
	ReadPacket() ([]byte, func(), error)
}

type Options struct {
	Name               string
	Inet4Address       []netip.Prefix
	Inet6Address       []netip.Prefix
	MTU                uint32
	AutoRoute          bool
	StrictRoute        bool
	Inet4RouteAddress  []netip.Prefix
	Inet6RouteAddress  []netip.Prefix
	IncludeUID         []ranges.Range[uint32]
	ExcludeUID         []ranges.Range[uint32]
	IncludeAndroidUser []int
	IncludePackage     []string
	ExcludePackage     []string
	InterfaceMonitor   DefaultInterfaceMonitor
	TableIndex         int
}

func CalculateInterfaceName(name string) (tunName string) {
	if runtime.GOOS == "darwin" {
		tunName = "utun"
	} else if name != "" {
		tunName = name
	} else {
		tunName = "tun"
	}
	interfaces, err := net.Interfaces()
	if err != nil {
		return
	}
	var tunIndex int
	for _, netInterface := range interfaces {
		if strings.HasPrefix(netInterface.Name, tunName) {
			index, parseErr := strconv.ParseInt(netInterface.Name[len(tunName):], 10, 16)
			if parseErr == nil {
				tunIndex = int(index) + 1
			}
		}
	}
	tunName = F.ToString(tunName, tunIndex)
	return
}
