package responsewriter

import (
	"net/http"
	"strings"
)

type ContentTypeWriter struct {
	http.ResponseWriter
}

func (c ContentTypeWriter) Write(b []byte) (int, error) {
	found := false
	for k := range c.Header() {
		if strings.EqualFold(k, "Content-Type") {
			found = true
			break
		}
	}
	if !found {
		c.Header().Set("Content-Type", http.DetectContentType(b))
	}
	return c.ResponseWriter.Write(b)
}

func ContentType(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writer := ContentTypeWriter{ResponseWriter: w}
		handler.ServeHTTP(writer, r)
	})
}
