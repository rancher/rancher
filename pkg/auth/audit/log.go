package audit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/pborman/uuid"
	auditlogv1 "github.com/rancher/rancher/pkg/apis/auditlog.cattle.io/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
)

const (
	contentTypeJSON  = "application/json"
	auditLogErrorKey = "auditLogError"
)

var (
	methodsWithBody = map[string]bool{
		http.MethodPut:  true,
		http.MethodPost: true,
	}
)

type log struct {
	AuditID       k8stypes.UID `json:"auditID,omitempty"`
	RequestURI    string       `json:"requestURI,omitempty"`
	User          *User        `json:"user,omitempty"`
	Method        string       `json:"method,omitempty"`
	RemoteAddr    string       `json:"remoteAddr,omitempty"`
	ResponseCode  int          `json:"responseCode,omitempty"`
	UserLoginName string       `json:"userLoginName,omitempty"`

	RequestTimestamp  string `json:"requestTimestamp,omitempty"`
	ResponseTimestamp string `json:"responseTimestamp,omitempty"`

	RequestHeader  http.Header `json:"requestHeader,omitempty"`
	ResponseHeader http.Header `json:"responseHeader,omitempty"`

	RequestBody  map[string]any `json:"requestBody,omitempty"`
	ResponseBody map[string]any `json:"responseBody,omitempty"`

	rawRequestBody  []byte
	rawResponseBody []byte
}

func newLog(userInfo *User, req *http.Request, rw *wrapWriter, reqTimestamp string, respTimestamp string) (*log, error) {
	log := &log{
		AuditID:       k8stypes.UID(uuid.NewRandom().String()),
		RequestURI:    req.RequestURI,
		User:          userInfo,
		Method:        req.Method,
		RemoteAddr:    req.RemoteAddr,
		ResponseCode:  rw.statusCode,
		UserLoginName: "",

		RequestTimestamp:  reqTimestamp,
		ResponseTimestamp: respTimestamp,

		RequestHeader:  req.Header,
		ResponseHeader: rw.Header(),

		rawResponseBody: rw.buf.Bytes(),
	}

	contentType := req.Header.Get("Content-Type")

	if methodsWithBody[req.Method] && strings.HasPrefix(contentType, contentTypeJSON) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read request body: %w", err)
		}

		req.Body = io.NopCloser(bytes.NewBuffer(body))

		if loginName := getUserNameForBasicLogin(body); loginName != "" {
			log.UserLoginName = loginName
		}

		log.rawRequestBody = body
	}

	return log, nil
}

func (l *log) prepare(verbosity auditlogv1.LogVerbosity) {
	if !verbosity.Request.Headers {
		l.RequestHeader = nil
	}

	if !verbosity.Response.Headers {
		l.ResponseHeader = nil
	}

	if verbosity.Request.Body && len(l.rawRequestBody) > 0 {
		if err := json.Unmarshal(l.rawRequestBody, &l.RequestBody); err != nil {
			l.RequestBody = map[string]any{
				auditLogErrorKey: fmt.Sprintf("failed to unmarshal request body: %s", err.Error()),
			}
		}
	}
	l.rawRequestBody = nil

	if verbosity.Response.Body && len(l.rawResponseBody) > 0 {
		if err := json.Unmarshal(l.rawResponseBody, &l.ResponseBody); err != nil {
			l.ResponseBody = map[string]any{
				auditLogErrorKey: fmt.Sprintf("failed to unmarshal response body: %s", err.Error()),
			}
		}
	}
	l.rawResponseBody = nil
}
