//go:build with_gvisor

package tun

import (
	"time"

	gLog "gvisor.dev/gvisor/pkg/log"
)

func init() {
	gLog.SetTarget((*nopEmitter)(nil))
}

type nopEmitter struct{}

func (n *nopEmitter) Emit(depth int, level gLog.Level, timestamp time.Time, format string, v ...interface{}) {
}
