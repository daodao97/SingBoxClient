package deadline

import (
	"net"
	"os"
	"sync"
	"time"

	"github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type PacketReader struct {
	N.NetPacketReader
	deadline     time.Time
	pipeDeadline pipeDeadline
	cacheAccess  sync.RWMutex
	cached       bool
	cachedBuffer *buf.Buffer
	cachedAddr   M.Socksaddr
	cachedErr    error
}

func NewPacketReader(reader N.NetPacketReader) *PacketReader {
	return &PacketReader{NetPacketReader: reader, pipeDeadline: makePipeDeadline()}
}

func (r *PacketReader) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	r.cacheAccess.Lock()
	if r.cached {
		n = copy(p, r.cachedBuffer.Bytes())
		addr = r.cachedAddr.UDPAddr()
		err = r.cachedErr
		r.cachedBuffer.Release()
		r.cached = false
		r.cacheAccess.Unlock()
		return
	}
	r.cacheAccess.Unlock()
	done := make(chan struct{})
	go func() {
		n, addr, err = r.pipeReadFrom(p, r.pipeDeadline.wait())
		close(done)
	}()
	select {
	case <-done:
		return
	case <-r.pipeDeadline.wait():
		return 0, nil, os.ErrDeadlineExceeded
	}
}

func (r *PacketReader) pipeReadFrom(p []byte, cancel chan struct{}) (n int, addr net.Addr, err error) {
	r.cacheAccess.Lock()
	defer r.cacheAccess.Unlock()
	cacheBuffer := buf.NewSize(len(p))
	n, addr, err = r.NetPacketReader.ReadFrom(cacheBuffer.Bytes())
	if isClosedChan(cancel) {
		r.cached = true
		r.cachedBuffer = cacheBuffer
		r.cachedAddr = M.SocksaddrFromNet(addr)
		r.cachedErr = err
	} else {
		copy(p, cacheBuffer.Bytes())
		cacheBuffer.Release()
	}
	return
}

func (r *PacketReader) ReadPacket(buffer *buf.Buffer) (destination M.Socksaddr, err error) {
	r.cacheAccess.Lock()
	if r.cached {
		destination = r.cachedAddr
		err = r.cachedErr
		buffer.Write(r.cachedBuffer.Bytes())
		r.cachedBuffer.Release()
		r.cached = false
		r.cacheAccess.Unlock()
		return
	}
	r.cacheAccess.Unlock()
	done := make(chan struct{})
	go func() {
		destination, err = r.pipeReadPacket(buffer, r.pipeDeadline.wait())
		close(done)
	}()
	select {
	case <-done:
		return
	case <-r.pipeDeadline.wait():
		return M.Socksaddr{}, os.ErrDeadlineExceeded
	}
}

func (r *PacketReader) pipeReadPacket(buffer *buf.Buffer, cancel chan struct{}) (destination M.Socksaddr, err error) {
	r.cacheAccess.Lock()
	defer r.cacheAccess.Unlock()
	cacheBuffer := buf.NewSize(buffer.FreeLen())
	destination, err = r.NetPacketReader.ReadPacket(cacheBuffer)
	if isClosedChan(cancel) {
		r.cached = true
		r.cachedBuffer = cacheBuffer
		r.cachedAddr = destination
		r.cachedErr = err
	} else {
		buffer.ReadOnceFrom(cacheBuffer)
		cacheBuffer.Release()
	}
	return
}

func (r *PacketReader) SetReadDeadline(t time.Time) error {
	r.deadline = t
	r.pipeDeadline.set(t)
	return nil
}

func (r *PacketReader) ReaderReplaceable() bool {
	return r.deadline.IsZero()
}

func (r *PacketReader) UpstreamReader() any {
	return r.NetPacketReader
}
