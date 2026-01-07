package audit

import (
	"context"
	"net/http"
	"time"

	auditlogv1 "github.com/rancher/rancher/pkg/apis/auditlog.cattle.io/v1"
)

func GetAuditLoggerMiddleware(auditLog *LoggingHandler) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			if auditLog == nil || auditLog.writer == nil {
				next.ServeHTTP(rw, req)
				return
			}

			reqTimestamp := time.Now().Format(time.RFC3339)
			user := getUserInfo(req)
			context := context.WithValue(req.Context(), userKeyValue, user)
			req = req.WithContext(context)
			keepReqBody := auditLog.level >= auditlogv1.LevelRequest
			rawReqBody, userName := copyReqBody(req, keepReqBody)

			// keepResBody determines whether to buffer response bodies for audit logging.
			// Note: Buffering large responses (e.g., cluster lists) can consume significant
			// memory (MBs-GBs). Only enable LevelRequestResponse if response body logging is required.
			keepResBody := auditLog.level >= auditlogv1.LevelRequestResponse
			wrappedRw := &wrapWriter{
				ResponseWriter: rw,
				keepBody:       keepResBody,
				headerWrote:    false,
				statusCode:     http.StatusTeapot, // Default status should never matter so it can be nonsense; controversial, but it serves as our canary in the coal mine. If we see teapots we KNOW we have bugs somewhere.
			}

			next.ServeHTTP(wrappedRw, req)
			if wrappedRw.hijacked {
				return
			}

			respTimestamp := time.Now().Format(time.RFC3339)

			verbosityLevel := auditLog.ResolveVerbosity(req.RequestURI)
			auditLogEntry := newLog(verbosityLevel, user, req, wrappedRw, reqTimestamp, respTimestamp, rawReqBody, userName)
			auditLog.Write(auditLogEntry)
		})
	}
}
