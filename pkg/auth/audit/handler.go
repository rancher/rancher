package audit

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"reflect"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	errorDebounceTime = time.Second * 30
)

type userKey string

var userKeyValue userKey = "audit_user"

func NewAuditLogMiddleware(writer *Writer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return handler{
			next:    next,
			writer:  writer,
			errMap:  make(map[string]time.Time),
			errLock: &sync.Mutex{},
		}
	}
}

type handler struct {
	next   http.Handler
	writer *Writer

	errMap  map[string]time.Time
	errLock *sync.Mutex
}

func (h handler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if h.writer == nil {
		h.next.ServeHTTP(rw, req)
		return
	}

	reqTimestamp := time.Now().Format(time.RFC3339)

	user := getUserInfo(req)

	context := context.WithValue(req.Context(), userKeyValue, user)
	req = req.WithContext(context)

	wr := &wrapWriter{
		next: rw,

		statusCode: http.StatusOK,
	}
	h.next.ServeHTTP(wr, req)
	wr.Apply()

	respTimestamp := time.Now().Format(time.RFC3339)

	log := newLog(user, req, wr, reqTimestamp, respTimestamp)

	if err := h.writer.Write(log); err != nil {
		// Locking after next is called to avoid performance hits on the request.
		h.errLock.Lock()
		defer h.errLock.Unlock()

		// Only log duplicate error messages at most every errorDebounceTime.
		// This is to prevent the rancher logs from being flooded with error messages
		// when the log path is invalid or any other error that will always cause a write to fail.
		if lastSeen, ok := h.errMap[err.Error()]; !ok || time.Since(lastSeen) > errorDebounceTime {
			logrus.Warnf("Failed to write audit log: %s", err)
			h.errMap[err.Error()] = time.Now()
		}
	}
}

type wrapWriter struct {
	next http.ResponseWriter

	wroteHeader bool

	wroteBody bool
	buf       bytes.Buffer

	statusCode int
}

func (w *wrapWriter) Header() http.Header {
	return w.next.Header()
}

func (w *wrapWriter) WriteHeader(statusCode int) {
	w.wroteHeader = true
	w.statusCode = statusCode
}

func (w *wrapWriter) Write(body []byte) (int, error) {
	w.wroteBody = true
	return w.buf.Write(body)
}

func (w *wrapWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := w.next.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, fmt.Errorf("upstream ResponseWriter of type %v does not implement http.Hijacker", reflect.TypeOf(w.next))
}

func (w *wrapWriter) CloseNotify() <-chan bool {
	if cn, ok := w.next.(http.CloseNotifier); ok {
		return cn.CloseNotify()
	}
	logrus.Errorf("Upstream ResponseWriter of type %v does not implement http.CloseNotifier", reflect.TypeOf(w.next))
	return make(<-chan bool)
}

func (w *wrapWriter) Flush() {
	if f, ok := w.next.(http.Flusher); ok {
		f.Flush()
		return
	}
	logrus.Errorf("Upstream ResponseWriter of type %v does not implement http.Flusher", reflect.TypeOf(w.next))
}

func (w *wrapWriter) Apply() {
	w.next.WriteHeader(w.statusCode)
	w.next.Write(w.buf.Bytes())
}
