package ext

import (
	"fmt"
	"net"
	"sync"
)

var _ net.Listener = &blockingListener{}

type blockingListener struct {
	stopMu   sync.RWMutex
	stopChan chan struct{}
}

func NewBlockingListener() net.Listener {
	return &blockingListener{
		stopChan: make(chan struct{}),
	}
}

func (l *blockingListener) Accept() (net.Conn, error) {
	l.stopMu.RLock()
	if l.stopChan == nil {
		l.stopMu.RUnlock()
		return nil, fmt.Errorf("listener is closed")
	}
	l.stopMu.RUnlock()

	<-l.stopChan

	return nil, fmt.Errorf("listener is closed")
}

func (d *blockingListener) Addr() net.Addr {
	return &net.TCPAddr{
		Port: Port,
	}
}

func (d *blockingListener) Close() error {
	d.stopMu.Lock()
	defer d.stopMu.Unlock()

	if d.stopChan == nil {
		return fmt.Errorf("listener is already closed")
	}

	close(d.stopChan)
	d.stopChan = nil

	return nil
}
