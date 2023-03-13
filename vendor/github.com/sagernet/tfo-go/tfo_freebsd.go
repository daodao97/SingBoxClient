package tfo

import (
	"syscall"

	"golang.org/x/sys/unix"
)

func SetTFOListener(fd uintptr) error {
	return setTFO(fd)
}

func SetTFODialer(fd uintptr) error {
	return setTFO(fd)
}

func setTFO(fd uintptr) error {
	return unix.SetsockoptInt(int(fd), unix.IPPROTO_TCP, unix.TCP_FASTOPEN, 1)
}

func socket(domain int) (int, error) {
	return unix.Socket(domain, unix.SOCK_STREAM|unix.SOCK_NONBLOCK|unix.SOCK_CLOEXEC, unix.IPPROTO_TCP)
}

func connect(rawConn syscall.RawConn, rsa syscall.Sockaddr, b []byte) (n int, err error) {
	var done bool

	if perr := rawConn.Write(func(fd uintptr) bool {
		if done {
			return true
		}

		n, err = syscall.SendmsgN(int(fd), b, nil, rsa, 0)
		switch err {
		case unix.EINPROGRESS:
			done = true
			err = nil
			return false
		case unix.EAGAIN:
			return false
		default:
			return true
		}
	}); perr != nil {
		return 0, perr
	}

	if err != nil {
		return 0, wrapSyscallError("sendmsg", err)
	}

	if perr := rawConn.Control(func(fd uintptr) {
		err = getSocketError(int(fd), "sendmsg")
	}); perr != nil {
		return 0, perr
	}

	return
}
