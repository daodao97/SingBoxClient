//go:build linux
// +build linux

package quic

import (
	"errors"
	"fmt"
	"syscall"

	"golang.org/x/sys/unix"
)

func setConnReadBufferForce(c interface{}, size int) error {
	conn, ok := c.(interface {
		SyscallConn() (syscall.RawConn, error)
	})
	if !ok {
		return errors.New("doesn't have a SyscallConn")
	}
	rawConn, err := conn.SyscallConn()
	if err != nil {
		return fmt.Errorf("couldn't get syscall.RawConn: %w", err)
	}
	var serr error
	if err := rawConn.Control(func(fd uintptr) {
		serr = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_RCVBUFFORCE, size)
	}); err != nil {
		return err
	}
	return serr
}
