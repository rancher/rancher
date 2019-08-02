package websocket

import (
	"fmt"
	"net/http"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

type testResponseWriter struct {
	header http.Header
}

type testHandler struct {
}

func serveHTTPWithHeader(port string, origins []string, connections []string, uas []string) int {
	testRW := newTestResponseWriter()
	testReq := newTestRequest(port, origins, connections, uas)
	testWSHandler := NewWebsocketHandler(&testHandler{})
	testWSHandler.ServeHTTP(testRW, testReq)

	if statusCode := testRW.header["code"]; len(statusCode) == 0 {
		return -1
	}

	code, err := strconv.Atoi(testRW.header["code"][0])
	if err != nil {
		return -1
	}

	return code
}

func newTestResponseWriter() *testResponseWriter {
	return &testResponseWriter{
		header: http.Header(make(map[string][]string)),
	}
}

func newTestRequest(port string, origins []string, connections []string, uas []string) *http.Request {
	return &http.Request{
		Header: http.Header(
			map[string][]string{
				"Origin":     origins,
				"Connection": connections,
				"User-Agent": uas,
			},
		),
		Host: fmt.Sprintf("rancher%s", port),
	}
}

// TestServeHTTP tests the websocket handler using various values for relevant header fields
func TestServeHTTP(t *testing.T) {
	assert := assert.New(t)

	// if origin is a websocket request and contains "mozilla" then the origin request origin header must match the request host
	assert.Equal(403, serveHTTPWithHeader("", []string{"asdf"}, []string{"upgrade"}, []string{"dsafmozillaasdf"}))
	assert.Equal(403, serveHTTPWithHeader("", []string{"asdf"}, []string{"upgrade"}, []string{"mozilla"}))
	assert.Equal(403, serveHTTPWithHeader(":3000", []string{"https://rancher"}, []string{"upgrade"}, []string{"dsafmozillaasdf"}))
	assert.Equal(403, serveHTTPWithHeader("", []string{"https://rancher:3000"}, []string{"upgrade"}, []string{"dsafmozillaasdf"}))
	assert.Equal(403, serveHTTPWithHeader("", []string{""}, []string{"upgrade"}, []string{"dsafmozillaasdf"}))
	assert.Equal(200, serveHTTPWithHeader(":3000", []string{"https://rancher:3000"}, []string{"upgrade"}, []string{"asdf"}))
	assert.Equal(200, serveHTTPWithHeader(":3000", []string{"https://rancher:3000"}, []string{}, []string{"mozilla"}))
	assert.Equal(200, serveHTTPWithHeader("", []string{"asdf"}, []string{"upgrade"}, []string{"somthingelse"}))
	assert.Equal(200, serveHTTPWithHeader("", []string{"https://rancher"}, []string{"upgrade"}, []string{"mozilla"}))
	assert.Equal(200, serveHTTPWithHeader(":3000", []string{"https://rancher:3000"}, []string{"upgrade"}, []string{"mozilla"}))
	assert.Equal(200, serveHTTPWithHeader(":3000", []string{"https://rancher:3000"}, []string{"upgrade"}, []string{"dsafmozillaasdf"}))
}

func (trw *testResponseWriter) Header() http.Header {
	return trw.header
}

func (trw *testResponseWriter) Write([]byte) (int, error) {
	return 0, nil
}

func (trw *testResponseWriter) WriteHeader(statusCode int) {
	trw.header["code"] = []string{strconv.Itoa(statusCode)}
}

func (th *testHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	rw.WriteHeader(200)
}
