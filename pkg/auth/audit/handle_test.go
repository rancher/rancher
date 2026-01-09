package audit

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"
)

func TestAuditLogMiddleware(t *testing.T) {
	var buf bytes.Buffer
	writerOpts := WriterOptions{}
	dummyW, err := NewWriter(&buf, writerOpts)
	assert.NoError(t, err)

	readLog := func(t *testing.T) *logEntry {
		t.Helper()

		var log logEntry
		err := json.Unmarshal(buf.Bytes(), &log)
		assert.NoError(t, err)
		buf.Reset()
		return &log
	}

	withUserMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := request.WithUser(req.Context(), &user.DefaultInfo{})
			req = req.WithContext(ctx)
			next.ServeHTTP(w, req)
		})
	}
	auditMiddleware := NewAuditLogMiddleware(dummyW)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte(`hello world`))
	})
	test2Handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	test3Handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`hello`))
		w.(http.Flusher).Flush()
		w.Write([]byte(`world`))
		w.(http.Flusher).Flush()
	})
	test4Handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte(`hello`))
		w.(http.Flusher).Flush()
		w.Write([]byte(`world`))
		w.(http.Flusher).Flush()
	})
	test5Handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.WriteHeader(http.StatusUnauthorized)
	})
	test6Handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		upgrader := websocket.Upgrader{}
		conn, err := upgrader.Upgrade(w, req, nil)
		assert.NoError(t, err)
		conn.Close()
	})

	mux := http.NewServeMux()
	mux.Handle("/foo", withUserMiddleware(auditMiddleware(testHandler)))
	mux.Handle("/bar", withUserMiddleware(auditMiddleware(test2Handler)))
	mux.Handle("/baz", withUserMiddleware(auditMiddleware(test3Handler)))
	mux.Handle("/toto", withUserMiddleware(auditMiddleware(test4Handler)))
	mux.Handle("/hello", withUserMiddleware(auditMiddleware(test5Handler)))
	mux.Handle("/ws", withUserMiddleware(auditMiddleware(test6Handler)))

	ts := httptest.NewServer(mux)
	defer ts.Close()

	res, err := http.Get(ts.URL + "/foo")
	assert.NoError(t, err)
	defer res.Body.Close()
	assert.Equal(t, http.StatusOK, res.StatusCode)
	log1 := readLog(t)
	assert.Equal(t, http.StatusOK, log1.ResponseCode)

	res, err = http.Get(ts.URL + "/bar")
	assert.NoError(t, err)
	defer res.Body.Close()
	assert.Equal(t, http.StatusNoContent, res.StatusCode)
	log2 := readLog(t)
	assert.Equal(t, http.StatusNoContent, log2.ResponseCode)

	res, err = http.Get(ts.URL + "/baz")
	assert.NoError(t, err)
	defer res.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, res.StatusCode)
	_, err = io.ReadAll(res.Body)
	assert.NoError(t, err)
	log3 := readLog(t)
	assert.Equal(t, http.StatusUnauthorized, log3.ResponseCode)

	res, err = http.Get(ts.URL + "/toto")
	assert.NoError(t, err)
	defer res.Body.Close()
	assert.Equal(t, http.StatusOK, res.StatusCode)
	_, err = io.ReadAll(res.Body)
	assert.NoError(t, err)
	log4 := readLog(t)
	assert.Equal(t, http.StatusOK, log4.ResponseCode)

	res, err = http.Get(ts.URL + "/hello")
	assert.NoError(t, err)
	defer res.Body.Close()
	assert.Equal(t, http.StatusOK, res.StatusCode)
	log5 := readLog(t)
	assert.Equal(t, http.StatusOK, log5.ResponseCode)

	wsURL := strings.Replace(ts.URL, "http", "ws", 1)
	c, res, err := websocket.DefaultDialer.Dial(wsURL+"/ws", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusSwitchingProtocols, res.StatusCode)
	c.Close()

	assert.Empty(t, buf.Bytes())
}
