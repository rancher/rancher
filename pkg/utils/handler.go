package utils

import (
	"net/http"
)

// APIBodyLimitingHandler returns a middleware that can be applied to
// http.Handlers to restrict the number of bytes that can be read in a handler.
func APIBodyLimitingHandler(limit int64) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.MaxBytesHandler(h, limit)
	}
}
