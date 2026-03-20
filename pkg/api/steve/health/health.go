package health

import (
	"net/http"

	"k8s.io/apiserver/pkg/server/healthz"
)

func Register(router *http.ServeMux) {
	healthz.InstallHandler((*muxWrapper)(router))
	router.Handle("/ping", Pong())
}

func Pong() http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Write([]byte("pong"))
	})
}

type muxWrapper http.ServeMux

func (m *muxWrapper) Handle(path string, handler http.Handler) {
	(*http.ServeMux)(m).Handle(path, handler)
}
