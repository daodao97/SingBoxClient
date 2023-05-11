package network

import (
	"github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"
)

type ReadWaiter interface {
	WaitReadBuffer(newBuffer func() *buf.Buffer) error
}

type ReadWaitCreator interface {
	CreateReadWaiter() (ReadWaiter, bool)
}

type PacketReadWaiter interface {
	WaitReadPacket(newBuffer func() *buf.Buffer) (destination M.Socksaddr, err error)
}

type PacketReadWaitCreator interface {
	CreateReadWaiter() (PacketReadWaiter, bool)
}
