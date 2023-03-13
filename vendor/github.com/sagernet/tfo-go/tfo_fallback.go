//go:build !darwin && !freebsd && !linux && !windows

package tfo

import (
	"context"
	"net"
	"syscall"
)

func SetTFOListener(fd uintptr) error {
	return ErrPlatformUnsupported
}

func SetTFODialer(fd uintptr) error {
	return ErrPlatformUnsupported
}

func (d *Dialer) dialTFOContext(ctx context.Context, network, address string, b []byte) (net.Conn, error) {
	return nil, ErrPlatformUnsupported
}

func dialTFO(ctx context.Context, network string, laddr, raddr *net.TCPAddr, b []byte, ctrlCtxFn func(context.Context, string, string, syscall.RawConn) error) (*net.TCPConn, error) {
	return nil, ErrPlatformUnsupported
}
