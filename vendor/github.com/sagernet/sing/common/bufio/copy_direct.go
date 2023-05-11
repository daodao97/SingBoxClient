package bufio

import (
	"syscall"

	N "github.com/sagernet/sing/common/network"
)

func CopyDirect(source syscall.Conn, destination syscall.Conn, readCounters []N.CountFunc, writeCounters []N.CountFunc) (handed bool, n int64, err error) {
	rawSource, err := source.SyscallConn()
	if err != nil {
		return
	}
	rawDestination, err := destination.SyscallConn()
	if err != nil {
		return
	}
	handed, n, err = splice(rawSource, rawDestination, readCounters, writeCounters)
	return
}
