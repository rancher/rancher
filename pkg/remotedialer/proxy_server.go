package remotedialer

import (
	"net"
	"net/http"
	"time"

	"github.com/rancher/types/config/dialer"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type ProxyServer struct {
	ready         func() bool
	authorizer    ProxyAuthorizer
	errorWriter   ErrorWriter
	proxySessions *proxySessionManager
	sessions      *sessionManager
}

func NewProxyServer(auth ProxyAuthorizer, errorWriter ErrorWriter, server *Server, ready func() bool) *ProxyServer {
	rtn := &ProxyServer{
		authorizer:    auth,
		errorWriter:   errorWriter,
		sessions:      server.sessions,
		proxySessions: newProxySessionManager(),
	}
	rtn.sessions.setRemoveFunc(rtn.proxySessions.onTargetSessionRemove)
	rtn.ready = func() bool {
		rtn.proxySessions.Lock()
		defer rtn.proxySessions.Unlock()
		return ready() && rtn.proxySessions.leaderBaseURL != ""
	}
	return rtn
}

func (s *ProxyServer) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if !s.ready() {
		s.errorWriter(rw, req, 503, errors.New("tunnel server not active"))
		return
	}

	clusterName, clientKey, authed, err := s.authorizer(req)
	if err != nil {
		s.errorWriter(rw, req, 400, err)
		return
	}
	if !authed {
		s.errorWriter(rw, req, 401, errFailedAuth)
		return
	}

	ses, err := s.sessions.getByClient(clusterName)
	if err != nil {
		s.errorWriter(rw, req, 400, err)
		return
	}

	logrus.Infof("Handling backend proxy connection request [%s] for [%s]", clientKey, clusterName)

	upgrader := websocket.Upgrader{
		HandshakeTimeout: 5 * time.Second,
		CheckOrigin:      func(r *http.Request) bool { return true },
		Error:            s.errorWriter,
	}

	wsConn, err := upgrader.Upgrade(rw, req, nil)
	if err != nil {
		s.errorWriter(rw, req, 400, errors.Wrapf(err, "Error during upgrade for host [%v]", clientKey))
		return
	}
	session := s.proxySessions.add(clientKey+"/"+clusterName, ses, wsConn)
	defer s.proxySessions.remove(session)

	// Don't need to associate req.Context() to the session, it will cancel otherwise
	code, err := session.serve()
	if err != nil {
		// Hijacked so we can't write to the client
		logrus.Debugf("error in remotedialer server [%d]: %v", code, err)
	}
}

func (s *ProxyServer) OnNewLeader(leaderURL string, isLeader bool) {
	s.proxySessions.onNewLeader(leaderURL, isLeader)
}

func (s *ProxyServer) HasSession(clientKey string) bool {
	_, err := s.proxySessions.getByClient(clientKey)
	return err == nil
}

func (s *ProxyServer) ClientDial(clientKey string, deadline time.Duration, headers http.Header, dialer *websocket.Dialer, proto, address string) (net.Conn, error) {
	session, err := s.proxySessions.getByClient(clientKey)
	if err != nil {
		session, err = s.proxySessions.clientAdd(clientKey, headers, dialer)
		if err != nil {
			return nil, err
		}
		go func() {
			_, err = session.serve()
			session.Close()
			s.proxySessions.remove(session)
		}()
	}

	return session.serverConnect(deadline, proto, address)
}

func (s *ProxyServer) ClientDialer(clientKey string, deadline time.Duration, headers http.Header, dialer *websocket.Dialer) dialer.Dialer {
	return func(proto, address string) (net.Conn, error) {
		return s.ClientDial(clientKey, deadline, headers, dialer, proto, address)
	}
}
