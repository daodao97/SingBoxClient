//go:build !with_gvisor

package tun

import "github.com/sagernet/sing/common/buf"

type DirectDestination interface {
	WritePacket(buffer *buf.Buffer) error
	Close() error
	Timeout() bool
}
