package responsewriter

import (
	"net/http"
	"strings"

	"github.com/gorilla/mux"
)

func CacheMiddleware(suffixes ...string) mux.MiddlewareFunc {
	return mux.MiddlewareFunc(func(handler http.Handler) http.Handler {
		return Cache(handler, suffixes...)
	})
}

func Cache(handler http.Handler, suffixes ...string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		i := strings.LastIndex(r.URL.Path, ".")
		if i >= 0 {
			for _, suffix := range suffixes {
				if suffix == r.URL.Path[i+1:] {
					w.Header().Set("Cache-Control", "max-age=31536000, public")
				}
			}
		}
		handler.ServeHTTP(w, r)
	})
}

func NoCache(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		handler.ServeHTTP(w, r)
	})
}

func DenyFrameOptions(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "deny")
		handler.ServeHTTP(w, r)
	})
}

func ContentTypeOptions(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		handler.ServeHTTP(w, r)
	})
}
