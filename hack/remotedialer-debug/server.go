package remotedialer

import (
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var (
	errFailedAuth       = errors.New("failed authentication")
	errWrongMessageType = errors.New("wrong websocket message type")
)

type Authorizer func(req *http.Request) (clientKey string, authed bool, err error)
type ErrorWriter func(rw http.ResponseWriter, req *http.Request, code int, err error)

func DefaultErrorWriter(rw http.ResponseWriter, req *http.Request, code int, err error) {
	rw.WriteHeader(code)
	rw.Write([]byte(err.Error()))
}

type Server struct {
	PeerID                  string
	PeerToken               string
	ClientConnectAuthorizer ConnectAuthorizer
	authorizer              Authorizer
	errorWriter             ErrorWriter
	sessions                *sessionManager
	peers                   map[string]peer
	peerLock                sync.Mutex
}

func New(auth Authorizer, errorWriter ErrorWriter) *Server {
	return &Server{
		peers:       map[string]peer{},
		authorizer:  auth,
		errorWriter: errorWriter,
		sessions:    newSessionManager(),
	}
}

func (s *Server) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	logrus.Debugf("🔍 REMOTEDIALER: HTTP request received: %s %s", req.Method, req.URL.Path)

	clientKey, authed, peer, err := s.auth(req)
	if err != nil {
		logrus.Errorf("🔍 REMOTEDIALER: Authentication failed: %v", err)
		s.errorWriter(rw, req, 400, err)
		return
	}
	if !authed {
		logrus.Errorf("🔍 REMOTEDIALER: Authentication denied for client: %s", clientKey)
		s.errorWriter(rw, req, 401, errFailedAuth)
		return
	}

	logrus.Infof("🔍 REMOTEDIALER: Handling backend connection request [%s] (peer: %v)", clientKey, peer)

	upgrader := websocket.Upgrader{
		HandshakeTimeout: 5 * time.Second,
		CheckOrigin:      func(r *http.Request) bool { return true },
		Error:            s.errorWriter,
	}

	wsConn, err := upgrader.Upgrade(rw, req, nil)
	if err != nil {
		logrus.Errorf("🔍 REMOTEDIALER: WebSocket upgrade failed for client [%s]: %v", clientKey, err)
		s.errorWriter(rw, req, 400, errors.Wrapf(err, "Error during upgrade for host [%v]", clientKey))
		return
	}

	logrus.Infof("🔍 REMOTEDIALER: WebSocket connection established for client [%s]", clientKey)

	session := s.sessions.add(clientKey, wsConn, peer)
	session.auth = s.ClientConnectAuthorizer
	defer func() {
		logrus.Infof("🔍 REMOTEDIALER: Removing session for client [%s]", clientKey)
		s.sessions.remove(session)
	}()

	code, err := session.Serve(req.Context())
	if err != nil {
		// Hijacked so we can't write to the client
		logrus.Infof("🔍 REMOTEDIALER: Session error for client [%s]: code=%d, error=%v", clientKey, code, err)
	} else {
		logrus.Infof("🔍 REMOTEDIALER: Session completed normally for client [%s]", clientKey)
	}
}

func (s *Server) ListClients() []string {
	return s.sessions.listClients()
}

func (s *Server) auth(req *http.Request) (clientKey string, authed, peer bool, err error) {
	id := req.Header.Get(ID)
	token := req.Header.Get(Token)
	if id != "" && token != "" {
		// peer authentication
		s.peerLock.Lock()
		p, ok := s.peers[id]
		s.peerLock.Unlock()

		if ok && p.token == token {
			return id, true, true, nil
		}
	}

	id, authed, err = s.authorizer(req)
	return id, authed, false, err
}
