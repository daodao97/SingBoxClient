//go:build (darwin || freebsd || windows) && go1.20

package tfo

import (
	"context"
	"syscall"
)

func (d *Dialer) ctrlCtxFn() func(ctx context.Context, network string, address string, c syscall.RawConn) error {
	ctrlCtxFn := d.ControlContext
	if ctrlCtxFn == nil && d.Control != nil {
		ctrlCtxFn = func(ctx context.Context, network, address string, c syscall.RawConn) error {
			return d.Control(network, address, c)
		}
	}
	return ctrlCtxFn
}
