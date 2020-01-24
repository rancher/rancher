package logstream

import (
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/sirupsen/logrus"
)

var (
	logs          = map[string]LoggerStream{}
	lock          = sync.Mutex{}
	counter int64 = 1
)

type LogEvent struct {
	Error   bool
	Message string
}

type Logger interface {
	Infof(msg string, args ...interface{})
	Warnf(msg string, args ...interface{})
	Debugf(msg string, args ...interface{})
}

type LoggerStream interface {
	Logger
	ID() string
	Stream() <-chan LogEvent
	Close()
}

func GetLogStream(id string) LoggerStream {
	lock.Lock()
	defer lock.Unlock()
	return logs[id]
}

func NewLogStream() LoggerStream {
	id := atomic.AddInt64(&counter, 1)
	ls := newLoggerStream(strconv.FormatInt(id, 10))

	lock.Lock()
	logs[ls.ID()] = ls
	lock.Unlock()

	return ls
}

type loggerStream struct {
	sync.Mutex
	closed bool
	id     string
	c      chan LogEvent
}

func newLoggerStream(id string) LoggerStream {
	return &loggerStream{
		id: id,
		c:  make(chan LogEvent, 100),
	}
}

func (l *loggerStream) Infof(msg string, args ...interface{}) {
	l.write(false, msg, args...)
}

func (l *loggerStream) Warnf(msg string, args ...interface{}) {
	l.write(true, msg, args...)
}

func (l *loggerStream) Debugf(msg string, args ...interface{}) {
	logrus.Debugf(msg, args...)
}

func (l *loggerStream) write(error bool, msg string, args ...interface{}) {
	msg = fmt.Sprintf(msg, args...)
	l.Lock()
	if !l.closed {
		l.c <- LogEvent{
			Error:   error,
			Message: msg,
		}
	}
	l.Unlock()
}

func (l *loggerStream) ID() string {
	return l.id
}

func (l *loggerStream) Stream() <-chan LogEvent {
	return l.c
}

func (l *loggerStream) Close() {
	l.Lock()
	if !l.closed {
		close(l.c)
		l.closed = true
	}
	l.Unlock()
	lock.Lock()
	delete(logs, l.id)
	lock.Unlock()
}
