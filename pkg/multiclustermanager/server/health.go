package server

import (
	"net/http"

	"github.com/gorilla/mux"
	"k8s.io/apiserver/pkg/server/healthz"
)

func registerHealth(router *mux.Router) {
	healthz.InstallHandler(muxWrapper{mux: router})
	router.Handle("/ping", http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		resp.Write([]byte("pong"))
	}))
}

type muxWrapper struct {
	mux *mux.Router
}

func (m muxWrapper) Handle(path string, handler http.Handler) {
	m.mux.Handle(path, handler)
}
