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

	"github.com/rancher/rancher/pkg/auth/util"
	"github.com/sirupsen/logrus"
)

const (
	errorDebounceTime = time.Second * 30
)

var userKey struct{}

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

	context := context.WithValue(req.Context(), userKey, user)
	req = req.WithContext(context)

	wr := &wrapWriter{ResponseWriter: rw, statusCode: http.StatusOK}
	h.next.ServeHTTP(wr, req)

	respTimestamp := time.Now().Format(time.RFC3339)

	log, err := newLog(user, req, wr, reqTimestamp, respTimestamp)
	if err != nil {
		util.ReturnHTTPError(rw, req, http.StatusInternalServerError, err.Error())
		return
	}

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
	http.ResponseWriter
	statusCode int
	buf        bytes.Buffer
}

func (aw *wrapWriter) WriteHeader(statusCode int) {
	aw.ResponseWriter.WriteHeader(statusCode)
	aw.statusCode = statusCode
}

func (aw *wrapWriter) Write(body []byte) (int, error) {
	aw.buf.Write(body)
	return aw.ResponseWriter.Write(body)
}

func (aw *wrapWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := aw.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, fmt.Errorf("upstream ResponseWriter of type %v does not implement http.Hijacker", reflect.TypeOf(aw.ResponseWriter))
}

func (aw *wrapWriter) CloseNotify() <-chan bool {
	if cn, ok := aw.ResponseWriter.(http.CloseNotifier); ok {
		return cn.CloseNotify()
	}
	logrus.Errorf("Upstream ResponseWriter of type %v does not implement http.CloseNotifier", reflect.TypeOf(aw.ResponseWriter))
	return make(<-chan bool)
}

func (aw *wrapWriter) Flush() {
	if f, ok := aw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
		return
	}
	logrus.Errorf("Upstream ResponseWriter of type %v does not implement http.Flusher", reflect.TypeOf(aw.ResponseWriter))
}
