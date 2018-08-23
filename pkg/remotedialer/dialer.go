package remotedialer

import (
	"net"
	"time"

	"github.com/rancher/types/config/dialer"
)

func (s *Server) HasSession(clientKey string) bool {
	_, err := s.sessions.getDialer(clientKey, 0)
	return err == nil
}

func (s *Server) Dial(clientKey string, deadline time.Duration, proto, address string) (net.Conn, error) {
	d, err := s.sessions.getDialer(clientKey, deadline)
	if err != nil {
		return nil, err
	}

	return d(proto, address)
}

func (s *Server) Dialer(clientKey string, deadline time.Duration) dialer.Dialer {
	return func(proto, address string) (net.Conn, error) {
		return s.Dial(clientKey, deadline, proto, address)
	}
}
