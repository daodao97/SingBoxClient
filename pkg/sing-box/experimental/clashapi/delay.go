package clashapi

import (
	"context"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/urltest"
	"sync"
	"time"
)

var DELAY = newDelay()

func newDelay() *delay {
	return &delay{}
}

type delay struct {
	maps sync.Map
}

func (d *delay) run(detour adapter.Outbound) {
	d.maps.Store(detour, uint16(0))
	go func() {
		for i := 0; i < 10; i++ {
			ts, err := urltest.URLTest(context.Background(), "http://www.gstatic.com/generate_204", detour)
			if err != nil {
				ts = uint16(0)
			}
			d.maps.Store(detour, ts)
			time.Sleep(time.Minute)
		}
		d.maps.Delete(detour)
	}()
}

func (d *delay) get(detour adapter.Outbound) uint16 {
	val, ok := d.maps.Load(detour)
	if !ok {
		d.run(detour)
		return uint16(0)
	}
	return val.(uint16)
}
