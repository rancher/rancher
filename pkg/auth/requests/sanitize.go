package requests

import (
	"net/http"
	"strings"
)

func NewHeaderSanitizer(h http.Handler) http.Handler {
	return &sanitizeHeaderHandler{
		handler: h,
	}
}

type sanitizeHeaderHandler struct {
	handler http.Handler
}

func (h sanitizeHeaderHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	connHeader := req.Header.Get("Connection")
	headers := strings.Split(connHeader, ",")
	cleanHeaders := make([]string, 0, len(headers))
	for _, h := range headers {
		if !strings.HasPrefix(strings.ToLower(h), "impersonate-") {
			cleanHeaders = append(cleanHeaders, h)
		}
	}
	if len(cleanHeaders) > 0 {
		req.Header.Set("Connection", strings.Join(cleanHeaders, ","))
	} else {
		req.Header.Del("Connection")
	}
	h.handler.ServeHTTP(rw, req)
}
