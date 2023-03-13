//go:build windows

package quic

import (
	"errors"
	"syscall"

	"golang.org/x/sys/windows"
)

const (
	// same for both IPv4 and IPv6 on Windows
	// https://microsoft.github.io/windows-docs-rs/doc/windows/Win32/Networking/WinSock/constant.IP_DONTFRAGMENT.html
	// https://microsoft.github.io/windows-docs-rs/doc/windows/Win32/Networking/WinSock/constant.IPV6_DONTFRAG.html
	IP_DONTFRAGMENT = 14
	IPV6_DONTFRAG   = 14
)

func setDF(rawConn syscall.RawConn) error {
	rawConn.Control(func(fd uintptr) {
		_ = windows.SetsockoptInt(windows.Handle(fd), windows.IPPROTO_IP, IP_DONTFRAGMENT, 1)
		_ = windows.SetsockoptInt(windows.Handle(fd), windows.IPPROTO_IPV6, IPV6_DONTFRAG, 1)
	})
	return nil
}

func isMsgSizeErr(err error) bool {
	// https://docs.microsoft.com/en-us/windows/win32/winsock/windows-sockets-error-codes-2
	return errors.Is(err, windows.WSAEMSGSIZE)
}
