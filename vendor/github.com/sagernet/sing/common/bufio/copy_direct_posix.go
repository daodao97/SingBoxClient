//go:build !windows

package bufio

import (
	"errors"
	"io"
	"net/netip"
	"syscall"

	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

func copyWaitWithPool(originDestination io.Writer, destination N.ExtendedWriter, source N.ReadWaiter, readCounters []N.CountFunc, writeCounters []N.CountFunc) (handled bool, n int64, err error) {
	handled = true
	frontHeadroom := N.CalculateFrontHeadroom(destination)
	rearHeadroom := N.CalculateRearHeadroom(destination)
	bufferSize := N.CalculateMTU(source, destination)
	if bufferSize > 0 {
		bufferSize += frontHeadroom + rearHeadroom
	} else {
		bufferSize = buf.BufferSize
	}
	var (
		buffer     *buf.Buffer
		readBuffer *buf.Buffer
	)
	newBuffer := func() *buf.Buffer {
		if buffer != nil {
			buffer.Release()
		}
		buffer = buf.NewSize(bufferSize)
		readBufferRaw := buffer.Slice()
		readBuffer = buf.With(readBufferRaw[:len(readBufferRaw)-rearHeadroom])
		readBuffer.Resize(frontHeadroom, 0)
		return readBuffer
	}
	var notFirstTime bool
	for {
		err = source.WaitReadBuffer(newBuffer)
		if err != nil {
			buffer.Release()
			if errors.Is(err, io.EOF) {
				err = nil
				return
			}
			if !notFirstTime {
				err = N.HandshakeFailure(originDestination, err)
			}
			return
		}
		dataLen := readBuffer.Len()
		buffer.Resize(readBuffer.Start(), dataLen)
		err = destination.WriteBuffer(buffer)
		if err != nil {
			if buffer != nil {
				buffer.Release()
			}
			return
		}
		n += int64(dataLen)
		for _, counter := range readCounters {
			counter(int64(dataLen))
		}
		for _, counter := range writeCounters {
			counter(int64(dataLen))
		}
		notFirstTime = true
	}
}

func copyPacketWaitWithPool(destinationConn N.PacketWriter, source N.PacketReadWaiter, readCounters []N.CountFunc, writeCounters []N.CountFunc) (handled bool, n int64, err error) {
	handled = true
	frontHeadroom := N.CalculateFrontHeadroom(destinationConn)
	rearHeadroom := N.CalculateRearHeadroom(destinationConn)
	bufferSize := N.CalculateMTU(source, destinationConn)
	if bufferSize > 0 {
		bufferSize += frontHeadroom + rearHeadroom
	} else {
		bufferSize = buf.UDPBufferSize
	}
	var (
		buffer     *buf.Buffer
		readBuffer *buf.Buffer
	)
	newBuffer := func() *buf.Buffer {
		if buffer != nil {
			buffer.Release()
		}
		buffer = buf.NewSize(bufferSize)
		readBufferRaw := buffer.Slice()
		readBuffer = buf.With(readBufferRaw[:len(readBufferRaw)-rearHeadroom])
		readBuffer.Resize(frontHeadroom, 0)
		return readBuffer
	}
	var destination M.Socksaddr
	var notFirstTime bool
	for {
		destination, err = source.WaitReadPacket(newBuffer)
		if err != nil {
			buffer.Release()
			if !notFirstTime {
				err = N.HandshakeFailure(destinationConn, err)
			}
			return
		}
		dataLen := readBuffer.Len()
		buffer.Resize(readBuffer.Start(), dataLen)
		err = destinationConn.WritePacket(buffer, destination)
		if err != nil {
			buffer.Release()
			return
		} else {
			buffer = nil
		}
		n += int64(dataLen)
		for _, counter := range readCounters {
			counter(int64(dataLen))
		}
		for _, counter := range writeCounters {
			counter(int64(dataLen))
		}
		notFirstTime = true
	}
}

type syscallReadWaiter struct {
	rawConn  syscall.RawConn
	readErr  error
	readFunc func(fd uintptr) (done bool)
}

func createSyscallReadWaiter(reader any) (*syscallReadWaiter, bool) {
	if syscallConn, isSyscallConn := reader.(syscall.Conn); isSyscallConn {
		rawConn, err := syscallConn.SyscallConn()
		if err == nil {
			return &syscallReadWaiter{rawConn: rawConn}, true
		}
	}
	return nil, false
}

func (w *syscallReadWaiter) WaitReadBuffer(newBuffer func() *buf.Buffer) error {
	if w.readFunc == nil {
		w.readFunc = func(fd uintptr) (done bool) {
			buffer := newBuffer()
			var readN int
			readN, w.readErr = syscall.Read(int(fd), buffer.FreeBytes())
			if readN > 0 {
				buffer.Truncate(readN)
			} else {
				buffer.Release()
				buffer = nil
			}
			if w.readErr == syscall.EAGAIN {
				return false
			}
			if readN == 0 {
				w.readErr = io.EOF
			}
			return true
		}
	}
	err := w.rawConn.Read(w.readFunc)
	if err != nil {
		return err
	}
	if w.readErr != nil {
		return E.Cause(w.readErr, "raw read")
	}
	return nil
}

type syscallPacketReadWaiter struct {
	rawConn  syscall.RawConn
	readErr  error
	readFrom M.Socksaddr
	readFunc func(fd uintptr) (done bool)
}

func createSyscallPacketReadWaiter(reader any) (*syscallPacketReadWaiter, bool) {
	if syscallConn, isSyscallConn := reader.(syscall.Conn); isSyscallConn {
		rawConn, err := syscallConn.SyscallConn()
		if err == nil {
			return &syscallPacketReadWaiter{rawConn: rawConn}, true
		}
	}
	return nil, false
}

func (w *syscallPacketReadWaiter) WaitReadPacket(newBuffer func() *buf.Buffer) (destination M.Socksaddr, err error) {
	if w.readFunc == nil {
		w.readFunc = func(fd uintptr) (done bool) {
			buffer := newBuffer()
			var readN int
			var from syscall.Sockaddr
			readN, _, _, from, w.readErr = syscall.Recvmsg(int(fd), buffer.FreeBytes(), nil, 0)
			if readN > 0 {
				buffer.Truncate(readN)
			} else {
				buffer.Release()
				buffer = nil
			}
			if w.readErr == syscall.EAGAIN {
				return false
			}
			if from != nil {
				switch fromAddr := from.(type) {
				case *syscall.SockaddrInet4:
					w.readFrom = M.SocksaddrFrom(netip.AddrFrom4(fromAddr.Addr), uint16(fromAddr.Port))
				case *syscall.SockaddrInet6:
					w.readFrom = M.SocksaddrFrom(netip.AddrFrom16(fromAddr.Addr), uint16(fromAddr.Port))
				}
			}
			if readN == 0 {
				w.readErr = io.EOF
			}
			return true
		}
	}
	err = w.rawConn.Read(w.readFunc)
	if err != nil {
		return
	}
	if w.readErr != nil {
		err = E.Cause(w.readErr, "raw read")
		return
	}
	destination = w.readFrom
	return
}
