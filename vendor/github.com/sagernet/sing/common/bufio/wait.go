package bufio

import (
	"io"

	N "github.com/sagernet/sing/common/network"
)

func CreateReadWaiter(reader io.Reader) (N.ReadWaiter, bool) {
	reader = N.UnwrapReader(reader)
	if readWaiter, isReadWaiter := reader.(N.ReadWaiter); isReadWaiter {
		return readWaiter, true
	}
	if readWaitCreator, isCreator := reader.(N.ReadWaitCreator); isCreator {
		return readWaitCreator.CreateReadWaiter()
	}
	if readWaiter, created := createSyscallReadWaiter(reader); created {
		return readWaiter, true
	}
	return nil, false
}

func CreatePacketReadWaiter(reader N.PacketReader) (N.PacketReadWaiter, bool) {
	reader = N.UnwrapPacketReader(reader)
	if readWaiter, isReadWaiter := reader.(N.PacketReadWaiter); isReadWaiter {
		return readWaiter, true
	}
	if readWaitCreator, isCreator := reader.(N.PacketReadWaitCreator); isCreator {
		return readWaitCreator.CreateReadWaiter()
	}
	if readWaiter, created := createSyscallPacketReadWaiter(reader); created {
		return readWaiter, true
	}
	return nil, false
}
