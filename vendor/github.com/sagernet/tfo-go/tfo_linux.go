package tfo

import (
	"golang.org/x/sys/unix"
)

// TCPFastopenQueueLength sets the maximum number of total pending TFO connection requests.
// ref: https://datatracker.ietf.org/doc/html/rfc7413#section-5.1
// We default to 4096 to align with listener's default backlog.
// Change to a lower value if your application is vulnerable to such attacks.
const TCPFastopenQueueLength = 4096

func SetTFOListener(fd uintptr) error {
	return unix.SetsockoptInt(int(fd), unix.IPPROTO_TCP, unix.TCP_FASTOPEN, TCPFastopenQueueLength)
}

func SetTFODialer(fd uintptr) error {
	return unix.SetsockoptInt(int(fd), unix.IPPROTO_TCP, unix.TCP_FASTOPEN_CONNECT, 1)
}
