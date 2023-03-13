package control

import (
	"syscall"

	E "github.com/sagernet/sing/common/exceptions"
)

type Func = func(network, address string, conn syscall.RawConn) error

func Append(oldFunc Func, newFunc Func) Func {
	if oldFunc == nil {
		return newFunc
	} else if newFunc == nil {
		return oldFunc
	}
	return func(network, address string, conn syscall.RawConn) error {
		if err := oldFunc(network, address, conn); err != nil {
			return err
		}
		return newFunc(network, address, conn)
	}
}

func Conn(conn syscall.Conn, block func(fd uintptr) error) error {
	rawConn, err := conn.SyscallConn()
	if err != nil {
		return err
	}
	return Raw(rawConn, block)
}

func Raw(rawConn syscall.RawConn, block func(fd uintptr) error) error {
	var innerErr error
	err := rawConn.Control(func(fd uintptr) {
		innerErr = block(fd)
	})
	return E.Errors(innerErr, err)
}
