package clashapi

import (
	"context"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/badjson"
	"github.com/sagernet/sing/common"
	F "github.com/sagernet/sing/common/format"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

func proxyProviderRouter(server *Server, router adapter.Router) http.Handler {
	r := chi.NewRouter()
	r.Get("/", getProviders(server, router))

	r.Route("/{name}", func(r chi.Router) {
		r.Use(parseProviderName, findProviderByName)
		r.Get("/", getProvider)
		r.Put("/", updateProvider)
		r.Get("/healthcheck", healthCheckProvider)
	})
	return r
}

func getProviders(server *Server, router adapter.Router) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		m := render.M{}
		outbounds := common.Filter(router.Outbounds(), func(detour adapter.Outbound) bool {
			return detour.Tag() != "" && detour.Type() == "provider"
		})

		for i, detour := range outbounds {
			var tag string
			if detour.Tag() == "" {
				tag = F.ToString(i)
			} else {
				tag = detour.Tag()
			}
			m[tag] = providerInfo(server, detour)
			//if i == 0 {
			//	m["default"] = m[tag]
			//}
		}

		render.JSON(w, r, render.M{
			"providers": m,
		})
	}
}

func providerInfo(server *Server, detour adapter.Outbound) *badjson.JSONObject {
	var info badjson.JSONObject
	info.Put("name", detour.Tag())

	var proxies []*badjson.JSONObject
	if d, ok := detour.(adapter.ProxyProvider); ok {
		all := d.AllOutbound()
		for _, v := range all {
			proxies = append(proxies, proxyInfo(server, v))
		}
	}

	info.Put("proxies", proxies)
	info.Put("type", "Proxy")
	info.Put("vehicleType", "HTTP")
	info.Put("updatedAt", time.Now())
	return &info
}

func getProvider(w http.ResponseWriter, r *http.Request) {
	/*provider := r.Context().Value(CtxKeyProvider).(provider.ProxyProvider)
	render.JSON(w, r, provider)*/
	render.NoContent(w, r)
}

func updateProvider(w http.ResponseWriter, r *http.Request) {
	/*provider := r.Context().Value(CtxKeyProvider).(provider.ProxyProvider)
	if err := provider.Update(); err != nil {
		render.Status(r, http.StatusServiceUnavailable)
		render.JSON(w, r, newError(err.Error()))
		return
	}*/
	render.NoContent(w, r)
}

func healthCheckProvider(w http.ResponseWriter, r *http.Request) {
	/*provider := r.Context().Value(CtxKeyProvider).(provider.ProxyProvider)
	provider.HealthCheck()*/
	render.NoContent(w, r)
}

func parseProviderName(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := getEscapeParam(r, "name")
		ctx := context.WithValue(r.Context(), CtxKeyProviderName, name)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func findProviderByName(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		/*name := r.Context().Value(CtxKeyProviderName).(string)
		providers := tunnel.ProxyProviders()
		provider, exist := providers[name]
		if !exist {*/
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, ErrNotFound)
		//return
		//}

		// ctx := context.WithValue(r.Context(), CtxKeyProvider, provider)
		// next.ServeHTTP(w, r.WithContext(ctx))
	})
}
