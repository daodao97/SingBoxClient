package control

import (
	"syscall"

	"golang.org/x/sys/unix"
)

func bindToInterface(conn syscall.RawConn, network string, address string, interfaceName string, interfaceIndex int) error {
	if interfaceIndex == -1 {
		return nil
	}
	return Raw(conn, func(fd uintptr) error {
		switch network {
		case "tcp6", "udp6":
			return unix.SetsockoptInt(int(fd), unix.IPPROTO_IPV6, unix.IPV6_BOUND_IF, interfaceIndex)
		default:
			return unix.SetsockoptInt(int(fd), unix.IPPROTO_IP, unix.IP_BOUND_IF, interfaceIndex)
		}
	})
}
