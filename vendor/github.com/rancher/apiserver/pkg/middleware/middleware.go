package middleware

import (
	"net/http"

	"github.com/gorilla/mux"
)

type Chain struct {
	middleWares []mux.MiddlewareFunc
}

func NewMiddlewareChain(middleWares ...mux.MiddlewareFunc) *Chain {
	return &Chain{middleWares: middleWares}
}

func (m *Chain) Handler(handler http.Handler) http.Handler {
	rtn := handler
	for i := len(m.middleWares) - 1; i >= 0; i-- {
		w := m.middleWares[i]
		rtn = w.Middleware(rtn)
	}
	return rtn
}
