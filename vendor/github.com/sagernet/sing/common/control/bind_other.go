//go:build !(linux || windows || darwin)

package control

import "syscall"

func bindToInterface(conn syscall.RawConn, network string, address string, interfaceName string, interfaceIndex int) error {
	return nil
}
