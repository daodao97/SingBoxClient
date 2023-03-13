//go:build !windows

package bufio

import (
	"unsafe"

	"github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"

	"golang.org/x/sys/unix"
)

func (w *SyscallVectorisedWriter) WriteVectorised(buffers []*buf.Buffer) error {
	defer buf.ReleaseMulti(buffers)
	iovecList := make([]unix.Iovec, 0, len(buffers))
	for _, buffer := range buffers {
		var iovec unix.Iovec
		iovec.Base = &buffer.Bytes()[0]
		iovec.SetLen(buffer.Len())
		iovecList = append(iovecList, iovec)
	}
	var innerErr unix.Errno
	err := w.rawConn.Write(func(fd uintptr) (done bool) {
		//nolint:staticcheck
		//goland:noinspection GoDeprecation
		_, _, innerErr = unix.Syscall(unix.SYS_WRITEV, fd, uintptr(unsafe.Pointer(&iovecList[0])), uintptr(len(iovecList)))
		return innerErr != unix.EAGAIN && innerErr != unix.EWOULDBLOCK
	})
	if innerErr != 0 {
		err = innerErr
	}
	return err
}

func (w *SyscallVectorisedPacketWriter) WriteVectorisedPacket(buffers []*buf.Buffer, destination M.Socksaddr) error {
	defer buf.ReleaseMulti(buffers)
	var sockaddr unix.Sockaddr
	if destination.IsIPv4() {
		sockaddr = &unix.SockaddrInet4{
			Port: int(destination.Port),
			Addr: destination.Addr.As4(),
		}
	} else {
		sockaddr = &unix.SockaddrInet6{
			Port: int(destination.Port),
			Addr: destination.Addr.As16(),
		}
	}
	var innerErr error
	err := w.rawConn.Write(func(fd uintptr) (done bool) {
		_, innerErr = unix.SendmsgBuffers(int(fd), buf.ToSliceMulti(buffers), nil, sockaddr, 0)
		return innerErr != unix.EAGAIN && innerErr != unix.EWOULDBLOCK
	})
	if innerErr != nil {
		err = innerErr
	}
	return err
}
