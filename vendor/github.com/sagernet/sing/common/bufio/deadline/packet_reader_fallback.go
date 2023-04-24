package deadline

import (
	"net"
	"time"

	"github.com/sagernet/sing/common/atomic"
	"github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"
)

type fallbackPacketReader struct {
	*packetReader
	disablePipe atomic.Bool
	inRead      atomic.Bool
}

func NewFallbackPacketReader(timeoutReader TimeoutPacketReader) PacketReader {
	return &fallbackPacketReader{packetReader: NewPacketReader(timeoutReader).(*packetReader)}
}

func (r *fallbackPacketReader) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	select {
	case result := <-r.result:
		return r.pipeReturnFrom(result, p)
	default:
	}
	if r.disablePipe.Load() {
		return r.TimeoutPacketReader.ReadFrom(p)
	}
	select {
	case <-r.done:
		if r.deadline.Load().IsZero() {
			r.done <- struct{}{}
			r.inRead.Store(true)
			defer r.inRead.Store(false)
			n, addr, err = r.TimeoutPacketReader.ReadFrom(p)
			return
		}
		go r.pipeReadFrom(len(p))
	default:
	}
	return r.readFrom(p)
}

func (r *fallbackPacketReader) ReadPacket(buffer *buf.Buffer) (destination M.Socksaddr, err error) {
	select {
	case result := <-r.result:
		return r.pipeReturnFromBuffer(result, buffer)
	default:
	}
	if r.disablePipe.Load() {
		return r.TimeoutPacketReader.ReadPacket(buffer)
	}
	select {
	case <-r.done:
		if r.deadline.Load().IsZero() {
			r.done <- struct{}{}
			r.inRead.Store(true)
			defer r.inRead.Store(false)
			destination, err = r.TimeoutPacketReader.ReadPacket(buffer)
			return
		}
		go r.pipeReadFromBuffer(buffer.Cap(), buffer.Start())
	default:
	}
	return r.readPacket(buffer)
}

func (r *fallbackPacketReader) SetReadDeadline(t time.Time) error {
	if r.disablePipe.Load() {
		return r.TimeoutPacketReader.SetReadDeadline(t)
	} else if r.inRead.Load() {
		r.disablePipe.Store(true)
		return r.TimeoutPacketReader.SetReadDeadline(t)
	}
	return r.packetReader.SetReadDeadline(t)
}

func (r *fallbackPacketReader) ReaderReplaceable() bool {
	return r.disablePipe.Load() || r.packetReader.ReaderReplaceable()
}

func (r *fallbackPacketReader) UpstreamReader() any {
	return r.packetReader.UpstreamReader()
}
