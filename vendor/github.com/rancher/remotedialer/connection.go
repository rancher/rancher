package remotedialer

import (
	"io"
	"net"
	"time"

	"github.com/rancher/remotedialer/metrics"
)

type connection struct {
	err           error
	writeDeadline time.Time
	buffer        *readBuffer
	addr          addr
	session       *Session
	connID        int64
}

func newConnection(connID int64, session *Session, proto, address string) *connection {
	c := &connection{
		buffer: newReadBuffer(),
		addr: addr{
			proto:   proto,
			address: address,
		},
		connID:  connID,
		session: session,
	}
	metrics.IncSMTotalAddConnectionsForWS(session.clientKey, proto, address)
	return c
}

func (c *connection) tunnelClose(err error) {
	metrics.IncSMTotalRemoveConnectionsForWS(c.session.clientKey, c.addr.Network(), c.addr.String())
	c.writeErr(err)
	c.doTunnelClose(err)
}

func (c *connection) doTunnelClose(err error) {
	if c.err != nil {
		return
	}

	c.err = err
	if c.err == nil {
		c.err = io.ErrClosedPipe
	}

	c.buffer.Close(c.err)
}

func (c *connection) OnData(m *message) error {
	return c.buffer.Offer(m.body)
}

func (c *connection) Close() error {
	c.session.closeConnection(c.connID, io.EOF)
	return nil
}

func (c *connection) Read(b []byte) (int, error) {
	n, err := c.buffer.Read(b)
	metrics.AddSMTotalReceiveBytesOnWS(c.session.clientKey, float64(n))
	return n, err
}

func (c *connection) Write(b []byte) (int, error) {
	if c.err != nil {
		return 0, c.err
	}

	msg := newMessage(c.connID, b)
	metrics.AddSMTotalTransmitBytesOnWS(c.session.clientKey, float64(len(msg.Bytes())))
	n, err := c.session.writeMessage(c.writeDeadline, msg)
	if err != nil {
		return 0, err
	}
	return n, c.err
}

func (c *connection) writeErr(err error) {
	if err != nil {
		msg := newErrorMessage(c.connID, err)
		metrics.AddSMTotalTransmitErrorBytesOnWS(c.session.clientKey, float64(len(msg.Bytes())))
		c.session.writeMessage(c.writeDeadline, msg)
	}
}

func (c *connection) LocalAddr() net.Addr {
	return c.addr
}

func (c *connection) RemoteAddr() net.Addr {
	return c.addr
}

func (c *connection) SetDeadline(t time.Time) error {
	if err := c.SetReadDeadline(t); err != nil {
		return err
	}
	return c.SetWriteDeadline(t)
}

func (c *connection) SetReadDeadline(t time.Time) error {
	c.buffer.deadline = t
	return nil
}

func (c *connection) SetWriteDeadline(t time.Time) error {
	c.writeDeadline = t
	return nil
}

type addr struct {
	proto   string
	address string
}

func (a addr) Network() string {
	return a.proto
}

func (a addr) String() string {
	return a.address
}
