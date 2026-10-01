package tunnelserver

import (
	"bufio"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"
)

// hijackableWriter is a response writer whose connection can be taken over, like the server's.
type hijackableWriter struct {
	*httptest.ResponseRecorder
	conn net.Conn
}

func (w *hijackableWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return w.conn, bufio.NewReadWriter(bufio.NewReader(w.conn), bufio.NewWriter(w.conn)), nil
}

// serveHijacking runs one request through the tracker's handler. The handler records uid as the
// request's cluster, unless it's empty, and takes over the connection, then waits for release before
// returning, which is what ends a session.
func serveHijacking(t *testing.T, tracker *SessionTracker, uid types.UID, release <-chan struct{}) (net.Conn, <-chan struct{}) {
	t.Helper()
	server, client := net.Pipe()
	t.Cleanup(func() { client.Close() })
	hijacked := make(chan struct{})
	done := make(chan struct{})
	handler := tracker.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if uid != "" {
			SetSessionCluster(r, "c-m-test", uid)
		}
		_, _, err := http.NewResponseController(w).Hijack()
		assert.NoError(t, err)
		close(hijacked)
		<-release
	}))
	go func() {
		defer close(done)
		handler.ServeHTTP(&hijackableWriter{ResponseRecorder: httptest.NewRecorder(), conn: server}, httptest.NewRequest(http.MethodGet, "/v3/connect", nil))
	}()
	<-hijacked
	return client, done
}

func trackedCount(tracker *SessionTracker, uid types.UID) int {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	return len(tracker.conns[uid])
}

func TestSessionTrackerTracksAConnectionForAsLongAsItIsServed(t *testing.T) {
	tracker := NewSessionTracker()
	release := make(chan struct{})

	_, done := serveHijacking(t, tracker, "uid-1", release)
	assert.Equal(t, 1, trackedCount(tracker, "uid-1"))

	close(release)
	<-done
	assert.Equal(t, 0, trackedCount(tracker, "uid-1"), "the connection should be forgotten once the session ends")
}

func TestSessionTrackerIgnoresRequestsNotAuthorizedForACluster(t *testing.T) {
	tracker := NewSessionTracker()
	release := make(chan struct{})
	defer close(release)

	serveHijacking(t, tracker, "", release)

	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	assert.Empty(t, tracker.conns)
}

func TestCloseClusterClosesOnlyThatClustersConnections(t *testing.T) {
	tracker := NewSessionTracker()
	release := make(chan struct{})
	defer close(release)
	oldClient, _ := serveHijacking(t, tracker, "uid-1", release)
	newClient, _ := serveHijacking(t, tracker, "uid-2", release)

	closed := tracker.CloseCluster("uid-1")

	assert.Equal(t, 1, closed)
	_, err := oldClient.Read(make([]byte, 1))
	assert.Error(t, err, "the removed cluster's connection should be closed")
	assert.Equal(t, 1, trackedCount(tracker, "uid-2"), "a new cluster with the same name should keep its session")
	require.NoError(t, newClient.SetReadDeadline(time.Now().Add(50*time.Millisecond)))
	_, err = newClient.Read(make([]byte, 1))
	var netErr net.Error
	require.ErrorAs(t, err, &netErr)
	assert.True(t, netErr.Timeout(), "the new cluster's connection should still be open")
	assert.Equal(t, 0, tracker.CloseCluster("uid-1"), "closing again finds nothing")
}

func TestNilSessionTrackerTracksNothing(t *testing.T) {
	var tracker *SessionTracker
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})

	assert.NotNil(t, tracker.Handler(next))
	assert.Equal(t, 0, tracker.CloseCluster("uid-1"))
}

func TestSessionClusterWithoutATracker(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v3/connect", nil)
	SetSessionCluster(req, "c-m-test", "uid-1")

	name, uid := SessionCluster(req)
	assert.Empty(t, name)
	assert.Empty(t, uid)
}
