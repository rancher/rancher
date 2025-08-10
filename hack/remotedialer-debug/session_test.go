package remotedialer

import (
	"math/rand"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var dummyConnectionsNextID int64 = 1

func getDummyConnectionID() int64 {
	return atomic.AddInt64(&dummyConnectionsNextID, 1)
}

func setupDummySession(t *testing.T, nConnections int) *Session {
	t.Helper()

	s := newSession(rand.Int63(), "", nil)

	var wg sync.WaitGroup
	ready := make(chan struct{})
	for i := 0; i < nConnections; i++ {
		connID := getDummyConnectionID()
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-ready
			s.addConnection(connID, &connection{})
		}()
	}
	close(ready)
	wg.Wait()

	if got, want := len(s.conns), nConnections; got != want {
		t.Fatalf("incorrect number of connections, got: %d, want %d", got, want)
	}

	return s
}

func TestSession_connections(t *testing.T) {
	t.Parallel()

	const n = 10
	s := setupDummySession(t, n)

	connID, conn := getDummyConnectionID(), &connection{}
	s.addConnection(connID, conn)
	if got, want := len(s.conns), n+1; got != want {
		t.Errorf("incorrect number of connections, got: %d, want %d", got, want)
	}
	if got, want := s.getConnection(connID), conn; got != want {
		t.Errorf("incorrect result from getConnection, got: %v, want %v", got, want)
	}
	if got, want := s.removeConnection(connID), conn; got != want {
		t.Errorf("incorrect result from removeConnection, got: %v, want %v", got, want)
	}
}

func TestSession_sessionKeys(t *testing.T) {
	t.Parallel()

	s := setupDummySession(t, 0)

	clientKey, sessionKey := "testkey", rand.Int()
	s.addSessionKey(clientKey, sessionKey)
	if got, want := len(s.remoteClientKeys), 1; got != want {
		t.Errorf("incorrect number of remote client keys, got: %d, want %d", got, want)
	}

	if got, want := s.getSessionKeys(clientKey), map[int]bool{sessionKey: true}; !reflect.DeepEqual(got, want) {
		t.Errorf("incorrect result from getSessionKeys, got: %v, want %v", got, want)
	}

	s.removeSessionKey(clientKey, sessionKey)
	if got, want := len(s.remoteClientKeys), 0; got != want {
		t.Errorf("incorrect number of remote client keys after removal, got: %d, want %d", got, want)
	}
}

func TestSession_activeConnectionIDs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		conns    map[int64]*connection
		expected []int64
	}{
		{
			name:     "no connections",
			conns:    map[int64]*connection{},
			expected: []int64{},
		},
		{
			name: "single",
			conns: map[int64]*connection{
				1234: nil,
			},
			expected: []int64{1234},
		},
		{
			name: "multiple connections",
			conns: map[int64]*connection{
				5:  nil,
				20: nil,
				3:  nil,
			},
			expected: []int64{3, 5, 20},
		},
	}
	for x := range tests {
		tt := tests[x]
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			session := Session{conns: tt.conns}
			if got, want := session.activeConnectionIDs(), tt.expected; !reflect.DeepEqual(got, want) {
				t.Errorf("incorrect result, got: %v, want: %v", got, want)
			}
		})
	}
}

func TestSession_sendPings(t *testing.T) {
	t.Parallel()

	conn := testServerWS(t, nil)
	session := newSession(rand.Int63(), "pings-test", newWSConn(conn))

	pongHandler := conn.PongHandler()

	pongs := make(chan struct{})
	conn.SetPongHandler(func(appData string) error {
		pongs <- struct{}{}
		return pongHandler(appData)
	})
	go func() {
		// Read channel must be consumed (even if discarded) for control messages to work:
		// https://pkg.go.dev/github.com/gorilla/websocket#hdr-Control_Messages
		for {
			if _, _, err := conn.NextReader(); err != nil {
				return
			}
		}
	}()

	for i := 1; i <= 4; i++ {
		if err := session.sendPing(); err != nil {
			t.Fatal(err)
		}
		select {
		// pong received, ping was successful
		case <-pongs:
		// High timeout on purpose to avoid flakiness
		case <-time.After(5 * time.Second):
			t.Errorf("ping %d not received in time", i)
		}
	}
}
