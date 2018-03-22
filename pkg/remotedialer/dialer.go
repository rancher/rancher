package remotedialer

import (
	"net"
	"time"

	"github.com/rancher/types/config/dialer"
)

func (s *Server) HasSession(clientKey string) bool {
	_, err := s.sessions.getByClient(clientKey)
	return err == nil
}

func (s *Server) Dial(clientKey string, deadline time.Duration, proto, address string) (net.Conn, error) {
	session, err := s.sessions.getByClient(clientKey)
	if err != nil {
		return nil, err
	}

	return session.serverConnect(deadline, proto, address)
}

func (s *Server) Dialer(clientKey string, deadline time.Duration) dialer.Dialer {
	return func(proto, address string) (net.Conn, error) {
		return s.Dial(clientKey, deadline, proto, address)
	}
}
