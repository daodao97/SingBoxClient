//go:build darwin || freebsd

package tfo

import (
	"context"
	"net"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

func setIPv6Only(fd int, family int, ipv6only bool) error {
	if family == unix.AF_INET6 {
		// Allow both IP versions even if the OS default
		// is otherwise. Note that some operating systems
		// never admit this option.
		return unix.SetsockoptInt(fd, unix.IPPROTO_IPV6, unix.IPV6_V6ONLY, boolint(ipv6only))
	}
	return nil
}

func setNoDelay(fd int, noDelay int) error {
	return unix.SetsockoptInt(fd, unix.IPPROTO_TCP, unix.TCP_NODELAY, noDelay)
}

func ctrlNetwork(network string, family int) string {
	if network == "tcp4" || family == unix.AF_INET {
		return "tcp4"
	}
	return "tcp6"
}

func dialTFO(ctx context.Context, network string, laddr, raddr *net.TCPAddr, b []byte, ctrlCtxFn func(context.Context, string, string, syscall.RawConn) error) (*net.TCPConn, error) {
	ltsa := (*tcpSockaddr)(laddr)
	rtsa := (*tcpSockaddr)(raddr)
	family, ipv6only := favoriteAddrFamily(network, ltsa, rtsa, "dial")

	lsa, err := ltsa.sockaddr(family)
	if err != nil {
		return nil, err
	}

	rsa, err := rtsa.sockaddr(family)
	if err != nil {
		return nil, err
	}

	fd, err := socket(family)
	if err != nil {
		return nil, wrapSyscallError("socket", err)
	}

	if err := setIPv6Only(fd, family, ipv6only); err != nil {
		unix.Close(fd)
		return nil, wrapSyscallError("setsockopt", err)
	}

	if err := setNoDelay(fd, 1); err != nil {
		unix.Close(fd)
		return nil, wrapSyscallError("setsockopt", err)
	}

	if err := SetTFODialer(uintptr(fd)); err != nil {
		unix.Close(fd)
		return nil, wrapSyscallError("setsockopt", err)
	}

	f := os.NewFile(uintptr(fd), "")
	if err != nil {
		unix.Close(fd)
		return nil, err
	}
	defer f.Close()

	rawConn, err := f.SyscallConn()
	if err != nil {
		return nil, err
	}

	if ctrlCtxFn != nil {
		if err := ctrlCtxFn(ctx, ctrlNetwork(network, family), raddr.String(), rawConn); err != nil {
			return nil, err
		}
	}

	if laddr != nil {
		if cErr := rawConn.Control(func(fd uintptr) {
			err = syscall.Bind(int(fd), lsa)
		}); cErr != nil {
			return nil, cErr
		}
		if err != nil {
			return nil, wrapSyscallError("bind", err)
		}
	}

	n, err := connect(rawConn, rsa, b)
	if err != nil {
		return nil, err
	}

	c, err := net.FileConn(f)
	if err != nil {
		return nil, err
	}
	tcpConn := c.(*net.TCPConn)

	if n < len(b) {
		if _, err := tcpConn.Write(b[n:]); err != nil {
			tcpConn.Close()
			return nil, err
		}
	}

	return tcpConn, nil
}

func getSocketError(fd int, call string) error {
	nerr, err := unix.GetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_ERROR)
	if err != nil {
		return wrapSyscallError("getsockopt", err)
	}

	switch err := syscall.Errno(nerr); err {
	case unix.EINPROGRESS, unix.EALREADY, unix.EINTR, unix.EISCONN, syscall.Errno(0):
		return nil
	default:
		return os.NewSyscallError(call, err)
	}
}
