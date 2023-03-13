//go:build !linux

package control

func RoutingMark(mark int) Func {
	return nil
}
