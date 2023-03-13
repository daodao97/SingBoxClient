//go:build !((go1.19 && unix) || (!go1.19 && (linux || darwin)) || windows)

package control

func DisableUDPFragment() Func {
	return nil
}
