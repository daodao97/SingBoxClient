package bufio

import (
	"github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"

	"golang.org/x/sys/windows"
)

func (w *SyscallVectorisedWriter) WriteVectorised(buffers []*buf.Buffer) error {
	defer buf.ReleaseMulti(buffers)
	iovecList := make([]*windows.WSABuf, len(buffers))
	for i, buffer := range buffers {
		iovecList[i] = &windows.WSABuf{
			Len: uint32(buffer.Len()),
			Buf: &buffer.Bytes()[0],
		}
	}
	var n uint32
	var innerErr error
	err := w.rawConn.Write(func(fd uintptr) (done bool) {
		innerErr = windows.WSASend(windows.Handle(fd), iovecList[0], uint32(len(iovecList)), &n, 0, nil, nil)
		return innerErr != windows.WSAEWOULDBLOCK
	})
	if innerErr != nil {
		err = innerErr
	}
	return err
}

func (w *SyscallVectorisedPacketWriter) WriteVectorisedPacket(buffers []*buf.Buffer, destination M.Socksaddr) error {
	defer buf.ReleaseMulti(buffers)
	iovecList := make([]*windows.WSABuf, len(buffers))
	for i, buffer := range buffers {
		iovecList[i] = &windows.WSABuf{
			Len: uint32(buffer.Len()),
			Buf: &buffer.Bytes()[0],
		}
	}
	var sockaddr windows.Sockaddr
	if destination.IsIPv4() {
		sockaddr = &windows.SockaddrInet4{
			Port: int(destination.Port),
			Addr: destination.Addr.As4(),
		}
	} else {
		sockaddr = &windows.SockaddrInet6{
			Port: int(destination.Port),
			Addr: destination.Addr.As16(),
		}
	}
	var n uint32
	var innerErr error
	err := w.rawConn.Write(func(fd uintptr) (done bool) {
		innerErr = windows.WSASendto(windows.Handle(fd), iovecList[0], uint32(len(iovecList)), &n, 0, sockaddr, nil, nil)
		return innerErr != windows.WSAEWOULDBLOCK
	})
	if innerErr != nil {
		err = innerErr
	}
	return err
}
