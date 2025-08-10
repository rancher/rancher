package remotedialer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestSession_clientConnect(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	msgProto, msgAddr := "testproto", "testaddr"
	s := setupDummySession(t, 0)
	s.auth = func(proto, address string) bool { return proto == msgProto && address == msgAddr }

	dialerC := make(chan struct{})
	s.dialer = func(ctx context.Context, network, address string) (net.Conn, error) {
		close(dialerC)
		clientConn, _ := net.Pipe()
		return clientConn, nil
	}

	connID := getDummyConnectionID()
	if err := s.clientConnect(ctx, newConnect(connID, msgProto, msgAddr)); err != nil {
		t.Fatal(err)
	}

	select {
	case <-dialerC:
	case <-time.After(1 * time.Second):
		t.Errorf("timed out waiting for dialer")
	}

	if conn := s.getConnection(connID); conn == nil {
		t.Errorf("Connection not found in session for ID %d", connID)
	}
}

func TestSession_addRemoveRemoteClient(t *testing.T) {
	s := setupDummySession(t, 0)
	clientKey, sessionKey := "test", rand.Int()

	msgAddress := fmt.Sprintf("%s/%d", clientKey, sessionKey)
	if err := s.addRemoteClient(msgAddress); err != nil {
		t.Fatal(err)
	}

	if got, want := s.getSessionKeys(clientKey), map[int]bool{sessionKey: true}; !reflect.DeepEqual(got, want) {
		t.Errorf("remote client session was not added correctly, got %v, want %v", got, want)
	}

	if err := s.removeRemoteClient(msgAddress); err != nil {
		t.Fatal(err)
	}

	if got, want := s.getSessionKeys(clientKey), 0; len(got) != want {
		t.Errorf("remote client session was not removed correctly, got %v, want len(%d)", got, want)
	}
}

func TestSession_connectionData(t *testing.T) {
	s := setupDummySession(t, 0)
	connID := getDummyConnectionID()
	conn := newConnection(connID, s, "test", "test")
	s.addConnection(connID, conn)

	data := "testing!"
	s.connectionData(connID, strings.NewReader(data))

	if got, want := conn.buffer.offerCount, int64(len(data)); got != want {
		t.Errorf("incorrect data length, got %d, want %d", got, want)
	}

	buf := make([]byte, conn.buffer.offerCount)
	if _, err := conn.buffer.Read(buf); err != nil {
		t.Fatal(err)
	}
	if got, want := string(buf), data; got != want {
		t.Errorf("incorrect data, got %q, want %q", got, want)
	}
}

func TestSession_pauseResumeConnection(t *testing.T) {
	s := setupDummySession(t, 0)
	connID := getDummyConnectionID()
	conn := newConnection(connID, s, "test", "test")
	s.addConnection(connID, conn)

	s.pauseConnection(connID)
	if !conn.backPressure.paused {
		t.Errorf("connection was not paused correctly")
	}

	s.resumeConnection(connID)
	if conn.backPressure.paused {
		t.Errorf("connection was not resumed correctly")
	}
}

func TestSession_closeConnection(t *testing.T) {
	s := setupDummySession(t, 0)
	var msg *message
	s.conn = &fakeWSConn{
		writeMessageCallback: func(msgType int, deadline time.Time, data []byte) (err error) {
			if !deadline.IsZero() && deadline.Before(time.Now()) {
				return errors.New("deadline exceeded")
			}
			msg, err = newServerMessage(bytes.NewReader(data))
			return
		},
	}
	connID := getDummyConnectionID()
	conn := newConnection(connID, s, "test", "test")
	s.addConnection(connID, conn)

	// Ensure Error message is sent regardless of the WriteDeadline value, see https://github.com/rancher/remotedialer/pull/79
	_ = conn.SetWriteDeadline(time.Now())

	expectedErr := errors.New("connection closed")
	s.closeConnection(connID, expectedErr)

	if s.getConnection(connID) != nil {
		t.Errorf("connection was not closed correctly")
	}
	if conn.err == nil || msg == nil {
		t.Fatal("message not sent on closed connection")
	} else if msg.messageType != Error {
		t.Errorf("incorrect message type sent")
	} else if got, want := msg.Err().Error(), expectedErr.Error(); got != want {
		t.Errorf("wrong error, got %v, want %v", got, want)
	} else if got, want := conn.err, expectedErr; !errors.Is(got, want) {
		t.Errorf("wrong error, got %v, want %v", got, want)
	}
}
