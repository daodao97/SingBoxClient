// go-libtor - Self-contained Tor from Go
// Copyright (c) 2018 Péter Szilágyi. All rights reserved.
// +build darwin,amd64 darwin,arm64 ios,amd64 ios,arm64

package libtor

/*
#cgo CFLAGS: -I${SRCDIR}/../openssl_config
#cgo CFLAGS: -I${SRCDIR}/../darwin/openssl
#cgo CFLAGS: -I${SRCDIR}/../darwin/openssl/include
#cgo CFLAGS: -I${SRCDIR}/../darwin/openssl/crypto/ec/curve448
#cgo CFLAGS: -I${SRCDIR}/../darwin/openssl/crypto/ec/curve448/arch_32
#cgo CFLAGS: -I${SRCDIR}/../darwin/openssl/crypto/modes
*/
import "C"
