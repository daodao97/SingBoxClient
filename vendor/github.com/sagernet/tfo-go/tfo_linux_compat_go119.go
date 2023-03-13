//go:build linux && !go1.20

package tfo

import (
	"context"
	"net"
	"syscall"
)

func (d *Dialer) dialTFOContext(ctx context.Context, network, address string, b []byte) (net.Conn, error) {
	ld := *d
	ld.Control = func(network, address string, c syscall.RawConn) (err error) {
		if d.Control != nil {
			if err = d.Control(network, address, c); err != nil {
				return
			}
		}
		if cerr := c.Control(func(fd uintptr) {
			err = SetTFODialer(fd)
		}); cerr != nil {
			return cerr
		}
		return
	}
	c, err := ld.Dialer.DialContext(ctx, network, address)
	if err != nil {
		return nil, err
	}
	if _, err = c.Write(b); err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}

func dialTFO(ctx context.Context, network string, laddr, raddr *net.TCPAddr, b []byte, ctrlCtxFn func(context.Context, string, string, syscall.RawConn) error) (*net.TCPConn, error) {
	d := Dialer{Dialer: net.Dialer{LocalAddr: laddr, Control: func(network, address string, c syscall.RawConn) error {
		return ctrlCtxFn(context.Background(), network, address, c)
	}}}
	c, err := d.dialTFOContext(ctx, network, raddr.String(), b)
	if err != nil {
		return nil, err
	}
	return c.(*net.TCPConn), nil
}
