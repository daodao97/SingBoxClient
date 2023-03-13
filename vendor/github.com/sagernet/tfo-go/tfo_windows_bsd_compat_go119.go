//go:build (darwin || freebsd || windows) && !go1.20

package tfo

import (
	"context"
	"syscall"
)

func (d *Dialer) ctrlCtxFn() func(context.Context, string, string, syscall.RawConn) error {
	if d.Control != nil {
		return func(ctx context.Context, network string, address string, c syscall.RawConn) error {
			return d.Control(network, address, c)
		}
	}
	return nil
}
