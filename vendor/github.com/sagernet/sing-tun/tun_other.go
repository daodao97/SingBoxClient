//go:build !(linux || windows || darwin)

package tun

import (
	"os"
)

func Open(config Options) (Tun, error) {
	return nil, os.ErrInvalid
}
