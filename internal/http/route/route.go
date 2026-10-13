// Package route provides helpers for registering routes on an http.ServeMux.
package route

import "net/http"

// HandleSubtree registers h for pattern and its subtree.
func HandleSubtree(mux *http.ServeMux, pattern string, h http.Handler) {
	mux.Handle(pattern, h)
	mux.Handle(pattern+"/", h)
}
