//go:build !android

package tun

import "os"

func NewPackageManager(callback PackageManagerCallback) (PackageManager, error) {
	return nil, os.ErrInvalid
}
