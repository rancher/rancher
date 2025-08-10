package remotedialer

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// Data is the main message type, used to transport application data
	Data messageType = iota + 1
	// Connect is a control message type, used to request opening a new connection
	Connect
	// Error is a message type used to send an error during the communication.
	// Any receiver of an Error message can assume the connection can be closed.
	// io.EOF is used for graceful termination of connections.
	Error
	// AddClient is a message type used to open a new client to the peering session
	AddClient
	// RemoveClient is a message type used to remove an existing client from a peering session
	RemoveClient
	// Pause is a message type used to temporarily stop a given connection
	Pause
	// Resume is a message type used to resume a paused connection
	Resume
	// SyncConnections is a message type used to communicate active connection IDs.
	// The receiver can consider any ID not present in this message as stale and free any associated resource.
	SyncConnections
)

var (
	idCounter      int64
	legacyDeadline = (15 * time.Second).Milliseconds()
)

func init() {
	r := rand.New(rand.NewSource(int64(time.Now().Nanosecond())))
	idCounter = r.Int63()
}

type messageType int64

type message struct {
	id          int64
	err         error
	connID      int64
	messageType messageType
	bytes       []byte
	body        io.Reader
	proto       string
	address     string
}

func nextid() int64 {
	return atomic.AddInt64(&idCounter, 1)
}

func newMessage(connID int64, bytes []byte) *message {
	return &message{
		id:          nextid(),
		connID:      connID,
		messageType: Data,
		bytes:       bytes,
	}
}

func newPause(connID int64) *message {
	return &message{
		id:          nextid(),
		connID:      connID,
		messageType: Pause,
	}
}

func newResume(connID int64) *message {
	return &message{
		id:          nextid(),
		connID:      connID,
		messageType: Resume,
	}
}

func newConnect(connID int64, proto, address string) *message {
	return &message{
		id:          nextid(),
		connID:      connID,
		messageType: Connect,
		bytes:       []byte(fmt.Sprintf("%s/%s", proto, address)),
		proto:       proto,
		address:     address,
	}
}

func newErrorMessage(connID int64, err error) *message {
	return &message{
		id:          nextid(),
		err:         err,
		connID:      connID,
		messageType: Error,
		bytes:       []byte(err.Error()),
	}
}

func newAddClient(client string) *message {
	return &message{
		id:          nextid(),
		messageType: AddClient,
		address:     client,
		bytes:       []byte(client),
	}
}

func newRemoveClient(client string) *message {
	return &message{
		id:          nextid(),
		messageType: RemoveClient,
		address:     client,
		bytes:       []byte(client),
	}
}

func newServerMessage(reader io.Reader) (*message, error) {
	buf := bufio.NewReader(reader)

	id, err := binary.ReadVarint(buf)
	if err != nil {
		return nil, err
	}

	connID, err := binary.ReadVarint(buf)
	if err != nil {
		return nil, err
	}

	mType, err := binary.ReadVarint(buf)
	if err != nil {
		return nil, err
	}

	m := &message{
		id:          id,
		messageType: messageType(mType),
		connID:      connID,
		body:        buf,
	}

	if m.messageType == Data || m.messageType == Connect {
		// no longer used, this is the deadline field
		_, err := binary.ReadVarint(buf)
		if err != nil {
			return nil, err
		}
	}

	if m.messageType == Connect {
		bytes, err := ioutil.ReadAll(io.LimitReader(buf, 100))
		if err != nil {
			return nil, err
		}
		parts := strings.SplitN(string(bytes), "/", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("failed to parse connect address")
		}
		m.proto = parts[0]
		m.address = parts[1]
		m.bytes = bytes
	} else if m.messageType == AddClient || m.messageType == RemoveClient {
		bytes, err := ioutil.ReadAll(io.LimitReader(buf, 100))
		if err != nil {
			return nil, err
		}
		m.address = string(bytes)
		m.bytes = bytes
	}

	return m, nil
}

func (m *message) Err() error {
	if m.err != nil {
		return m.err
	}
	bytes, err := ioutil.ReadAll(io.LimitReader(m.body, 100))
	if err != nil {
		return err
	}

	str := string(bytes)
	if str == "EOF" {
		m.err = io.EOF
	} else {
		m.err = errors.New(str)
	}
	return m.err
}

func (m *message) Bytes() []byte {
	return append(m.header(len(m.bytes)), m.bytes...)
}

func (m *message) header(space int) []byte {
	buf := make([]byte, 24+space)
	offset := 0
	offset += binary.PutVarint(buf[offset:], m.id)
	offset += binary.PutVarint(buf[offset:], m.connID)
	offset += binary.PutVarint(buf[offset:], int64(m.messageType))
	if m.messageType == Data || m.messageType == Connect {
		offset += binary.PutVarint(buf[offset:], legacyDeadline)
	}
	return buf[:offset]
}

func (m *message) Read(p []byte) (int, error) {
	return m.body.Read(p)
}

func (m *message) WriteTo(deadline time.Time, wsConn wsConn) (int, error) {
	err := wsConn.WriteMessage(websocket.BinaryMessage, deadline, m.Bytes())
	return len(m.bytes), err
}

func (m *message) String() string {
	switch m.messageType {
	case Data:
		if m.body == nil {
			return fmt.Sprintf("%d DATA         [%d]: %d bytes: %s", m.id, m.connID, len(m.bytes), string(m.bytes))
		}
		return fmt.Sprintf("%d DATA         [%d]: buffered", m.id, m.connID)
	case Error:
		return fmt.Sprintf("%d ERROR        [%d]: %s", m.id, m.connID, m.Err())
	case Connect:
		return fmt.Sprintf("%d CONNECT      [%d]: %s/%s", m.id, m.connID, m.proto, m.address)
	case AddClient:
		return fmt.Sprintf("%d ADDCLIENT    [%s]", m.id, m.address)
	case RemoveClient:
		return fmt.Sprintf("%d REMOVECLIENT [%s]", m.id, m.address)
	case Pause:
		return fmt.Sprintf("%d PAUSE        [%d]", m.id, m.connID)
	case Resume:
		return fmt.Sprintf("%d RESUME       [%d]", m.id, m.connID)
	case SyncConnections:
		return fmt.Sprintf("%d SYNCCONNS    [%d]", m.id, m.connID)
	}
	return fmt.Sprintf("%d UNKNOWN[%d]: %d", m.id, m.connID, m.messageType)
}
