package audit

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/pborman/uuid"
	auditlogv1 "github.com/rancher/rancher/pkg/apis/auditlog.cattle.io/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
)

const (
	contentTypeJSON = "application/json"
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

	RequestBody  []byte `json:"requestBody,omitempty"`
	ResponseBody []byte `json:"responseBody,omitempty"`

	unmarsheldRequestBody  map[string]any
	unmarsheldResponseBody map[string]any
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

		RequestBody:  nil,
		ResponseBody: rw.buf.Bytes(),
	}

	contentType := req.Header.Get("Content-Type")

	if isLoginRequest(req.RequestURI) {
		if methodsWithBody[req.Method] && strings.HasPrefix(contentType, contentTypeJSON) {
			// todo: determine if we need this info
			body, err := io.ReadAll(req.Body)
			if err != nil {
				return nil, fmt.Errorf("failed to read request body: %w", err)
			}

			req.Body = io.NopCloser(bytes.NewBuffer(body))

			if loginName := getUserNameForBasicLogin(body); loginName != "" {
				log.UserLoginName = loginName
			}

			log.RequestBody = body
		}
	}

	return log, nil
}

func (l *log) applyVerbosity(verbosity auditlogv1.LogVerbosity) {
	if !verbosity.Request.Headers {
		l.RequestHeader = nil
	}

	if !verbosity.Response.Headers {
		l.ResponseHeader = nil
	}

	if !verbosity.Request.Body {
		l.RequestBody = nil
	}

	if !verbosity.Response.Body {
		l.ResponseBody = nil
	}
}
