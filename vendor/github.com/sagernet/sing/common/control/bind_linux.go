package control

import (
	"syscall"

	"golang.org/x/sys/unix"
)

func bindToInterface(conn syscall.RawConn, network string, address string, interfaceName string, interfaceIndex int) error {
	return Raw(conn, func(fd uintptr) error {
		return unix.BindToDevice(int(fd), interfaceName)
	})
}
