package remotedialer

import (
	"context"
	"fmt"
	"io"

	"github.com/sirupsen/logrus"
)

// serveMessage accepts an incoming message from the underlying websocket connection and processes the request based on its messageType
func (s *Session) serveMessage(ctx context.Context, reader io.Reader) error {
	message, err := newServerMessage(reader)
	if err != nil {
		return fmt.Errorf("failed to create server message: %w", err)
	}

	if PrintTunnelData {
		logrus.Debug("REQUEST ", message)
	}

	switch message.messageType {
	case Connect:
		return s.clientConnect(ctx, message)
	case AddClient:
		return s.addRemoteClient(message.address)
	case RemoveClient:
		return s.removeRemoteClient(message.address)
	case SyncConnections:
		return s.syncConnections(message.body)
	case Data:
		s.connectionData(message.connID, message.body)
	case Pause:
		s.pauseConnection(message.connID)
	case Resume:
		s.resumeConnection(message.connID)
	case Error:
		s.closeConnection(message.connID, message.Err())
	default:
		logrus.Warnf("unknown message type: %v", message.messageType)
	}
	return nil
}

// clientConnect accepts a new connection request, dialing back to establish the connection
func (s *Session) clientConnect(ctx context.Context, message *message) error {
	if s.auth == nil || !s.auth(message.proto, message.address) {
		return fmt.Errorf("connect not allowed for %s://%s", message.proto, message.address)
	}

	conn := newConnection(message.connID, s, message.proto, message.address)
	s.addConnection(message.connID, conn)

	go clientDial(ctx, s.dialer, conn, message)

	return nil
}

// addRemoteClient registers a new remote client, making it accessible for requests
func (s *Session) addRemoteClient(address string) error {
	if address == "" {
		return fmt.Errorf("address cannot be empty")
	}

	if s.remoteClientKeys == nil {
		return nil
	}

	clientKey, sessionKey, err := parseAddress(address)
	if err != nil {
		return fmt.Errorf("invalid remote Session %s: %v", address, err)
	}
	s.addSessionKey(clientKey, sessionKey)

	if PrintTunnelData {
		logrus.Debugf("ADD REMOTE CLIENT %s, SESSION %d", address, s.sessionKey)
	}

	return nil
}

// removeRemoteClient removes a given client from a session
func (s *Session) removeRemoteClient(address string) error {
	if address == "" {
		return fmt.Errorf("address cannot be empty")
	}

	clientKey, sessionKey, err := parseAddress(address)
	if err != nil {
		return fmt.Errorf("invalid remote Session %s: %v", address, err)
	}
	s.removeSessionKey(clientKey, sessionKey)

	if PrintTunnelData {
		logrus.Debugf("REMOVE REMOTE CLIENT %s, SESSION %d", address, s.sessionKey)
	}

	return nil
}

// syncConnections closes any session connection that is not present in the IDs received from the client
func (s *Session) syncConnections(r io.Reader) error {
	payload, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("reading message body: %w", err)
	}
	clientActiveConnections, err := decodeConnectionIDs(payload)
	if err != nil {
		return fmt.Errorf("decoding sync connections payload: %w", err)
	}

	s.compareAndCloseStaleConnections(clientActiveConnections)
	return nil
}

// closeConnection removes a connection for a given ID from the session, sending an error message to communicate the closing to the other end.
// If an error is not provided, io.EOF will be used instead.
func (s *Session) closeConnection(connID int64, err error) {
	if conn := s.removeConnection(connID); conn != nil {
		conn.tunnelClose(err)
	}
}

// connectionData process incoming data from connection by reading the body into an internal readBuffer
func (s *Session) connectionData(connID int64, body io.Reader) {
	conn := s.getConnection(connID)
	if conn == nil {
		errMsg := newErrorMessage(connID, fmt.Errorf("connection not found %s/%d/%d", s.clientKey, s.sessionKey, connID))
		if _, err := errMsg.WriteTo(defaultDeadline(), s.conn); err != nil {
			logrus.Errorf("failed to write error message for connection %d: %v", connID, err)
		}
		return
	}

	if err := conn.OnData(body); err != nil {
		logrus.Debugf("connection %d data processing error: %v", connID, err)
		s.closeConnection(connID, err)
	}
}

// pauseConnection activates backPressure for a given connection ID
func (s *Session) pauseConnection(connID int64) {
	if conn := s.getConnection(connID); conn != nil {
		conn.OnPause()
	}
}

// resumeConnection deactivates backPressure for a given connection ID
func (s *Session) resumeConnection(connID int64) {
	if conn := s.getConnection(connID); conn != nil {
		conn.OnResume()
	}
}
