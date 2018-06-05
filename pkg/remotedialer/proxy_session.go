package remotedialer

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type proxySession struct {
	session
	target  *session
	connMap map[int64]int64
}

func newProxyClientSession(identity string, conn *websocket.Conn) *proxySession {
	return &proxySession{
		session: session{
			clientKey:  identity,
			conn:       newWSConn(conn),
			conns:      map[int64]*connection{},
			auth:       nil,
			client:     true,
			nextConnID: 1,
		},
	}
}

func newProxySession(sessionKey int64, clientKey string, conn *websocket.Conn, target *session) *proxySession {
	return &proxySession{
		session: session{
			nextConnID: 1,
			clientKey:  clientKey,
			sessionKey: sessionKey,
			conn:       newWSConn(conn),
			conns:      map[int64]*connection{},
		},
		target:  target,
		connMap: map[int64]int64{},
	}
}

func (s *proxySession) serve() (int, error) {
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

func (s *proxySession) serveMessage(reader io.Reader) error {
	message, err := newServerMessage(reader)
	if err != nil {
		return err
	}

	logrus.Debug("Proxy ", message)

	if message.messageType == Connect {
		id, _, err := s.target.connect(time.Duration(message.deadline), message.proto, message.address)
		if err != nil {
			return errors.Wrap(err, "proxy connect error")
		}
		s.proxyClientConnect(message, id)
		return nil
	}

	if s.client {
		s.Lock()
		conn := s.conns[message.connID]
		s.Unlock()
		if conn == nil {
			if message.messageType == Data {
				err := fmt.Errorf("proxy connection not found %s/%d/%d", s.clientKey, s.sessionKey, message.connID)
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
	} else {
		s.Lock()
		targetConnID, ok := s.connMap[message.connID]
		if !ok {
			err := fmt.Errorf("connect lost for %s", s.clientKey)
			newErrorMessage(message.connID, err).WriteTo(s.conn)
			s.Unlock()
			return err
		}
		conn := s.target.conns[targetConnID]
		s.Unlock()
		if conn == nil {
			if message.messageType == Data {
				err := fmt.Errorf("proxy connection not found %s/%d/%d", s.clientKey, s.sessionKey, message.connID)
				newErrorMessage(message.connID, err).WriteTo(s.conn)
			}
			return nil
		}
		switch message.messageType {
		case Data:
			if _, err := io.Copy(conn, message); err != nil {
				s.closeConnection(message.connID, err)
			}
		case Error:
			s.closeConnection(message.connID, message.Err())
		}
	}

	return nil
}

func (s *proxySession) proxyClientConnect(message *message, targetID int64) {
	conn := newConnection(message.connID, &s.session, message.proto, message.address)

	s.Lock()
	s.conns[message.connID] = conn
	logrus.Debugf("CONNECTIONS %d %d", s.sessionKey, len(s.conns))
	s.target.Lock()
	tarconn := s.target.conns[targetID]
	s.target.Unlock()
	s.connMap[message.connID] = targetID
	s.Unlock()

	go proxyPipe(conn, tarconn)
}

func (s *proxySession) closeConnection(connID int64, err error) {
	s.Lock()
	conn := s.conns[connID]
	tarid := s.connMap[connID]
	delete(s.conns, connID)
	delete(s.connMap, connID)
	if s.target != nil {
		s.target.closeConnection(tarid, err)
	}
	logrus.Debugf("CONNECTIONS %d %d", s.sessionKey, len(s.conns))
	s.Unlock()

	if conn != nil {
		conn.tunnelClose(err)
	}
}

func (s *proxySession) proxyServerConnect(deadline time.Duration, proto, address string) (net.Conn, error) {
	connID := atomic.AddInt64(&s.nextConnID, 1)
	conn := newConnection(connID, &s.session, proto, address)

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

func (s *proxySession) Close() {
	s.Lock()
	defer s.Unlock()

	s.stopPings()

	for _, connection := range s.conns {
		connection.tunnelClose(errors.New("tunnel disconnect"))
	}

	s.conns = map[int64]*connection{}

	if s.target != nil {
		s.target.Close()
	}
	s.conn.conn.WriteControl(websocket.CloseMessage, nil, time.Now().Add(5*time.Second))
}

func (s *proxySession) startPings() {
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

func (s *proxySession) stopPings() {
	if s.pingCancel == nil {
		return
	}

	s.pingCancel()
	s.pingWait.Wait()
}
