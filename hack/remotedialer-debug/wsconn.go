package remotedialer

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type wsWrapper struct {
	// Mutex is used to protect from concurrent usage of the websocket connection
	sync.Mutex
	// conn is the underlying websocket connection
	conn *websocket.Conn
}

func newWSConn(conn *websocket.Conn) *wsWrapper {
	w := &wsWrapper{
		conn: conn,
	}
	w.setupDeadline()
	return w
}

type wsConn interface {
	// Close will indicate the underlying websocket connection
	Close() error
	// NextReader gets a new reader from the underlying websocket connection
	NextReader() (int, io.Reader, error)
	// WriteControl writes a new websocket control frame, see https://datatracker.ietf.org/doc/html/rfc6455#section-5.5
	WriteControl(messageType int, deadline time.Time, data []byte) error
	// WriteMessage writes a new websocket data frame, see https://datatracker.ietf.org/doc/html/rfc6455#section-6
	WriteMessage(messageType int, deadline time.Time, data []byte) error
}

func (w *wsWrapper) WriteControl(messageType int, deadline time.Time, data []byte) error {
	w.Lock()
	defer w.Unlock()

	return w.conn.WriteControl(messageType, data, deadline)
}

func (w *wsWrapper) WriteMessage(messageType int, deadline time.Time, data []byte) error {
	if deadline.IsZero() {
		w.Lock()
		defer w.Unlock()
		return w.conn.WriteMessage(messageType, data)
	}

	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		w.Lock()
		defer w.Unlock()
		done <- w.conn.WriteMessage(messageType, data)
	}()

	select {
	case <-ctx.Done():
		return fmt.Errorf("i/o timeout")
	case err := <-done:
		return err
	}
}

func (w *wsWrapper) NextReader() (int, io.Reader, error) {
	return w.conn.NextReader()
}

func (w *wsWrapper) Close() error {
	return w.conn.Close()
}

func (w *wsWrapper) setupDeadline() {
	w.conn.SetReadDeadline(time.Now().Add(PingWaitDuration))
	w.conn.SetPingHandler(func(string) error {
		w.Lock()
		err := w.conn.WriteControl(websocket.PongMessage, []byte(""), time.Now().Add(PingWaitDuration))
		w.Unlock()
		if err != nil {
			return err
		}
		if err := w.conn.SetReadDeadline(time.Now().Add(PingWaitDuration)); err != nil {
			return err
		}
		return w.conn.SetWriteDeadline(time.Now().Add(PingWaitDuration))
	})
	w.conn.SetPongHandler(func(string) error {
		if err := w.conn.SetReadDeadline(time.Now().Add(PingWaitDuration)); err != nil {
			return err
		}
		return w.conn.SetWriteDeadline(time.Now().Add(PingWaitDuration))
	})

}
