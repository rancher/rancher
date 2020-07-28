package health

import (
	"net/http"

	"github.com/gorilla/mux"
	"k8s.io/apiserver/pkg/server/healthz"
)

func Register(router *mux.Router) {
	healthz.InstallHandler((*muxWrapper)(router))
	router.Handle("/ping", Pong())
}

func Pong() http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Write([]byte("pong"))
	})
}

type muxWrapper mux.Router

func (m *muxWrapper) Handle(path string, handler http.Handler) {
	(*mux.Router)(m).Handle(path, handler)
}
