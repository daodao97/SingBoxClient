package bufio

import (
	"io"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	N "github.com/sagernet/sing/common/network"
)

func CopyTimes(dst io.Writer, src io.Reader, times int) (n int64, err error) {
	return CopyExtendedTimes(NewExtendedWriter(N.UnwrapWriter(dst)), NewExtendedReader(N.UnwrapReader(src)), times)
}

func CopyExtendedTimes(dst N.ExtendedWriter, src N.ExtendedReader, times int) (n int64, err error) {
	frontHeadroom := N.CalculateFrontHeadroom(dst)
	rearHeadroom := N.CalculateRearHeadroom(dst)
	bufferSize := N.CalculateMTU(src, dst)
	if bufferSize > 0 {
		bufferSize += frontHeadroom + rearHeadroom
	} else {
		bufferSize = buf.BufferSize
	}
	dstUnsafe := N.IsUnsafeWriter(dst)
	var buffer *buf.Buffer
	if !dstUnsafe {
		_buffer := buf.StackNewSize(bufferSize)
		defer common.KeepAlive(_buffer)
		buffer = common.Dup(_buffer)
		defer buffer.Release()
		buffer.IncRef()
		defer buffer.DecRef()
	}
	notFirstTime := true
	for i := 0; i < times; i++ {
		if dstUnsafe {
			buffer = buf.NewSize(bufferSize)
		}
		readBufferRaw := buffer.Slice()
		readBuffer := buf.With(readBufferRaw[:cap(readBufferRaw)-rearHeadroom])
		readBuffer.Resize(frontHeadroom, 0)
		err = src.ReadBuffer(readBuffer)
		if err != nil {
			buffer.Release()
			if !notFirstTime {
				err = N.HandshakeFailure(dst, err)
			}
			return
		}
		dataLen := readBuffer.Len()
		buffer.Resize(readBuffer.Start(), dataLen)
		err = dst.WriteBuffer(buffer)
		if err != nil {
			buffer.Release()
			return
		}
		n += int64(dataLen)
		notFirstTime = true
	}
	return
}

type ReadFromWriter interface {
	io.ReaderFrom
	io.Writer
}

func ReadFrom0(readerFrom ReadFromWriter, reader io.Reader) (n int64, err error) {
	n, err = CopyTimes(readerFrom, reader, 1)
	if err != nil {
		return
	}
	var rn int64
	rn, err = readerFrom.ReadFrom(reader)
	if err != nil {
		return
	}
	n += rn
	return
}

func ReadFromN(readerFrom ReadFromWriter, reader io.Reader, times int) (n int64, err error) {
	n, err = CopyTimes(readerFrom, reader, times)
	if err != nil {
		return
	}
	var rn int64
	rn, err = readerFrom.ReadFrom(reader)
	if err != nil {
		return
	}
	n += rn
	return
}

type WriteToReader interface {
	io.WriterTo
	io.Reader
}

func WriteTo0(writerTo WriteToReader, writer io.Writer) (n int64, err error) {
	n, err = CopyTimes(writer, writerTo, 1)
	if err != nil {
		return
	}
	var wn int64
	wn, err = writerTo.WriteTo(writer)
	if err != nil {
		return
	}
	n += wn
	return
}

func WriteToN(writerTo WriteToReader, writer io.Writer, times int) (n int64, err error) {
	n, err = CopyTimes(writer, writerTo, times)
	if err != nil {
		return
	}
	var wn int64
	wn, err = writerTo.WriteTo(writer)
	if err != nil {
		return
	}
	n += wn
	return
}
