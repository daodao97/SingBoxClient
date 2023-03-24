package tun

import (
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/cache"
)

type RouteMapping struct {
	status *cache.LruCache[RouteSession, RouteAction]
}

func NewRouteMapping(maxAge int64) *RouteMapping {
	return &RouteMapping{
		status: cache.New(
			cache.WithAge[RouteSession, RouteAction](maxAge),
			cache.WithUpdateAgeOnGet[RouteSession, RouteAction](),
			cache.WithEvict[RouteSession, RouteAction](func(key RouteSession, conn RouteAction) {
				common.Close(conn)
			}),
		),
	}
}

func (m *RouteMapping) Lookup(session RouteSession, constructor func() RouteAction) RouteAction {
	action, _ := m.status.LoadOrStore(session, constructor)
	if action.Timeout() {
		common.Close(action)
		action = constructor()
		m.status.Store(session, action)
	}
	return action
}
