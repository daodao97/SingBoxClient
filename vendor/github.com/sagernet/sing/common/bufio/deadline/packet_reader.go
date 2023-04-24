package deadline

import (
	"net"
	"os"
	"time"

	"github.com/sagernet/sing/common/atomic"
	"github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type TimeoutPacketReader interface {
	N.NetPacketReader
	SetReadDeadline(t time.Time) error
}

type PacketReader interface {
	TimeoutPacketReader
	N.WithUpstreamReader
	N.ReaderWithUpstream
}

type packetReader struct {
	TimeoutPacketReader
	deadline     atomic.TypedValue[time.Time]
	pipeDeadline pipeDeadline
	result       chan *packetReadResult
	done         chan struct{}
}

type packetReadResult struct {
	buffer      *buf.Buffer
	destination M.Socksaddr
	err         error
}

func NewPacketReader(timeoutReader TimeoutPacketReader) PacketReader {
	return &packetReader{
		TimeoutPacketReader: timeoutReader,
		pipeDeadline:        makePipeDeadline(),
		result:              make(chan *packetReadResult, 1),
		done:                makeFilledChan(),
	}
}

func (r *packetReader) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	select {
	case result := <-r.result:
		return r.pipeReturnFrom(result, p)
	default:
	}
	select {
	case <-r.done:
		go r.pipeReadFrom(len(p))
	default:
	}
	return r.readFrom(p)
}

func (r *packetReader) readFrom(p []byte) (n int, addr net.Addr, err error) {
	select {
	case result := <-r.result:
		return r.pipeReturnFrom(result, p)
	case <-r.pipeDeadline.wait():
		return 0, nil, os.ErrDeadlineExceeded
	}
}

func (r *packetReader) pipeReadFrom(pLen int) {
	buffer := buf.NewSize(pLen)
	n, addr, err := r.TimeoutPacketReader.ReadFrom(buffer.FreeBytes())
	buffer.Truncate(n)
	r.result <- &packetReadResult{
		buffer:      buffer,
		destination: M.SocksaddrFromNet(addr),
		err:         err,
	}
	r.done <- struct{}{}
}

func (r *packetReader) pipeReturnFrom(result *packetReadResult, p []byte) (n int, addr net.Addr, err error) {
	n = copy(p, result.buffer.Bytes())
	if result.destination.IsValid() {
		if result.destination.IsFqdn() {
			addr = result.destination
		} else {
			addr = result.destination.UDPAddr()
		}
	}
	result.buffer.Advance(n)
	if result.buffer.IsEmpty() {
		result.buffer.Release()
		err = result.err
	} else {
		r.result <- result
	}
	return
}

func (r *packetReader) ReadPacket(buffer *buf.Buffer) (destination M.Socksaddr, err error) {
	select {
	case result := <-r.result:
		return r.pipeReturnFromBuffer(result, buffer)
	default:
	}
	select {
	case <-r.done:
		go r.pipeReadFromBuffer(buffer.Cap(), buffer.Start())
	default:
	}
	return r.readPacket(buffer)
}

func (r *packetReader) readPacket(buffer *buf.Buffer) (destination M.Socksaddr, err error) {
	select {
	case result := <-r.result:
		return r.pipeReturnFromBuffer(result, buffer)
	case <-r.pipeDeadline.wait():
		return M.Socksaddr{}, os.ErrDeadlineExceeded
	}
}

func (r *packetReader) pipeReturnFromBuffer(result *packetReadResult, buffer *buf.Buffer) (M.Socksaddr, error) {
	buffer.Resize(result.buffer.Start(), 0)
	n := copy(buffer.FreeBytes(), result.buffer.Bytes())
	buffer.Truncate(n)
	result.buffer.Advance(n)
	if !result.buffer.IsEmpty() {
		r.result <- result
		return result.destination, nil
	} else {
		result.buffer.Release()
		return result.destination, result.err
	}
}

func (r *packetReader) pipeReadFromBuffer(bufLen int, bufStart int) {
	buffer := buf.NewSize(bufLen)
	buffer.Advance(bufStart)
	destination, err := r.TimeoutPacketReader.ReadPacket(buffer)
	r.result <- &packetReadResult{
		buffer:      buffer,
		destination: destination,
		err:         err,
	}
	r.done <- struct{}{}
}

func (r *packetReader) SetReadDeadline(t time.Time) error {
	r.deadline.Store(t)
	r.pipeDeadline.set(t)
	return nil
}

func (r *packetReader) ReaderReplaceable() bool {
	select {
	case <-r.done:
		r.done <- struct{}{}
	default:
		return false
	}
	select {
	case result := <-r.result:
		r.result <- result
		return false
	default:
	}
	return r.deadline.Load().IsZero()
}

func (r *packetReader) UpstreamReader() any {
	return r.TimeoutPacketReader
}
