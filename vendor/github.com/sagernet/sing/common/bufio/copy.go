package bufio

import (
	"context"
	"errors"
	"io"
	"net"
	"syscall"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/rw"
	"github.com/sagernet/sing/common/task"
)

func Copy(destination io.Writer, source io.Reader) (n int64, err error) {
	if source == nil {
		return 0, E.New("nil reader")
	} else if destination == nil {
		return 0, E.New("nil writer")
	}
	originDestination := destination
	var readCounters, writeCounters []N.CountFunc
	for {
		source, readCounters = N.UnwrapCountReader(source, readCounters)
		destination, writeCounters = N.UnwrapCountWriter(destination, writeCounters)
		if cachedSrc, isCached := source.(N.CachedReader); isCached {
			cachedBuffer := cachedSrc.ReadCached()
			if cachedBuffer != nil {
				if !cachedBuffer.IsEmpty() {
					_, err = destination.Write(cachedBuffer.Bytes())
					if err != nil {
						cachedBuffer.Release()
						return
					}
				}
				cachedBuffer.Release()
				continue
			}
		}
		srcSyscallConn, srcIsSyscall := source.(syscall.Conn)
		dstSyscallConn, dstIsSyscall := destination.(syscall.Conn)
		if srcIsSyscall && dstIsSyscall {
			var handled bool
			handled, n, err = CopyDirect(srcSyscallConn, dstSyscallConn, readCounters, writeCounters)
			if handled {
				return
			}
		}
		break
	}
	return CopyExtended(originDestination, NewExtendedWriter(destination), NewExtendedReader(source), readCounters, writeCounters)
}

func CopyExtended(originDestination io.Writer, destination N.ExtendedWriter, source N.ExtendedReader, readCounters []N.CountFunc, writeCounters []N.CountFunc) (n int64, err error) {
	safeSrc := N.IsSafeReader(source)
	headroom := N.CalculateFrontHeadroom(destination) + N.CalculateRearHeadroom(destination)
	if safeSrc != nil {
		if headroom == 0 {
			return CopyExtendedWithSrcBuffer(originDestination, destination, safeSrc, readCounters, writeCounters)
		}
	}
	readWaiter, isReadWaiter := CreateReadWaiter(source)
	if isReadWaiter {
		var handled bool
		handled, n, err = copyWaitWithPool(originDestination, destination, readWaiter, readCounters, writeCounters)
		if handled {
			return
		}
	}
	if !common.UnsafeBuffer || N.IsUnsafeWriter(destination) {
		return CopyExtendedWithPool(originDestination, destination, source, readCounters, writeCounters)
	}
	bufferSize := N.CalculateMTU(source, destination)
	if bufferSize > 0 {
		bufferSize += headroom
	} else {
		bufferSize = buf.BufferSize
	}
	_buffer := buf.StackNewSize(bufferSize)
	defer common.KeepAlive(_buffer)
	buffer := common.Dup(_buffer)
	defer buffer.Release()
	return CopyExtendedBuffer(originDestination, destination, source, buffer, readCounters, writeCounters)
}

func CopyExtendedBuffer(originDestination io.Writer, destination N.ExtendedWriter, source N.ExtendedReader, buffer *buf.Buffer, readCounters []N.CountFunc, writeCounters []N.CountFunc) (n int64, err error) {
	buffer.IncRef()
	defer buffer.DecRef()
	frontHeadroom := N.CalculateFrontHeadroom(destination)
	rearHeadroom := N.CalculateRearHeadroom(destination)
	readBufferRaw := buffer.Slice()
	readBuffer := buf.With(readBufferRaw[:len(readBufferRaw)-rearHeadroom])
	var notFirstTime bool
	for {
		readBuffer.Resize(frontHeadroom, 0)
		err = source.ReadBuffer(readBuffer)
		if err != nil {
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

func CopyExtendedWithSrcBuffer(originDestination io.Writer, destination N.ExtendedWriter, source N.ThreadSafeReader, readCounters []N.CountFunc, writeCounters []N.CountFunc) (n int64, err error) {
	var notFirstTime bool
	for {
		var buffer *buf.Buffer
		buffer, err = source.ReadBufferThreadSafe()
		if err != nil {
			if errors.Is(err, io.EOF) {
				err = nil
				return
			}
			if !notFirstTime {
				err = N.HandshakeFailure(originDestination, err)
			}
			return
		}
		dataLen := buffer.Len()
		err = destination.WriteBuffer(buffer)
		if err != nil {
			buffer.Release()
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

func CopyExtendedWithPool(originDestination io.Writer, destination N.ExtendedWriter, source N.ExtendedReader, readCounters []N.CountFunc, writeCounters []N.CountFunc) (n int64, err error) {
	frontHeadroom := N.CalculateFrontHeadroom(destination)
	rearHeadroom := N.CalculateRearHeadroom(destination)
	bufferSize := N.CalculateMTU(source, destination)
	if bufferSize > 0 {
		bufferSize += frontHeadroom + rearHeadroom
	} else {
		bufferSize = buf.BufferSize
	}
	var notFirstTime bool
	for {
		buffer := buf.NewSize(bufferSize)
		readBufferRaw := buffer.Slice()
		readBuffer := buf.With(readBufferRaw[:len(readBufferRaw)-rearHeadroom])
		readBuffer.Resize(frontHeadroom, 0)
		err = source.ReadBuffer(readBuffer)
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
			buffer.Release()
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

func CopyConn(ctx context.Context, source net.Conn, destination net.Conn) error {
	return CopyConnContextList([]context.Context{ctx}, source, destination)
}

func CopyConnContextList(contextList []context.Context, source net.Conn, destination net.Conn) error {
	var group task.Group
	if _, dstDuplex := common.Cast[rw.WriteCloser](destination); dstDuplex {
		group.Append("upload", func(ctx context.Context) error {
			err := common.Error(Copy(destination, source))
			if err == nil {
				rw.CloseWrite(destination)
			} else {
				common.Close(destination)
			}
			return err
		})
	} else {
		group.Append("upload", func(ctx context.Context) error {
			defer common.Close(destination)
			return common.Error(Copy(destination, source))
		})
	}
	if _, srcDuplex := common.Cast[rw.WriteCloser](source); srcDuplex {
		group.Append("download", func(ctx context.Context) error {
			err := common.Error(Copy(source, destination))
			if err == nil {
				rw.CloseWrite(source)
			} else {
				common.Close(source)
			}
			return err
		})
	} else {
		group.Append("download", func(ctx context.Context) error {
			defer common.Close(source)
			return common.Error(Copy(source, destination))
		})
	}
	group.Cleanup(func() {
		common.Close(source, destination)
	})
	return group.RunContextList(contextList)
}

func CopyPacket(destinationConn N.PacketWriter, source N.PacketReader) (n int64, err error) {
	var readCounters, writeCounters []N.CountFunc
	var cachedPackets []*N.PacketBuffer
	for {
		source, readCounters = N.UnwrapCountPacketReader(source, readCounters)
		destinationConn, writeCounters = N.UnwrapCountPacketWriter(destinationConn, writeCounters)
		if cachedReader, isCached := source.(N.CachedPacketReader); isCached {
			packet := cachedReader.ReadCachedPacket()
			if packet != nil {
				cachedPackets = append(cachedPackets, packet)
				continue
			}
		}
		break
	}
	if cachedPackets != nil {
		n, err = WritePacketWithPool(destinationConn, cachedPackets)
		if err != nil {
			return
		}
	}
	safeSrc := N.IsSafePacketReader(source)
	frontHeadroom := N.CalculateFrontHeadroom(destinationConn)
	rearHeadroom := N.CalculateRearHeadroom(destinationConn)
	headroom := frontHeadroom + rearHeadroom
	if safeSrc != nil {
		if headroom == 0 {
			var copyN int64
			copyN, err = CopyPacketWithSrcBuffer(destinationConn, safeSrc, readCounters, writeCounters)
			n += copyN
			return
		}
	}
	readWaiter, isReadWaiter := CreatePacketReadWaiter(source)
	if isReadWaiter {
		var (
			handled bool
			copeN   int64
		)
		handled, copeN, err = copyPacketWaitWithPool(destinationConn, readWaiter, readCounters, writeCounters)
		if handled {
			n += copeN
			return
		}
	}
	if N.IsUnsafeWriter(destinationConn) {
		return CopyPacketWithPool(destinationConn, source, readCounters, writeCounters)
	}
	bufferSize := N.CalculateMTU(source, destinationConn)
	if bufferSize > 0 {
		bufferSize += headroom
	} else {
		bufferSize = buf.UDPBufferSize
	}
	_buffer := buf.StackNewSize(bufferSize)
	defer common.KeepAlive(_buffer)
	buffer := common.Dup(_buffer)
	defer buffer.Release()
	buffer.IncRef()
	defer buffer.DecRef()
	var destination M.Socksaddr
	var notFirstTime bool
	readBufferRaw := buffer.Slice()
	readBuffer := buf.With(readBufferRaw[:len(readBufferRaw)-rearHeadroom])
	for {
		readBuffer.Resize(frontHeadroom, 0)
		destination, err = source.ReadPacket(readBuffer)
		if err != nil {
			if !notFirstTime {
				err = N.HandshakeFailure(destinationConn, err)
			}
			return
		}
		dataLen := readBuffer.Len()
		buffer.Resize(readBuffer.Start(), dataLen)
		err = destinationConn.WritePacket(buffer, destination)
		if err != nil {
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

func CopyPacketWithSrcBuffer(destinationConn N.PacketWriter, source N.ThreadSafePacketReader, readCounters []N.CountFunc, writeCounters []N.CountFunc) (n int64, err error) {
	var buffer *buf.Buffer
	var destination M.Socksaddr
	var notFirstTime bool
	for {
		buffer, destination, err = source.ReadPacketThreadSafe()
		if err != nil {
			if !notFirstTime {
				err = N.HandshakeFailure(destinationConn, err)
			}
			return
		}
		dataLen := buffer.Len()
		if dataLen == 0 {
			continue
		}
		err = destinationConn.WritePacket(buffer, destination)
		if err != nil {
			buffer.Release()
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

func CopyPacketWithPool(destinationConn N.PacketWriter, source N.PacketReader, readCounters []N.CountFunc, writeCounters []N.CountFunc) (n int64, err error) {
	frontHeadroom := N.CalculateFrontHeadroom(destinationConn)
	rearHeadroom := N.CalculateRearHeadroom(destinationConn)
	bufferSize := N.CalculateMTU(source, destinationConn)
	if bufferSize > 0 {
		bufferSize += frontHeadroom + rearHeadroom
	} else {
		bufferSize = buf.UDPBufferSize
	}
	var destination M.Socksaddr
	var notFirstTime bool
	for {
		buffer := buf.NewSize(bufferSize)
		readBufferRaw := buffer.Slice()
		readBuffer := buf.With(readBufferRaw[:len(readBufferRaw)-rearHeadroom])
		readBuffer.Resize(frontHeadroom, 0)
		destination, err = source.ReadPacket(readBuffer)
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
		}
		n += int64(dataLen)
		for _, counter := range readCounters {
			counter(n)
		}
		for _, counter := range writeCounters {
			counter(n)
		}
		notFirstTime = true
	}
}

func WritePacketWithPool(destinationConn N.PacketWriter, packetBuffers []*N.PacketBuffer) (n int64, err error) {
	frontHeadroom := N.CalculateFrontHeadroom(destinationConn)
	rearHeadroom := N.CalculateRearHeadroom(destinationConn)
	for _, packetBuffer := range packetBuffers {
		buffer := buf.NewPacket()
		readBufferRaw := buffer.Slice()
		readBuffer := buf.With(readBufferRaw[:len(readBufferRaw)-rearHeadroom])
		readBuffer.Resize(frontHeadroom, 0)
		_, err = readBuffer.Write(packetBuffer.Buffer.Bytes())
		packetBuffer.Buffer.Release()
		if err != nil {
			continue
		}
		dataLen := readBuffer.Len()
		buffer.Resize(readBuffer.Start(), dataLen)
		err = destinationConn.WritePacket(buffer, packetBuffer.Destination)
		if err != nil {
			buffer.Release()
			return
		}
		n += int64(dataLen)
	}
	return
}

func CopyPacketConn(ctx context.Context, source N.PacketConn, destination N.PacketConn) error {
	return CopyPacketConnContextList([]context.Context{ctx}, source, destination)
}

func CopyPacketConnContextList(contextList []context.Context, source N.PacketConn, destination N.PacketConn) error {
	var group task.Group
	group.Append("upload", func(ctx context.Context) error {
		return common.Error(CopyPacket(destination, source))
	})
	group.Append("download", func(ctx context.Context) error {
		return common.Error(CopyPacket(source, destination))
	})
	group.Cleanup(func() {
		common.Close(source, destination)
	})
	group.FastFail()
	return group.RunContextList(contextList)
}
