package tfo

import (
	"context"
	"net"
	"syscall"

	"golang.org/x/sys/unix"
)

const TCP_FASTOPEN_FORCE_ENABLE = 0x218

// setTFOForceEnable disables the absolutely brutal TFO backoff mechanism.
func setTFOForceEnable(fd uintptr) error {
	return unix.SetsockoptInt(int(fd), unix.IPPROTO_TCP, TCP_FASTOPEN_FORCE_ENABLE, 1)
}

func SetTFOListener(fd uintptr) error {
	return unix.SetsockoptInt(int(fd), unix.IPPROTO_TCP, unix.TCP_FASTOPEN, 1)
}

func (lc *ListenConfig) listenTFO(ctx context.Context, network, address string) (net.Listener, error) {
	// When setting TCP_FASTOPEN_FORCE_ENABLE, the socket must be in the TCPS_CLOSED state.
	// This means setting it before listen().
	//
	// However, setting TCP_FASTOPEN requires being in the TCPS_LISTEN state,
	// which means setting it after listen().
	llc := *lc
	llc.Control = func(network, address string, c syscall.RawConn) (err error) {
		if lc.Control != nil {
			if err = lc.Control(network, address, c); err != nil {
				return err
			}
		}

		if cerr := c.Control(func(fd uintptr) {
			err = setTFOForceEnable(fd)
		}); cerr != nil {
			return cerr
		}

		if err != nil {
			return wrapSyscallError("setsockopt", err)
		}
		return nil
	}

	ln, err := llc.ListenConfig.Listen(ctx, network, address)
	if err != nil {
		return nil, err
	}

	rawConn, err := ln.(*net.TCPListener).SyscallConn()
	if err != nil {
		ln.Close()
		return nil, err
	}

	if cerr := rawConn.Control(func(fd uintptr) {
		err = SetTFOListener(fd)
	}); cerr != nil {
		ln.Close()
		return nil, cerr
	}

	if err != nil {
		ln.Close()
		return nil, wrapSyscallError("setsockopt", err)
	}

	return ln, nil
}

func SetTFODialer(fd uintptr) error {
	return setTFOForceEnable(fd)
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

const connectSyscallName = "connectx"

func doConnect(fd uintptr, rsa syscall.Sockaddr, b []byte) (int, error) {
	n, err := Connectx(int(fd), 0, nil, rsa, b)
	return int(n), err
}
