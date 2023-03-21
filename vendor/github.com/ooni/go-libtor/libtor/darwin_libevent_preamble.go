// go-libtor - Self-contained Tor from Go
// Copyright (c) 2018 Péter Szilágyi. All rights reserved.
// +build darwin,amd64 darwin,arm64 ios,amd64 ios,arm64

package libtor

/*
#cgo CFLAGS: -I${SRCDIR}/../libevent_config
#cgo CFLAGS: -I${SRCDIR}/../darwin/libevent
#cgo CFLAGS: -I${SRCDIR}/../darwin/libevent/compat
#cgo CFLAGS: -I${SRCDIR}/../darwin/libevent/include
*/
import "C"
