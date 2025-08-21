package audit

import (
	"context"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
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
			rawBody, userName := copyReqBody(req)

			wrappedRw := &wrapWriter{
				ResponseWriter: rw,
				statusCode:     http.StatusTeapot, // Default status should never matter so it can be nonsense; controversial, but it serves as our canary in the coal mine. If we see teapots we KNOW we have bugs somewhere.
			}

			next.ServeHTTP(wrappedRw, req)
			if wrappedRw.hijacked {
				return
			}

			respTimestamp := time.Now().Format(time.RFC3339)

			auditLogEntry := newLog(user, req, wrappedRw, reqTimestamp, respTimestamp, rawBody, userName)
			if err := auditLog.writer.Write(auditLogEntry); err != nil {
				// Locking after next is called to avoid performance hits on the request.
				auditLog.errLock.Lock()
				defer auditLog.errLock.Unlock()

				// Only log duplicate error messages at most every errorDebounceTime.
				// This is to prevent the rancher logs from being flooded with error messages
				// when the log path is invalid or any other error that will always cause a write to fail.
				if lastSeen, ok := auditLog.errMap[err.Error()]; !ok || time.Since(lastSeen) > errorDebounceTime {
					logrus.Warnf("Failed to write audit log: %s", err)
					auditLog.errMap[err.Error()] = time.Now()
				}
			}

		})
	}
}
