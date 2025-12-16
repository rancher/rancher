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

	auditlogv1 "github.com/rancher/rancher/pkg/apis/auditlog.cattle.io/v1"
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
		level:   writer.DefaultPolicyLevel,
		writer:  writer,
		errMap:  make(map[string]time.Time),
		errLock: &sync.Mutex{},
	})
}

type LoggingHandler struct {
	level  auditlogv1.Level
	writer *Writer

	errMap  map[string]time.Time
	errLock *sync.Mutex
}

func (lh *LoggingHandler) ResolveVerbosity(requestURI string) auditlogv1.LogVerbosity {
	verbosity := verbosityForLevel(lh.writer.DefaultPolicyLevel)

	lh.writer.policiesMutex.RLock()
	defer lh.writer.policiesMutex.RUnlock()

	for _, policy := range lh.writer.policies {
		if policy.actionForUri(requestURI) == auditlogv1.FilterActionAllow {
			verbosity = mergeLogVerbosities(verbosity, policy.Verbosity)
		}
	}

	return verbosity
}

func (lh *LoggingHandler) Write(entry *logEntry) {
	if err := lh.writer.Write(entry); err != nil {
		// Locking after next is called to avoid performance hits on the request.
		lh.errLock.Lock()
		defer lh.errLock.Unlock()

		// Only log duplicate error messages at most every errorDebounceTime.
		// This is to prevent the rancher logs from being flooded with error messages
		// when the log path is invalid or any other error that will always cause a write to fail.
		if lastSeen, ok := lh.errMap[err.Error()]; !ok || time.Since(lastSeen) > errorDebounceTime {
			logrus.Warnf("Failed to write audit logEntry: %s", err)
			lh.errMap[err.Error()] = time.Now()
		}
	}
}

type wrapWriter struct {
	http.ResponseWriter

	hijacked    bool
	headerWrote bool

	statusCode   int
	bytesWritten int
	keepBody     bool
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
	if w.keepBody {
		w.buf.Write(body)
	}
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
		return
	}
	logrus.Errorf("Upstream ResponseWriter of type %v does not implement http.Flusher", reflect.TypeOf(w.ResponseWriter))
}

var _ http.ResponseWriter = (*wrapWriter)(nil)
var _ http.Hijacker = (*wrapWriter)(nil)
var _ http.Flusher = (*wrapWriter)(nil)
