//go:build with_gvisor && linux

package tun

import (
	"github.com/sagernet/sing-tun/internal/fdbased"

	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

var _ GVisorTun = (*NativeTun)(nil)

func (t *NativeTun) NewEndpoint() (stack.LinkEndpoint, error) {
	return fdbased.New(&fdbased.Options{
		FDs: []int{t.tunFd},
		MTU: t.options.MTU,
	})
}
