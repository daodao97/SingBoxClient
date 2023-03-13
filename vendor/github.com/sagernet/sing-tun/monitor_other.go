//go:build !(linux || windows || darwin)

package tun

import (
	"os"

	E "github.com/sagernet/sing/common/exceptions"
)

func NewNetworkUpdateMonitor(errorHandler E.Handler) (NetworkUpdateMonitor, error) {
	return nil, os.ErrInvalid
}

func NewDefaultInterfaceMonitor(networkMonitor NetworkUpdateMonitor, options DefaultInterfaceMonitorOptions) (DefaultInterfaceMonitor, error) {
	return nil, os.ErrInvalid
}
