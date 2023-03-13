package rw

import (
	"syscall"
)

// Deprecated: use vectorised writer
func WriteV(fd uintptr, data [][]byte) (int, error) {
	var n uint32
	buffers := make([]*syscall.WSABuf, len(data))
	for i, buf := range data {
		buffers[i] = &syscall.WSABuf{
			Len: uint32(len(buf)),
			Buf: &buf[0],
		}
	}
	err := syscall.WSASend(syscall.Handle(fd), buffers[0], uint32(len(buffers)), &n, 0, nil, nil)
	return int(n), err
}
