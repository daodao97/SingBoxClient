//go:build !linux
// +build !linux

package quic

import "errors"

func setConnReadBufferForce(c interface{}, size int) error {
	return errors.New("Unsupported on this platform")
}
