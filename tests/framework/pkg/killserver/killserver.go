package killserver

import (
	"context"
	"net/http"

	"github.com/sirupsen/logrus"
)

const (
	Port = ":19999"
)

// KillServer is struct used to cancel a context of web service, that listens on a specific port.
type KillServer struct {
	Server http.Server
	cancel context.CancelFunc
}

// NewKillServer initializes a KillServer at a specific address/port and the cancel context of said web service.
func NewKillServer(addr string, cancel context.CancelFunc) *KillServer {
	return &KillServer{
		Server: http.Server{
			Addr: addr,
		},
		cancel: cancel,
	}
}

// Start starts the ListenAndServe of the KillServer server
func (s *KillServer) Start() {
	s.Server.Handler = s

	err := s.Server.ListenAndServe()
	if err != nil {
		logrus.Errorf("KillServer error: %v", err)
	}
}

// ServeHTTP should write reply headers and data to the ResponseWriter
// and then return.
func (s *KillServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)

	// cancel the context
	s.cancel()
}
