package responsewriter

import (
	"net/http"

	"github.com/gorilla/mux"
)

type MiddlewareChain struct {
	middleWares []mux.MiddlewareFunc
}

func NewMiddlewareChain(middleWares ...mux.MiddlewareFunc) *MiddlewareChain {
	return &MiddlewareChain{middleWares: middleWares}
}

func (m *MiddlewareChain) Handler(handler http.Handler) http.Handler {
	rtn := handler
	for i := len(m.middleWares) - 1; i >= 0; i-- {
		w := m.middleWares[i]
		rtn = w.Middleware(rtn)
	}
	return rtn
}
