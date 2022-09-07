// Package audit is used to preform audit logging.
package audit

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/pborman/uuid"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apiserver/pkg/endpoints/request"
)

const (
	contentTypeJSON     = "application/json"
	contentEncodingGZIP = "gzip"
	contentEncodingZLib = "deflate"
	redacted            = "[redacted]"
)

// Level represents a desired logging level.
type Level int

const (
	// LevelNull default value.
	LevelNull Level = iota
	// LevelMetadata log request header information.
	LevelMetadata
	// LevelRequest log metadata and request body.
	LevelRequest
	// LevelRequestResponse log metadata request body and response header and body.
	LevelRequestResponse
)

var (
	bodyMethods = map[string]bool{
		http.MethodPut:  true,
		http.MethodPost: true,
	}
	sensitiveRequestHeader  = []string{"Cookie", "Authorization"}
	sensitiveResponseHeader = []string{"Cookie", "Set-Cookie"}
	// ErrUnsupportedEncoding is returned when the response encoding is unsupported
	ErrUnsupportedEncoding = fmt.Errorf("unsupported encoding")
)

type auditLog struct {
	log                *log
	writer             *LogWriter
	reqBody            []byte
	keysToConcealRegex *regexp.Regexp
}

type log struct {
	AuditID           k8stypes.UID `json:"auditID,omitempty"`
	RequestURI        string       `json:"requestURI,omitempty"`
	User              *User        `json:"user,omitempty"`
	Method            string       `json:"method,omitempty"`
	RemoteAddr        string       `json:"remoteAddr,omitempty"`
	RequestTimestamp  string       `json:"requestTimestamp,omitempty"`
	ResponseTimestamp string       `json:"responseTimestamp,omitempty"`
	ResponseCode      int          `json:"responseCode,omitempty"`
	RequestHeader     http.Header  `json:"requestHeader,omitempty"`
	ResponseHeader    http.Header  `json:"responseHeader,omitempty"`
	RequestBody       []byte       `json:"requestBody,omitempty"`
	ResponseBody      []byte       `json:"responseBody,omitempty"`
	UserLoginName     string       `json:"userLoginName,omitempty"`
}

var userKey struct{}

// User holds information about the user who caused the audit log
type User struct {
	Name  string              `json:"name,omitempty"`
	Group []string            `json:"group,omitempty"`
	Extra map[string][]string `json:"extra,omitempty"`
	// RequestUser is the --as user
	RequestUser string `json:"requestUser,omitempty"`
	// RequestGroups is the --as-group list
	RequestGroups []string `json:"requestGroups,omitempty"`
}

func getUserInfo(req *http.Request) *User {
	user, _ := request.UserFrom(req.Context())
	return &User{
		Name:  user.GetName(),
		Group: user.GetGroups(),
		Extra: user.GetExtra(),
	}
}

func getUserNameForBasicLogin(body []byte) string {
	input := &v32.BasicLogin{}
	err := json.Unmarshal(body, input)
	if err != nil {
		logrus.Debugf("error unmarshalling input, cannot add login info to audit log: %v", err)
		return ""
	}
	return input.Username
}

// FromContext gets the user information from the given context.
func FromContext(ctx context.Context) (*User, bool) {
	u, ok := ctx.Value(userKey).(*User)
	return u, ok
}

func newAuditLog(writer *LogWriter, req *http.Request, keysToConcealRegex *regexp.Regexp) (*auditLog, error) {
	auditLog := &auditLog{
		writer: writer,
		log: &log{
			AuditID:          k8stypes.UID(uuid.NewRandom().String()),
			RequestURI:       req.RequestURI,
			Method:           req.Method,
			RemoteAddr:       req.RemoteAddr,
			RequestTimestamp: time.Now().Format(time.RFC3339),
		},
		keysToConcealRegex: keysToConcealRegex,
	}

	contentType := req.Header.Get("Content-Type")
	loginReq := isLoginRequest(req.RequestURI)
	if writer.Level >= LevelRequest || loginReq {
		if bodyMethods[req.Method] && strings.HasPrefix(contentType, contentTypeJSON) {
			reqBody, err := readBodyWithoutLosingContent(req)
			if err != nil {
				return nil, err
			}
			if loginReq {
				loginName := getUserNameForBasicLogin(reqBody)
				if loginName != "" {
					auditLog.log.UserLoginName = loginName
				}
			}
			if writer.Level >= LevelRequest {
				auditLog.reqBody = reqBody
			}
		}
	}
	return auditLog, nil
}

func (a *auditLog) write(userInfo *User, reqHeaders, resHeaders http.Header, resCode int, resBody []byte) error {
	a.log.User = userInfo
	a.log.ResponseTimestamp = time.Now().Format(time.RFC3339)
	a.log.RequestHeader = filterOutHeaders(reqHeaders, sensitiveRequestHeader)
	a.log.ResponseHeader = filterOutHeaders(resHeaders, sensitiveResponseHeader)
	a.log.ResponseCode = resCode

	if a.log.UserLoginName != "" {
		if a.log.User.Extra == nil {
			a.log.User.Extra = make(map[string][]string)
		}
		a.log.User.Extra["username"] = []string{a.log.UserLoginName}
		logrus.Debugf("Added username for login request to audit log %v", a.log.UserLoginName)
	}

	var buffer bytes.Buffer

	alByte, err := json.Marshal(a.log)
	if err != nil {
		return fmt.Errorf("failed to marshal log message: %w", err)
	}

	buffer.Write(bytes.TrimSuffix(alByte, []byte("}")))
	a.writeRequest(&buffer)

	if err = a.writeResponse(&buffer, resHeaders, resBody); err != nil {
		return err
	}

	buffer.WriteString("}")

	var compactBuffer bytes.Buffer
	err = json.Compact(&compactBuffer, buffer.Bytes())
	if err != nil {

		return fmt.Errorf("failed to compact audit log: %w", err)
	}

	compactBuffer.WriteString("\n")

	_, err = a.writer.Output.Write(compactBuffer.Bytes())
	if err != nil {
		return fmt.Errorf("failed to write log to output: %w", err)
	}

	return nil
}

// writeRequest attempts to write the API request to the log message.
func (a *auditLog) writeRequest(buf *bytes.Buffer) {
	if a.writer.Level < LevelRequest || len(a.reqBody) == 0 {
		return
	}

	buf.WriteString(`,"requestBody":`)
	buf.Write(bytes.TrimSuffix(a.concealSensitiveData(a.log.RequestURI, a.reqBody), []byte("\n")))
}

// writeResponse attempt to write the API response to the log message.
func (a *auditLog) writeResponse(buf *bytes.Buffer, resHeaders http.Header, resBody []byte) (err error) {
	if a.writer.Level < LevelRequestResponse || resHeaders.Get("Content-Type") != contentTypeJSON || len(resBody) == 0 {
		return nil
	}

	switch resHeaders.Get("Content-Encoding") {
	case contentEncodingGZIP:
		resBody, err = decompressGZIP(resBody)
	case contentEncodingZLib:
		resBody, err = decompressZLib(resBody)
	case "none":
		// do nothing message is not encoded
	case "":
		// do nothing message is not encoded
	default:
		err = fmt.Errorf("%w '%s' in response header", ErrUnsupportedEncoding, resHeaders.Get("Content-Encoding"))
	}

	if err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	buf.WriteString(`,"responseBody":`)
	buf.Write(bytes.TrimSuffix(a.concealSensitiveData(a.log.RequestURI, resBody), []byte("\n")))

	return nil
}

func isLoginRequest(uri string) bool {
	return strings.Contains(uri, "?action=login")
}

func readBodyWithoutLosingContent(req *http.Request) ([]byte, error) {
	if !bodyMethods[req.Method] {
		return nil, nil
	}

	bodyBytes, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}
	req.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))

	return bodyBytes, nil
}

func filterOutHeaders(headers http.Header, filterKeys []string) map[string][]string {
	newHeader := make(map[string][]string)
	for k, v := range headers {
		if isExist(filterKeys, k) {
			continue
		}
		newHeader[k] = v
	}
	return newHeader
}

func isExist(array []string, key string) bool {
	for _, v := range array {
		if v == key {
			return true
		}
	}
	return false
}

func (a *auditLog) concealSensitiveData(requestURI string, body []byte) []byte {
	var m map[string]interface{}
	if err := json.Unmarshal(body, &m); err != nil {
		return body
	}

	var changed bool
	// Conceal values of secret data.
	if strings.Contains(requestURI, "secrets") {
		dataKey := "data"
		data, _ := m[dataKey].(map[string]interface{})
		if data == nil {
			dataKey = "stringData"
			data, _ = m[dataKey].(map[string]interface{})
		}

		for key := range data {
			data[key] = redacted
		}
		if data != nil {
			changed = true
			m[dataKey] = data
		}
	}

	// Conceal values for data considered sensitive: passwords, tokens, etc.
	if !a.concealMap(m) && !changed {
		return body
	}

	newBody, err := json.Marshal(m)
	if err != nil {
		return body
	}
	return newBody
}

func (a *auditLog) concealMap(m map[string]interface{}) bool {
	var changed bool
	for key := range m {
		if _, ok := m[key].(string); ok {
			if a.keysToConcealRegex.MatchString(key) {
				changed = true
				m[key] = redacted
			}
		} else if nested, ok := m[key].(map[string]interface{}); ok && a.concealMap(nested) {
			changed = true
			m[key] = nested
		}
	}

	return changed
}

func decompressGZIP(data []byte) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}

	return decompress(gz)
}

func decompressZLib(data []byte) ([]byte, error) {
	zr, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create zlib reader: %w", err)
	}

	return decompress(zr)
}

func decompress(readCloser io.ReadCloser) ([]byte, error) {
	rawData, err := ioutil.ReadAll(readCloser)
	if err != nil {
		retErr := fmt.Errorf("failed to read compressed response: %w", err)
		closeErr := readCloser.Close()
		if closeErr != nil {
			// Using %v for close error because you can currently only wrap one error.
			// The read error is more important to the caller in this instance.
			retErr = fmt.Errorf("%w; failed to close readCloser: %v", retErr, closeErr)
		}
		return nil, retErr
	}

	if err = readCloser.Close(); err != nil {
		return rawData, fmt.Errorf("failed to close reader: %w", err)
	}

	return rawData, nil
}
