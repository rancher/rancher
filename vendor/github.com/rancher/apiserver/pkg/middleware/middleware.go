package middleware

import (
	"net/http"

	"github.com/gorilla/mux"
)

type Chain []mux.MiddlewareFunc

func (m Chain) Handler(handler http.Handler) http.Handler {
	rtn := handler
	for i := len(m) - 1; i >= 0; i-- {
		w := m[i]
		rtn = w(rtn)
	}
	return rtn
}
