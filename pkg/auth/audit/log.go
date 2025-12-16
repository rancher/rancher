package audit

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
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
	contentEncodingGZIP = "gzip"
	contentEncodingZLib = "deflate"

	contentTypeJSON = "application/json"

	auditLogErrorKey = "auditLogError"
)

var (
	methodsWithBody = map[string]bool{
		http.MethodPut:  true,
		http.MethodPost: true,
	}
)

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
	rawData, err := io.ReadAll(readCloser)
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

type logEntry struct {
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
}

func copyReqBody(req *http.Request, keepBody bool) ([]byte, string) {
	contentType := req.Header.Get("Content-Type")

	if !methodsWithBody[req.Method] || !strings.HasPrefix(contentType, contentTypeJSON) {
		return nil, ""
	}

	isLoginEndpoint := isLoginRequest(req)
	shouldReadBody := isLoginEndpoint || keepBody

	if !shouldReadBody {
		// Don't read - let it stream
		return nil, ""
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		body, err = json.Marshal(map[string]any{
			"responseReadError": err.Error(),
		})
		if err != nil {
			body = []byte(`{"responseReadError": "failed to read response body"}`)
		}
	}

	req.Body = io.NopCloser(bytes.NewBuffer(body))

	var user string
	if isLoginEndpoint {
		if loginName := getUserNameForBasicLogin(body); loginName != "" {
			user = loginName
		}
	}

	if keepBody {
		return body, user
	}

	return nil, user
}

func newLog(
	verbosity auditlogv1.LogVerbosity,
	userInfo *User,
	req *http.Request,
	rw *wrapWriter,
	reqTimestamp string,
	respTimestamp string,
	rawBody []byte,
	userName string,
) *logEntry {
	log := &logEntry{
		AuditID:       k8stypes.UID(uuid.NewRandom().String()),
		RequestURI:    req.RequestURI,
		User:          userInfo,
		Method:        req.Method,
		RemoteAddr:    req.RemoteAddr,
		ResponseCode:  rw.statusCode,
		UserLoginName: userName,

		RequestTimestamp:  reqTimestamp,
		ResponseTimestamp: respTimestamp,
	}

	if verbosity.Request.Headers {
		log.RequestHeader = req.Header.Clone()
	}

	// Attempt req body prep
	if verbosity.Request.Body && req.Header.Get("Content-Type") == contentTypeJSON && len(rawBody) > 0 {
		if err := json.Unmarshal(rawBody, &log.RequestBody); err != nil {
			log.RequestBody = map[string]any{
				auditLogErrorKey: fmt.Sprintf("failed to unmarshal request body: %s", err.Error()),
			}
		}
	}

	if verbosity.Response.Headers {
		log.ResponseHeader = rw.Header().Clone()
	}

	// Attempt res body prep
	if verbosity.Response.Body {
		log.prepareResponseBody(rw.Header(), rw.buf.Bytes())
	}

	return log
}

func (l *logEntry) prepareResponseBody(resHeaders http.Header, body []byte) {
	if resHeaders.Get("Content-Type") == contentTypeJSON && len(body) > 0 {
		decompressed, err := decompressResponse(resHeaders.Get("Content-Encoding"), body)
		if err != nil {
			l.ResponseBody = map[string]any{
				auditLogErrorKey: fmt.Sprintf("failed to decompress response body: %s", err),
			}
			return
		}

		if jsonErr := json.Unmarshal(decompressed, &l.ResponseBody); jsonErr != nil {
			l.ResponseBody = map[string]any{
				auditLogErrorKey: fmt.Sprintf("failed to unmarshal response body: %s", jsonErr.Error()),
			}
		}
	}
}

func decompressResponse(encoding string, rawResponseBody []byte) ([]byte, error) {
	var err error
	var decompressed []byte

	switch encoding {
	case "", "none":
		// not encoded do nothing
		return rawResponseBody, nil
	case contentEncodingGZIP:
		decompressed, err = decompressGZIP(rawResponseBody)
	case contentEncodingZLib:
		decompressed, err = decompressZLib(rawResponseBody)
	default:
		err = fmt.Errorf("%w '%s' in response header", ErrUnsupportedEncoding, encoding)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to decode response body: %w", err)
	}

	return decompressed, nil
}
