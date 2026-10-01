package tunnelserver

import (
	"bufio"
	"context"
	"errors"
	"net"
	"net/http"
	"sync"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/types"
)

// SessionTracker records the connections behind the tunnel sessions of downstream clusters, so that the
// sessions of a cluster can be ended once the cluster is gone. remotedialer only ends a session when its
// connection drops, and the session of a deleted cluster would otherwise keep serving the cluster's name,
// including for a new cluster created under the same name.
//
// Each replica only tracks the connections made to it.
type SessionTracker struct {
	mu    sync.Mutex
	conns map[types.UID]map[*trackedConn]struct{}
}

func NewSessionTracker() *SessionTracker {
	return &SessionTracker{conns: map[types.UID]map[*trackedConn]struct{}{}}
}

// sessionIdentity is filled in by an authorizer with the cluster a tunnel request was authorized for.
type sessionIdentity struct {
	mu          sync.Mutex
	clusterName string
	clusterUID  types.UID
}

type sessionIdentityKey struct{}

// SetSessionCluster records that req, a tunnel request, was authorized for the cluster with the given name
// and UID. Its session is then ended when that cluster is gone. It does nothing for requests that didn't
// come through a SessionTracker's Handler.
func SetSessionCluster(req *http.Request, name string, uid types.UID) {
	identity, ok := req.Context().Value(sessionIdentityKey{}).(*sessionIdentity)
	if !ok || uid == "" {
		return
	}
	identity.mu.Lock()
	defer identity.mu.Unlock()
	identity.clusterName = name
	identity.clusterUID = uid
}

// SessionCluster returns the cluster recorded for req by SetSessionCluster, if any.
func SessionCluster(req *http.Request) (string, types.UID) {
	identity, ok := req.Context().Value(sessionIdentityKey{}).(*sessionIdentity)
	if !ok {
		return "", ""
	}
	return identity.get()
}

func (i *sessionIdentity) get() (string, types.UID) {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.clusterName, i.clusterUID
}

type trackedConn struct {
	net.Conn
	clusterName string
	clusterUID  types.UID
}

// Handler wraps next, the tunnel endpoint, so that the connections of the sessions it authorizes for a
// cluster are tracked for as long as next serves them. A nil tracker tracks nothing.
func (t *SessionTracker) Handler(next http.Handler) http.Handler {
	if t == nil {
		return next
	}
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		identity := &sessionIdentity{}
		req = req.WithContext(context.WithValue(req.Context(), sessionIdentityKey{}, identity))

		w := &hijackRecorder{ResponseWriter: rw, identity: identity, tracker: t}
		defer func() {
			if w.conn != nil {
				t.remove(w.conn)
			}
		}()
		next.ServeHTTP(w, req)
	})
}

// CloseCluster ends the tunnel sessions this replica serves for the cluster with the given UID, and
// returns how many it ended.
func (t *SessionTracker) CloseCluster(uid types.UID) int {
	if t == nil {
		return 0
	}
	t.mu.Lock()
	conns := t.conns[uid]
	delete(t.conns, uid)
	t.mu.Unlock()

	name := ""
	for conn := range conns {
		name = conn.clusterName
		if err := conn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			logrus.Debugf("[tunnel-sessions] failed to close a tunnel session of cluster %s: %v", conn.clusterName, err)
		}
	}
	if len(conns) > 0 {
		logrus.Infof("[tunnel-sessions] closed %d tunnel sessions of removed cluster %s (uid %s)", len(conns), name, uid)
	}
	return len(conns)
}

func (t *SessionTracker) add(conn *trackedConn) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.conns[conn.clusterUID] == nil {
		t.conns[conn.clusterUID] = map[*trackedConn]struct{}{}
	}
	t.conns[conn.clusterUID][conn] = struct{}{}
}

func (t *SessionTracker) remove(conn *trackedConn) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.conns[conn.clusterUID], conn)
	if len(t.conns[conn.clusterUID]) == 0 {
		delete(t.conns, conn.clusterUID)
	}
}

// hijackRecorder records the connection a websocket upgrade takes over, if the request was authorized for
// a cluster by then.
type hijackRecorder struct {
	http.ResponseWriter
	identity *sessionIdentity
	tracker  *SessionTracker
	conn     *trackedConn
}

func (w *hijackRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("the response writer does not support hijacking")
	}
	conn, rw, err := hijacker.Hijack()
	if err != nil {
		return conn, rw, err
	}
	name, uid := w.identity.get()
	if uid == "" {
		return conn, rw, nil
	}
	w.conn = &trackedConn{Conn: conn, clusterName: name, clusterUID: uid}
	w.tracker.add(w.conn)
	return conn, rw, nil
}

// Unwrap lets http.ResponseController reach the underlying response writer.
func (w *hijackRecorder) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
