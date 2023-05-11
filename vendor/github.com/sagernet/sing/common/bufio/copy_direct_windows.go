package bufio

import (
	"io"

	N "github.com/sagernet/sing/common/network"
)

func copyWaitWithPool(originDestination io.Writer, destination N.ExtendedWriter, source N.ReadWaiter, readCounters []N.CountFunc, writeCounters []N.CountFunc) (handled bool, n int64, err error) {
	return
}

func copyPacketWaitWithPool(destinationConn N.PacketWriter, source N.PacketReadWaiter, readCounters []N.CountFunc, writeCounters []N.CountFunc) (handled bool, n int64, err error) {
	return
}

func createSyscallReadWaiter(reader any) (N.ReadWaiter, bool) {
	return nil, false
}

func createSyscallPacketReadWaiter(reader any) (N.PacketReadWaiter, bool) {
	return nil, false
}
