//go:build !darwin

package tfo

import (
	"context"
	"net"
	"syscall"
)

func (lc *ListenConfig) listenTFO(ctx context.Context, network, address string) (net.Listener, error) {
	llc := *lc
	llc.Control = func(network, address string, c syscall.RawConn) (err error) {
		if lc.Control != nil {
			if err = lc.Control(network, address, c); err != nil {
				return
			}
		}
		if cerr := c.Control(func(fd uintptr) {
			err = SetTFOListener(fd)
		}); cerr != nil {
			return cerr
		}
		return
	}
	return llc.ListenConfig.Listen(ctx, network, address)
}
