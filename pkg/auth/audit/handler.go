package audit

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"net/http"
	"reflect"
	"sync"
	"time"

	"github.com/rancher/steve/pkg/auth"
	"github.com/sirupsen/logrus"
)

const (
	errorDebounceTime = time.Second * 30
)

type userKey string

var userKeyValue userKey = "audit_user"

func NewAuditLogMiddleware(writer *Writer) auth.Middleware {
	return GetAuditLoggerMiddleware(&LoggingHandler{
		writer:  writer,
		errMap:  make(map[string]time.Time),
		errLock: &sync.Mutex{},
	})
}

type LoggingHandler struct {
	writer *Writer

	errMap  map[string]time.Time
	errLock *sync.Mutex
}

type wrapWriter struct {
	http.ResponseWriter

	hijacked    bool
	headerWrote bool

	statusCode   int
	bytesWritten int
	buf          bytes.Buffer
}

func (w *wrapWriter) WriteHeader(statusCode int) {
	w.ResponseWriter.WriteHeader(statusCode)
	if !w.headerWrote {
		w.statusCode = statusCode
		w.headerWrote = true
	}
}

func (w *wrapWriter) Write(body []byte) (int, error) {
	if !w.headerWrote {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(body)
	w.bytesWritten += n
	w.buf.Write(body)
	return n, err
}

func (w *wrapWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := w.ResponseWriter.(http.Hijacker); ok {
		w.hijacked = true
		return hijacker.Hijack()
	}
	return nil, nil, fmt.Errorf("upstream ResponseWriter of type %v does not implement http.Hijacker", reflect.TypeOf(w.ResponseWriter))
}

func (w *wrapWriter) CloseNotify() <-chan bool {
	if cn, ok := w.ResponseWriter.(http.CloseNotifier); ok {
		return cn.CloseNotify()
	}
	logrus.Errorf("Upstream ResponseWriter of type %v does not implement http.CloseNotifier", reflect.TypeOf(w.ResponseWriter))
	return make(<-chan bool)
}

func (w *wrapWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		if !w.headerWrote {
			w.WriteHeader(http.StatusOK)
		}
		flusher.Flush()
	}
	logrus.Errorf("Upstream ResponseWriter of type %v does not implement http.Flusher", reflect.TypeOf(w.ResponseWriter))
}

var _ http.ResponseWriter = (*wrapWriter)(nil)
var _ http.Hijacker = (*wrapWriter)(nil)
var _ http.Flusher = (*wrapWriter)(nil)
