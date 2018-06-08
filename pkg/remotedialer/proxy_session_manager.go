package remotedialer

import (
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

type proxySessionManager struct {
	leaderBaseURL string
	sync.Mutex
	sessions map[string][]*proxySession
}

func (sm *proxySessionManager) getByClient(clientKey string) (*proxySession, error) {
	sm.Lock()
	defer sm.Unlock()
	sessions := sm.sessions[clientKey]
	if len(sessions) > 0 {
		return sessions[0], nil
	}

	return nil, fmt.Errorf("failed to find session for client %s", clientKey)
}

func (sm *proxySessionManager) add(clientKey string, target *session, conn *websocket.Conn) *proxySession {
	sessionKey := rand.Int63()
	session := newProxySession(sessionKey, clientKey, conn, target)
	session.sessionKey = sessionKey

	sm.Lock()
	defer sm.Unlock()

	sm.sessions[clientKey] = append(sm.sessions[clientKey], session)

	return session
}

func (sm *proxySessionManager) clientAdd(clusterName string, headers http.Header, dialer *websocket.Dialer) (*proxySession, error) {
	sessionKey := rand.Int63()

	if dialer == nil {
		dialer = &websocket.Dialer{}
	}
	ws, _, err := dialer.Dial(sm.leaderBaseURL, headers)
	if err != nil {
		logrus.Debug(err.Error())
		logrus.WithError(err).Error("Failed to connect to proxy")
		return nil, err
	}

	session := newProxyClientSession(clusterName, ws)
	session.sessionKey = sessionKey

	sm.Lock()
	defer sm.Unlock()

	sm.sessions[clusterName] = append(sm.sessions[clusterName], session)

	return session, nil
}

func (sm *proxySessionManager) remove(s *proxySession) {
	sm.Lock()
	defer sm.Unlock()

	var newSessions []*proxySession

	for _, v := range sm.sessions[s.clientKey] {
		if v.sessionKey == s.sessionKey {
			continue
		}
		newSessions = append(newSessions, v)
	}

	if len(newSessions) == 0 {
		delete(sm.sessions, s.clientKey)
	} else {
		sm.sessions[s.clientKey] = newSessions
	}

	s.Close()
}

func newProxySessionManager() *proxySessionManager {
	return &proxySessionManager{
		// leaderBaseURL: LeaderBaseURL,
		sessions: map[string][]*proxySession{},
	}
}

func (sm *proxySessionManager) onNewLeader(LeaderBaseURL string, isLeader bool) {
	sm.Lock()
	defer sm.Unlock()

	sm.leaderBaseURL = LeaderBaseURL
	//close the connects that we already have
	for _, ss := range sm.sessions {
		for _, s := range ss {
			s.Close()
		}
	}
	sm.sessions = map[string][]*proxySession{}
}

func (sm *proxySessionManager) onTargetSessionRemove(s *session) {
	sm.Lock()
	defer sm.Unlock()
	for clientKey, ss := range sm.sessions {
		if strings.HasSuffix(clientKey, s.clientKey) {
			for _, s := range ss {
				s.Close()
				s.conn.conn.WriteControl(websocket.CloseMessage, nil, time.Now().Add(5*time.Second))
			}
		}
	}
}
