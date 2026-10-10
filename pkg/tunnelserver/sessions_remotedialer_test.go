//go:build !race

// The remotedialer client races with itself when a session ends: startPings sets the session's ping
// cancel func from the serving goroutine while Close reads it from the connecting one (remotedialer
// v0.6.1, session.go). This test ends client sessions on purpose, so it can't run with the race detector.

package tunnelserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rancher/remotedialer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"
)

// TestCloseClusterEndsRemotedialerSessions checks the whole path against a real remotedialer server: two
// agents connect under the same cluster name for different clusters, and only the removed cluster's
// session ends.
func TestCloseClusterEndsRemotedialerSessions(t *testing.T) {
	tracker := NewSessionTracker()
	tunnel := remotedialer.New(func(req *http.Request) (string, bool, error) {
		SetSessionCluster(req, "c-m-test", types.UID(req.Header.Get("X-Test-Cluster-UID")))
		return "c-m-test", true, nil
	}, remotedialer.DefaultErrorWriter)
	server := httptest.NewServer(tracker.Handler(tunnel))
	defer server.Close()
	url := "ws" + strings.TrimPrefix(server.URL, "http")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wg sync.WaitGroup
	connect := func(uid string) <-chan struct{} {
		done := make(chan struct{})
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer close(done)
			headers := http.Header{"X-Test-Cluster-UID": []string{uid}}
			_ = remotedialer.ConnectToProxy(ctx, url, headers, func(string, string) bool { return true }, nil, nil)
		}()
		return done
	}

	oldAgent := connect("uid-1")
	require.Eventually(t, func() bool { return trackedCount(tracker, "uid-1") == 1 }, 5*time.Second, 10*time.Millisecond)
	newAgent := connect("uid-2")
	require.Eventually(t, func() bool { return trackedCount(tracker, "uid-2") == 1 }, 5*time.Second, 10*time.Millisecond)

	assert.Equal(t, 1, tracker.CloseCluster("uid-1"))

	select {
	case <-oldAgent:
	case <-time.After(5 * time.Second):
		t.Fatal("the removed cluster's agent should have been disconnected")
	}
	select {
	case <-newAgent:
		t.Fatal("the new cluster's agent should still be connected")
	case <-time.After(200 * time.Millisecond):
	}
	assert.True(t, tunnel.HasSession("c-m-test"), "the new cluster's session should still serve the name")

	cancel()
	wg.Wait()
}
