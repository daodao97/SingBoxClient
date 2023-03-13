package control

import (
	"os"
	"runtime"
	"syscall"

	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

func BindToInterface(finder InterfaceFinder, interfaceName string, interfaceIndex int) Func {
	return func(network, address string, conn syscall.RawConn) error {
		return BindToInterface0(finder, conn, network, address, interfaceName, interfaceIndex)
	}
}

func BindToInterfaceFunc(finder InterfaceFinder, block func(network string, address string) (interfaceName string, interfaceIndex int)) Func {
	return func(network, address string, conn syscall.RawConn) error {
		interfaceName, interfaceIndex := block(network, address)
		return BindToInterface0(finder, conn, network, address, interfaceName, interfaceIndex)
	}
}

const useInterfaceName = runtime.GOOS == "linux" || runtime.GOOS == "android"

func BindToInterface0(finder InterfaceFinder, conn syscall.RawConn, network string, address string, interfaceName string, interfaceIndex int) error {
	if addr := M.ParseSocksaddr(address).Addr; addr.IsValid() && N.IsVirtual(addr) {
		return nil
	}
	if interfaceName == "" && interfaceIndex == -1 {
		return nil
	}
	if interfaceName != "" && useInterfaceName || interfaceIndex != -1 && !useInterfaceName {
		return bindToInterface(conn, network, address, interfaceName, interfaceIndex)
	}
	if finder == nil {
		return os.ErrInvalid
	}
	var err error
	if useInterfaceName {
		interfaceName, err = finder.InterfaceNameByIndex(interfaceIndex)
	} else {
		interfaceIndex, err = finder.InterfaceIndexByName(interfaceName)
	}
	if err != nil {
		return err
	}
	if useInterfaceName {
		if interfaceName == "" {
			return nil
		}
	} else {
		if interfaceIndex == -1 {
			return nil
		}
	}
	return bindToInterface(conn, network, address, interfaceName, interfaceIndex)
}
