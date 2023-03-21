package control

import (
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

func DisableUDPFragment() Func {
	return func(network, address string, conn syscall.RawConn) error {
		return Raw(conn, func(fd uintptr) error {
			switch network {
			case "udp4":
				if err := unix.SetsockoptInt(int(fd), unix.IPPROTO_IP, unix.IP_DONTFRAG, 1); err != nil {
					return os.NewSyscallError("SETSOCKOPT IP_DONTFRAG", err)
				}
			case "udp6":
				if err := unix.SetsockoptInt(int(fd), unix.IPPROTO_IPV6, unix.IPV6_DONTFRAG, 1); err != nil {
					return os.NewSyscallError("SETSOCKOPT IPV6_DONTFRAG", err)
				}
			}
			return nil
		})
	}
}
