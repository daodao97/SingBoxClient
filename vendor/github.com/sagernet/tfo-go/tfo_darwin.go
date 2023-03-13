package tfo

import (
	"context"
	"net"
	"syscall"

	"golang.org/x/sys/unix"
)

func SetTFOListener(fd uintptr) error {
	return unix.SetsockoptInt(int(fd), unix.IPPROTO_TCP, unix.TCP_FASTOPEN, 1)
}

func (lc *ListenConfig) listenTFO(ctx context.Context, network, address string) (net.Listener, error) {
	// darwin requires setting TCP_FASTOPEN after bind() and listen() calls.
	ln, err := lc.ListenConfig.Listen(ctx, network, address)
	if err != nil {
		return nil, err
	}

	rawConn, err := ln.(*net.TCPListener).SyscallConn()
	if err != nil {
		ln.Close()
		return nil, err
	}

	var innerErr error

	if err = rawConn.Control(func(fd uintptr) {
		innerErr = SetTFOListener(fd)
	}); err != nil {
		ln.Close()
		return nil, err
	}

	if innerErr != nil {
		ln.Close()
		return nil, innerErr
	}

	return ln, nil
}

func SetTFODialer(fd uintptr) error {
	return nil
}

func socket(domain int) (fd int, err error) {
	fd, err = unix.Socket(domain, unix.SOCK_STREAM, unix.IPPROTO_TCP)
	if err != nil {
		return
	}
	unix.CloseOnExec(fd)
	err = unix.SetNonblock(fd, true)
	if err != nil {
		unix.Close(fd)
		fd = 0
	}
	return
}

func connect(rawConn syscall.RawConn, rsa syscall.Sockaddr, b []byte) (n int, err error) {
	var done bool

	if perr := rawConn.Write(func(fd uintptr) bool {
		if done {
			return true
		}

		bytesSent, err := Connectx(int(fd), 0, nil, rsa, b)
		n = int(bytesSent)
		done = true
		if err == unix.EINPROGRESS {
			err = nil
			return false
		}
		return true
	}); perr != nil {
		return 0, perr
	}

	if err != nil {
		return 0, wrapSyscallError("connectx", err)
	}

	if perr := rawConn.Control(func(fd uintptr) {
		err = getSocketError(int(fd), "connectx")
	}); perr != nil {
		return 0, perr
	}

	return
}
