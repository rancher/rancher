package remotedialer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

type session struct {
	sync.Mutex

	nextConnID int64
	clientKey  string
	sessionKey int64
	conn       *wsConn
	conns      map[int64]*connection
	auth       ConnectAuthorizer
	pingCancel context.CancelFunc
	pingWait   sync.WaitGroup
	client     bool
}

func newClientSession(auth ConnectAuthorizer, conn *websocket.Conn) *session {
	return &session{
		clientKey: "client",
		conn:      newWSConn(conn),
		conns:     map[int64]*connection{},
		auth:      auth,
		client:    true,
	}
}

func newSession(sessionKey int64, clientKey string, conn *websocket.Conn) *session {
	return &session{
		nextConnID: 1,
		clientKey:  clientKey,
		sessionKey: sessionKey,
		conn:       newWSConn(conn),
		conns:      map[int64]*connection{},
	}
}

func (s *session) startPings() {
	ctx, cancel := context.WithCancel(context.Background())
	s.pingCancel = cancel
	s.pingWait.Add(1)

	go func() {
		defer s.pingWait.Done()

		t := time.NewTicker(PingWriteInterval)
		defer t.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				s.conn.Lock()
				if err := s.conn.conn.WriteControl(websocket.PingMessage, []byte(""), time.Now().Add(time.Second)); err != nil {
					logrus.WithError(err).Error("Error writing ping")
				}
				logrus.Debug("Wrote ping")
				s.conn.Unlock()
			}
		}
	}()
}

func (s *session) stopPings() {
	if s.pingCancel == nil {
		return
	}

	s.pingCancel()
	s.pingWait.Wait()
}

func (s *session) serve() (int, error) {
	if s.client {
		s.startPings()
	}

	for {
		msType, reader, err := s.conn.NextReader()
		if err != nil {
			return 400, err
		}

		if msType != websocket.BinaryMessage {
			return 400, errWrongMessageType
		}

		if err := s.serveMessage(reader); err != nil {
			return 500, err
		}
	}
}

func (s *session) serveMessage(reader io.Reader) error {
	message, err := newServerMessage(reader)
	if err != nil {
		return err
	}

	logrus.Debug("REQUEST ", message)

	if message.messageType == Connect {
		if s.auth == nil || !s.auth(message.proto, message.address) {
			return errors.New("connect not allowed")
		}
		s.clientConnect(message)
		return nil
	}

	s.Lock()
	conn := s.conns[message.connID]
	s.Unlock()

	if conn == nil {
		if message.messageType == Data {
			err := fmt.Errorf("connection not found %s/%d/%d", s.clientKey, s.sessionKey, message.connID)
			newErrorMessage(message.connID, err).WriteTo(s.conn)
		}
		return nil
	}

	switch message.messageType {
	case Data:
		if _, err := io.Copy(conn.tunnelWriter(), message); err != nil {
			s.closeConnection(message.connID, err)
		}
	case Error:
		s.closeConnection(message.connID, message.Err())
	}

	return nil
}

func (s *session) closeConnection(connID int64, err error) {
	s.Lock()
	conn := s.conns[connID]
	delete(s.conns, connID)
	logrus.Debugf("CONNECTIONS %d %d", s.sessionKey, len(s.conns))
	s.Unlock()

	if conn != nil {
		conn.tunnelClose(err)
	}
}

func (s *session) clientConnect(message *message) {
	conn := newConnection(message.connID, s, message.proto, message.address)

	s.Lock()
	s.conns[message.connID] = conn
	logrus.Debugf("CONNECTIONS %d %d", s.sessionKey, len(s.conns))
	s.Unlock()

	go clientDial(conn, message)
}

func (s *session) serverConnect(deadline time.Duration, proto, address string) (net.Conn, error) {
	connID := atomic.AddInt64(&s.nextConnID, 1)
	conn := newConnection(connID, s, proto, address)

	s.Lock()
	s.conns[connID] = conn
	logrus.Debugf("CONNECTIONS %d %d", s.sessionKey, len(s.conns))
	s.Unlock()

	_, err := s.writeMessage(newConnect(connID, deadline, proto, address))
	if err != nil {
		s.closeConnection(connID, err)
		return nil, err
	}

	return conn, err
}

func (s *session) writeMessage(message *message) (int, error) {
	logrus.Debug("RESPONSE ", message)
	return message.WriteTo(s.conn)
}

func (s *session) Close() {
	s.Lock()
	defer s.Unlock()

	s.stopPings()

	for _, connection := range s.conns {
		connection.tunnelClose(errors.New("tunnel disconnect"))
	}

	s.conns = map[int64]*connection{}
}
