package remotedialer

import (
	"net"
	"time"
)

func (s *Server) Dial(clientKey string, deadline time.Duration, proto, address string) (net.Conn, error) {
	session, err := s.sessions.getByClient(clientKey)
	if err != nil {
		return nil, err
	}

	return session.serverConnect(deadline, proto, address)
}

func (s *Server) Dialer(clientKey string, deadline time.Duration) Dialer {
	return func(proto, address string) (net.Conn, error) {
		return s.Dial(clientKey, deadline, proto, address)
	}
}
