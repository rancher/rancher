package audit

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"net"
	"net/http"

	"github.com/rancher/rancher/pkg/auth/util"
)

func NewAuditLogFilter(ctx context.Context, auditWriter *LogWriter, next http.Handler) http.Handler {
	return &auditHandler{
		next:        next,
		auditWriter: auditWriter,
	}
}

type auditHandler struct {
	next        http.Handler
	auditWriter *LogWriter
}

func (h auditHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if h.auditWriter == nil {
		h.next.ServeHTTP(rw, req)
		return
	}

	user := GetUserInfo(req)

	context := context.WithValue(req.Context(), userKey, user)
	req = req.WithContext(context)

	auditLog, err := new(h.auditWriter, req)
	if err != nil {
		util.ReturnHTTPError(rw, req, 500, err.Error())
		return
	}

	wr := &wrapWriter{ResponseWriter: rw, auditWriter: h.auditWriter, statusCode: http.StatusOK}
	h.next.ServeHTTP(wr, req)

	auditLog.write(user, req.Header, wr.Header(), wr.statusCode, wr.buf.Bytes())
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
	return nil, nil, errors.New("the ResponseWriter doesn't support the Hijacker interface")
}
