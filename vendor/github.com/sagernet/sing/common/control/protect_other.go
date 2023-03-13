//go:build (go1.19 && !unix) || (!go1.19 && !(linux || darwin))

package control

func ProtectPath(protectPath string) Func {
	return nil
}
