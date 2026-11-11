package main

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"runtime/pprof"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rancher/remotedialer"
	"github.com/sirupsen/logrus"
)

const (
	tunnelConnecting   = "connecting"
	tunnelConnected    = "connected"
	tunnelDisconnected = "disconnected"

	// The client pings every remotedialer.PingWriteInterval and the server
	// pongs, so a healthy tunnel never goes this long without a read.
	tunnelReadStall = 30 * time.Second

	tunnelWatchInterval = 15 * time.Second
)

// tunnelMonitor logs each transition of the remotedialer tunnel and warns
// when the tunnel stops making progress without the library noticing. With
// CATTLE_TUNNEL_DEBUG=true it also dumps goroutines once per stall so the
// blocked call shows up in the agent log.
type tunnelMonitor struct {
	mu         sync.Mutex
	state      string
	since      time.Time
	attempt    int
	sessions   int
	url        string
	conn       *tappedConn
	maxBackoff time.Duration
	dumped     bool
	dumpStacks bool
}

func newTunnelMonitor(backoff remotedialer.Backoff) *tunnelMonitor {
	// Longest sleep between attempts, plus jitter and a handshake.
	wait := max(backoff.Min, backoff.Max, remotedialer.DefaultRetryMin)
	return &tunnelMonitor{
		state:      tunnelDisconnected,
		since:      time.Now(),
		maxBackoff: wait + wait/10 + remotedialer.HandshakeTimeOut,
		dumpStacks: os.Getenv("CATTLE_TUNNEL_DEBUG") == "true",
	}
}

// attemptStarted is called from the URL func, which remotedialer consults
// before every connection attempt.
func (m *tunnelMonitor) attemptStarted(url string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.attempt++
	logrus.Infof("[tunnel] attempt %d to %s starting, was %s for %s",
		m.attempt, url, m.state, time.Since(m.since).Round(time.Millisecond))
	m.setState(tunnelConnecting)
	m.url = url
	m.conn = nil
}

func (m *tunnelMonitor) dialed(c *tappedConn) {
	m.mu.Lock()
	defer m.mu.Unlock()

	logrus.Infof("[tunnel] attempt %d dialed %s -> %s", m.attempt, c.LocalAddr(), c.RemoteAddr())
	m.conn = c
}

// attemptFailed moves a connecting attempt to disconnected so the backoff
// sleep that follows is not reported as a stuck handshake. c is nil when the
// TCP dial itself failed.
func (m *tunnelMonitor) attemptFailed(c *tappedConn) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state != tunnelConnecting || m.conn != c {
		return
	}
	logrus.Warnf("[tunnel] attempt %d failed after %s (%s)",
		m.attempt, time.Since(m.since).Round(time.Millisecond), c.describe())
	m.setState(tunnelDisconnected)
}

// connected is called from onConnect. The context is cancelled when
// ConnectToProxy returns, after the session has been torn down.
func (m *tunnelMonitor) connected(ctx context.Context) {
	m.mu.Lock()
	m.sessions++
	session, attempt, conn := m.sessions, m.attempt, m.conn
	logrus.Infof("[tunnel] session %d established on attempt %d (%s)", session, attempt, conn.describe())
	m.setState(tunnelConnected)
	start := m.since
	m.mu.Unlock()

	go func() {
		<-ctx.Done()
		m.mu.Lock()
		defer m.mu.Unlock()
		logrus.Warnf("[tunnel] session %d ended after %s (%s)",
			session, time.Since(start).Round(time.Millisecond), conn.describe())
		// A later attempt may already own the state.
		if m.sessions == session && m.state == tunnelConnected {
			m.setState(tunnelDisconnected)
		}
	}()
}

func (m *tunnelMonitor) setState(state string) {
	m.state = state
	m.since = time.Now()
	m.dumped = false
}

func (m *tunnelMonitor) dialer() *websocket.Dialer {
	// Matches the default remotedialer uses when given a nil dialer.
	d := &net.Dialer{}
	return &websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: remotedialer.HandshakeTimeOut,
		NetDialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			c, err := d.DialContext(ctx, network, addr)
			if err != nil {
				logrus.Errorf("[tunnel] dial %s %s failed: %v", network, addr, err)
				m.attemptFailed(nil)
				return nil, err
			}
			// Handshake failures (TLS, HTTP upgrade) close the conn before
			// onConnect runs, which ends the attempt.
			var tc *tappedConn
			tc = newTappedConn(c, func() { m.attemptFailed(tc) })
			m.dialed(tc)
			return tc, nil
		},
	}
}

func (m *tunnelMonitor) watch(ctx context.Context) {
	t := time.NewTicker(tunnelWatchInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			m.check()
		}
	}
}

func (m *tunnelMonitor) check() {
	if m.checkLocked() {
		// Outside the lock so the dump doesn't show goroutines parked on it.
		var buf bytes.Buffer
		if err := pprof.Lookup("goroutine").WriteTo(&buf, 2); err != nil {
			logrus.Errorf("[tunnel] goroutine dump failed: %v", err)
			return
		}
		logrus.Warnf("[tunnel] goroutine dump:\n%s", buf.String())
	}
}

// checkLocked logs the tunnel state and reports whether a goroutine dump is due.
func (m *tunnelMonitor) checkLocked() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	inState := time.Since(m.since)
	var stall string
	switch m.state {
	case tunnelConnecting:
		if inState > remotedialer.HandshakeTimeOut+tunnelWatchInterval {
			stall = "attempt has not completed its handshake"
		}
	case tunnelDisconnected:
		if m.attempt > 0 && inState > m.maxBackoff {
			stall = "no reconnect attempt since the last attempt or session ended"
		}
	case tunnelConnected:
		switch {
		case m.conn == nil:
		case m.conn.failedFor() > tunnelWatchInterval:
			stall = "conn failed (" + m.conn.describe() + ") but the session was not torn down"
		case m.conn.idleFor() > tunnelReadStall:
			stall = "connected but nothing read from the server (" + m.conn.describe() + ")"
		}
	}

	if stall == "" {
		logrus.Debugf("[tunnel] %s for %s, attempt %d, %s",
			m.state, inState.Round(time.Second), m.attempt, m.conn.describe())
		return false
	}

	logrus.Warnf("[tunnel] possible stall: %s; %s for %s, attempt %d, url %s",
		stall, m.state, inState.Round(time.Second), m.attempt, m.url)
	if !m.dumpStacks || m.dumped {
		return false
	}
	m.dumped = true
	return true
}

// tappedConn records activity on the raw TCP connection under the tunnel's
// TLS and websocket layers, and logs the first read or write error, which is
// usually the earliest sign of why a tunnel died.
type tappedConn struct {
	net.Conn
	opened    time.Time
	lastRead  atomic.Int64
	bytesRead atomic.Int64
	failedAt  atomic.Int64
	closed    atomic.Bool
	onClose   func()
	closeOnce sync.Once
	errOnce   sync.Once
}

func newTappedConn(c net.Conn, onClose func()) *tappedConn {
	tc := &tappedConn{Conn: c, opened: time.Now(), onClose: onClose}
	tc.lastRead.Store(tc.opened.UnixNano())
	return tc
}

func (c *tappedConn) Read(b []byte) (int, error) {
	n, err := c.Conn.Read(b)
	if n > 0 {
		c.lastRead.Store(time.Now().UnixNano())
		c.bytesRead.Add(int64(n))
	}
	if err != nil {
		c.failed("read", err)
	}
	return n, err
}

func (c *tappedConn) Write(b []byte) (int, error) {
	n, err := c.Conn.Write(b)
	if err != nil {
		c.failed("write", err)
	}
	return n, err
}

func (c *tappedConn) Close() error {
	c.closeOnce.Do(func() {
		c.closed.Store(true)
		logrus.Infof("[tunnel] closing %s -> %s after %s",
			c.LocalAddr(), c.RemoteAddr(), time.Since(c.opened).Round(time.Millisecond))
		c.onClose()
	})
	return c.Conn.Close()
}

func (c *tappedConn) failed(op string, err error) {
	c.errOnce.Do(func() {
		c.failedAt.Store(time.Now().UnixNano())
		log, cause := logrus.Warnf, ""
		// Pending I/O fails once we close the conn ourselves; that is a
		// consequence of the teardown, not its cause.
		if c.closed.Load() {
			log, cause = logrus.Infof, " (after local close)"
		}
		log("[tunnel] %s on %s -> %s failed%s after %s, last read %s ago: %v",
			op, c.LocalAddr(), c.RemoteAddr(), cause, time.Since(c.opened).Round(time.Millisecond),
			c.idleFor().Round(time.Millisecond), err)
	})
}

func (c *tappedConn) idleFor() time.Duration {
	return time.Since(time.Unix(0, c.lastRead.Load()))
}

func (c *tappedConn) failedFor() time.Duration {
	at := c.failedAt.Load()
	if at == 0 {
		return 0
	}
	return time.Since(time.Unix(0, at))
}

func (c *tappedConn) describe() string {
	if c == nil {
		return "no conn"
	}
	return fmt.Sprintf("%s -> %s, read %d bytes, last read %s ago",
		c.LocalAddr(), c.RemoteAddr(), c.bytesRead.Load(), c.idleFor().Round(time.Second))
}
