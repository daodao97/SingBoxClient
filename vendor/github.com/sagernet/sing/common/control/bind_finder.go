package control

import "net"

type InterfaceFinder interface {
	InterfaceIndexByName(name string) (int, error)
	InterfaceNameByIndex(index int) (string, error)
}

func DefaultInterfaceFinder() InterfaceFinder {
	return (*netInterfaceFinder)(nil)
}

type netInterfaceFinder struct{}

func (w *netInterfaceFinder) InterfaceIndexByName(name string) (int, error) {
	netInterface, err := net.InterfaceByName(name)
	if err != nil {
		return 0, err
	}
	return netInterface.Index, nil
}

func (w *netInterfaceFinder) InterfaceNameByIndex(index int) (string, error) {
	netInterface, err := net.InterfaceByIndex(index)
	if err != nil {
		return "", err
	}
	return netInterface.Name, nil
}
