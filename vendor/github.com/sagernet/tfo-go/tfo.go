// Package tfo provides TCP Fast Open support for the [net] dialer and listener.
//
// The dial functions have an additional buffer parameter, which specifies data in SYN.
// If the buffer is empty, TFO is not used.
//
// This package supports Linux, Windows, macOS, and FreeBSD.
// On unsupported platforms, [ErrPlatformUnsupported] is returned.
//
// FreeBSD code is completely untested. Use at your own risk. Feedback is welcome.
package tfo

import (
	"context"
	"errors"
	"net"
	"os"
	"syscall"
	"time"
)

var (
	ErrPlatformUnsupported = errors.New("tfo-go does not support TCP Fast Open on this platform")
	errMissingAddress      = errors.New("missing address")
)

// ListenConfig wraps [net.ListenConfig] with an additional option that allows you to disable TFO.
type ListenConfig struct {
	net.ListenConfig

	// DisableTFO controls whether TCP Fast Open is disabled when the Listen method is called.
	// TFO is enabled by default.
	// Set to true to disable TFO and it will behave exactly the same as [net.ListenConfig].
	DisableTFO bool
}

// Listen is like [net.ListenConfig.Listen] but enables TFO whenever possible,
// unless [ListenConfig.DisableTFO] is set to true.
func (lc *ListenConfig) Listen(ctx context.Context, network, address string) (net.Listener, error) {
	if lc.DisableTFO || network != "tcp" && network != "tcp4" && network != "tcp6" {
		return lc.ListenConfig.Listen(ctx, network, address)
	}
	return lc.listenTFO(ctx, network, address) // tfo_darwin.go, tfo_notdarwin.go
}

// ListenContext is like [net.ListenContext] but enables TFO whenever possible.
func ListenContext(ctx context.Context, network, address string) (net.Listener, error) {
	var lc ListenConfig
	return lc.Listen(ctx, network, address)
}

// Listen is like [net.Listen] but enables TFO whenever possible.
func Listen(network, address string) (net.Listener, error) {
	return ListenContext(context.Background(), network, address)
}

// ListenTCP is like [net.ListenTCP] but enables TFO whenever possible.
func ListenTCP(network string, laddr *net.TCPAddr) (*net.TCPListener, error) {
	switch network {
	case "tcp", "tcp4", "tcp6":
	default:
		return nil, &net.OpError{Op: "listen", Net: network, Source: nil, Addr: opAddr(laddr), Err: net.UnknownNetworkError(network)}
	}
	var address string
	if laddr != nil {
		address = laddr.String()
	}
	var lc ListenConfig
	ln, err := lc.listenTFO(context.Background(), network, address) // tfo_darwin.go, tfo_notdarwin.go
	if err != nil && err != ErrPlatformUnsupported {
		return nil, err
	}
	return ln.(*net.TCPListener), err
}

// Dialer wraps [net.Dialer] with an additional option that allows you to disable TFO.
type Dialer struct {
	net.Dialer

	// DisableTFO controls whether TCP Fast Open is disabled when the dial methods are called.
	// TFO is enabled by default.
	// Set to true to disable TFO and it will behave exactly the same as [net.Dialer].
	DisableTFO bool
}

// DialContext is like [net.Dialer.DialContext] but enables TFO whenever possible,
// unless [Dialer.DisableTFO] is set to true.
func (d *Dialer) DialContext(ctx context.Context, network, address string, b []byte) (net.Conn, error) {
	if len(b) == 0 {
		return d.Dialer.DialContext(ctx, network, address)
	}
	if d.DisableTFO || network != "tcp" && network != "tcp4" && network != "tcp6" {
		c, err := d.Dialer.DialContext(ctx, network, address)
		if err != nil {
			return nil, err
		}
		if _, err = c.Write(b); err != nil {
			c.Close()
			return nil, err
		}
		return c, nil
	}
	return d.dialTFOContext(ctx, network, address, b) // tfo_linux.go, tfo_windows_bsd.go, tfo_fallback.go
}

// Dial is like [net.Dialer.Dial] but enables TFO whenever possible,
// unless [Dialer.DisableTFO] is set to true.
func (d *Dialer) Dial(network, address string, b []byte) (net.Conn, error) {
	return d.DialContext(context.Background(), network, address, b)
}

// Dial is like [net.Dial] but enables TFO whenever possible.
func Dial(network, address string, b []byte) (net.Conn, error) {
	var d Dialer
	return d.DialContext(context.Background(), network, address, b)
}

// DialTimeout is like [net.DialTimeout] but enables TFO whenever possible.
func DialTimeout(network, address string, timeout time.Duration, b []byte) (net.Conn, error) {
	var d Dialer
	d.Timeout = timeout
	return d.DialContext(context.Background(), network, address, b)
}

// DialTCP is like [net.DialTCP] but enables TFO whenever possible.
func DialTCP(network string, laddr, raddr *net.TCPAddr, b []byte) (*net.TCPConn, error) {
	if len(b) == 0 {
		return net.DialTCP(network, laddr, raddr)
	}
	switch network {
	case "tcp", "tcp4", "tcp6":
	default:
		return nil, &net.OpError{Op: "dial", Net: network, Source: opAddr(laddr), Addr: opAddr(raddr), Err: net.UnknownNetworkError(network)}
	}
	if raddr == nil {
		return nil, &net.OpError{Op: "dial", Net: network, Source: opAddr(laddr), Addr: nil, Err: errMissingAddress}
	}
	return dialTFO(context.Background(), network, laddr, raddr, b, nil) // tfo_linux.go, tfo_windows.go, tfo_darwin.go, tfo_fallback.go
}

func opAddr(a *net.TCPAddr) net.Addr {
	if a == nil {
		return nil
	}
	return a
}

// wrapSyscallError takes an error and a syscall name. If the error is
// a syscall.Errno, it wraps it in a os.SyscallError using the syscall name.
func wrapSyscallError(name string, err error) error {
	if _, ok := err.(syscall.Errno); ok {
		err = os.NewSyscallError(name, err)
	}
	return err
}
