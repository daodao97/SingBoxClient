//go:build openbsd

package ipc

import "golang.org/x/sys/unix"

const (
	IpcErrorIO        = -int64(unix.EIO)
	IpcErrorProtocol  = -95
	IpcErrorInvalid   = -int64(unix.EINVAL)
	IpcErrorPortInUse = -int64(unix.EADDRINUSE)
	IpcErrorUnknown   = -55 // ENOANO
)
