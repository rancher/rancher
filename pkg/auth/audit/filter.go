package audit

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/rancher/rancher/pkg/auth/util"
	"github.com/rancher/rancher/pkg/data/management"
	"github.com/sirupsen/logrus"
)

var (
	errorDebounceTime = time.Second * 30
)

func NewAuditLogMiddleware(auditWriter *LogWriter) (func(http.Handler) http.Handler, error) {
	sensitiveRegex, err := constructKeyRedactRegex()
	return func(next http.Handler) http.Handler {
		return &auditHandler{
			next:            next,
			auditWriter:     auditWriter,
			sanitizingRegex: sensitiveRegex,
			errMap:          make(map[string]time.Time),
			errLock:         &sync.Mutex{},
		}
	}, err
}

// constructKeyRedactRegex builds a regex for matching non-public fields from management.DriverData as well as fields that end with [pP]assword or [tT]oken.
func constructKeyRedactRegex() (*regexp.Regexp, error) {
	s := strings.Builder{}
	s.WriteRune('(')
	for _, v := range management.DriverData {
		for key, value := range v {
			if strings.HasPrefix(key, "public") || strings.HasPrefix(key, "optional") {
				continue
			}
			for _, item := range value {
				s.WriteString(item + "|")
			}
		}
	}
	s.WriteString(`[pP]assword|[tT]oken|[kK]ube[cC]onfig)`)

	return regexp.Compile(s.String())
}

type auditHandler struct {
	next            http.Handler
	auditWriter     *LogWriter
	sanitizingRegex *regexp.Regexp
	errMap          map[string]time.Time
	errLock         *sync.Mutex
}

func (h auditHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if h.auditWriter == nil {
		h.next.ServeHTTP(rw, req)
		return
	}

	user := getUserInfo(req)

	context := context.WithValue(req.Context(), userKey, user)
	req = req.WithContext(context)

	auditLog, err := newAuditLog(h.auditWriter, req, h.sanitizingRegex)
	if err != nil {
		util.ReturnHTTPError(rw, req, http.StatusInternalServerError, err.Error())
		return
	}

	wr := &wrapWriter{ResponseWriter: rw, auditWriter: h.auditWriter, statusCode: http.StatusOK}
	h.next.ServeHTTP(wr, req)

	err = auditLog.write(user, req.Header, wr.Header(), wr.statusCode, wr.buf.Bytes())
	if err == nil {
		return
	}

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

type wrapWriter struct {
	http.ResponseWriter
	auditWriter *LogWriter
	statusCode  int
	buf         bytes.Buffer
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
	return nil, nil, fmt.Errorf("Upstream ResponseWriter of type %v does not implement http.Hijacker", reflect.TypeOf(aw.ResponseWriter))
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
