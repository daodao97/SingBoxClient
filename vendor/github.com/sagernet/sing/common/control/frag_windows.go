package control

import (
	"os"
	"syscall"

	N "github.com/sagernet/sing/common/network"

	"golang.org/x/sys/windows"
)

const (
	IP_MTU_DISCOVER   = 71
	IPV6_MTU_DISCOVER = 71
)

// enum PMTUD_STATE from ws2ipdef.h
const (
	IP_PMTUDISC_NOT_SET = iota
	IP_PMTUDISC_DO
	IP_PMTUDISC_DONT
	IP_PMTUDISC_PROBE
	IP_PMTUDISC_MAX
)

func DisableUDPFragment() Func {
	return func(network, address string, conn syscall.RawConn) error {
		switch N.NetworkName(network) {
		case N.NetworkUDP:
		default:
			return nil
		}
		return Raw(conn, func(fd uintptr) error {
			if err := windows.SetsockoptInt(windows.Handle(fd), windows.IPPROTO_IP, IP_MTU_DISCOVER, IP_PMTUDISC_DO); err != nil {
				return os.NewSyscallError("SETSOCKOPT IP_MTU_DISCOVER IP_PMTUDISC_DO", err)
			}
			if network == "udp6" {
				if err := windows.SetsockoptInt(windows.Handle(fd), windows.IPPROTO_IPV6, IPV6_MTU_DISCOVER, IP_PMTUDISC_DO); err != nil {
					return os.NewSyscallError("SETSOCKOPT IPV6_MTU_DISCOVER IP_PMTUDISC_DO", err)
				}
			}
			return nil
		})
	}
}
