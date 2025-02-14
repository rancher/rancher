package listener

import (
	"fmt"
	"net"
	"sync"

	"github.com/sirupsen/logrus"
)

type listenerState int

const (
	listenerStateUnknown listenerState = iota
	listenerStateStarted
	listenerStateStopped
	listenerStateClosed
)

var _ net.Listener = &Listener{}

type Listener struct {
	addr net.Addr
	ln   *WaitLoader[net.Listener]

	stateMu sync.RWMutex
	state   listenerState
}

// NewListener creates a new listener with the provided factory, initially in the stopped state. Currently only supports TCP listeners.
func NewListener(addr string) (*Listener, error) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve listener address:c %w", err)
	}

	return &Listener{
		addr: tcpAddr,
		ln:   NewWaitLoader[net.Listener](),

		state: listenerStateStopped,
	}, nil
}

// Start signals to start the underlying listener and unblocks any calls to Accept. Calls will fail if the listener is already started or closed.
func (l *Listener) Start() error {
	l.stateMu.Lock()
	defer l.stateMu.Unlock()

	switch l.state {
	case listenerStateStarted:
		return fmt.Errorf("cannot start an already started listener")
	case listenerStateClosed:
		return fmt.Errorf("cannot start a closed listener")
	case listenerStateUnknown:
		return fmt.Errorf("listener is in an unknown state")
	}

	ln, err := net.Listen("tcp", l.addr.String())
	if err != nil {
		return fmt.Errorf("failed to create listener: %w", err)
	}

	l.ln.Set(ln)

	l.state = listenerStateStarted

	return nil
}

// Stop signals to close the underlying listener. Any calls to Accept will block until Start is called. Calls will fail if the listener is already stopped or closed.
func (l *Listener) Stop() error {
	l.stateMu.Lock()
	defer l.stateMu.Unlock()

	switch l.state {
	case listenerStateStopped:
		return fmt.Errorf("listener is already stopped")
	case listenerStateClosed:
		return fmt.Errorf("listener is closed")
	case listenerStateUnknown:
		return fmt.Errorf("listener is in an unknown state")
	}

	l.state = listenerStateStopped

	ln := l.ln.Load()
	l.ln.Unset()

	if err := ln.Close(); err != nil {
		return fmt.Errorf("failed to close listener: %w", err)
	}

	return nil
}

func (l *Listener) ignoreError(err error) bool {
	l.stateMu.RLock()
	defer l.stateMu.RUnlock()

	if l.state != listenerStateStopped {
		return false
	}

	opErr, ok := err.(*net.OpError)
	if !ok {
		return false
	}

	if opErr.Source == nil {
		return true
	}

	return false

}

func (l *Listener) Accept() (net.Conn, error) {
	for {
		conn, err := l.ln.Load().Accept()
		if err == nil {
			return conn, err
		}

		if l.ignoreError(err) {
			logrus.Infof("listener was closed, waiting for another connection")
		} else {
			return nil, err
		}
	}
}

func (l *Listener) Close() error {
	l.stateMu.Lock()
	defer l.stateMu.Unlock()

	if l.state != listenerStateStarted {
		return nil
	}

	ln := l.ln.Load()
	l.ln.Unset()

	l.state = listenerStateClosed

	if err := ln.Close(); err != nil {
		return fmt.Errorf("failed to close listener: %w", err)
	}

	return nil
}

func (l *Listener) Addr() net.Addr {
	return l.addr
}
