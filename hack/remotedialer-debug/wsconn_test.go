package remotedialer

import (
	"errors"
	"io"
	"time"
)

type fakeWSConn struct {
	writeMessageCallback func(int, time.Time, []byte) error
}

func (f fakeWSConn) Close() error {
	return nil
}

func (f fakeWSConn) NextReader() (int, io.Reader, error) {
	return 0, nil, errors.New("not implemented")
}

func (f fakeWSConn) WriteMessage(messageType int, deadline time.Time, data []byte) error {
	if cb := f.writeMessageCallback; cb != nil {
		return cb(messageType, deadline, data)
	}
	return errors.New("callback not provided")
}

func (f fakeWSConn) WriteControl(int, time.Time, []byte) error {
	return errors.New("not implemented")
}
