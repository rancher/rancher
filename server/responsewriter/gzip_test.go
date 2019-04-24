package responsewriter

import (
	"compress/gzip"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

// all other writers will attempt additional unnecessary logic
// this implements http.responseWriter and io.Writer
type DummyWriter struct {
	header map[string][]string
}

type DummyHandler struct {
}

func NewDummyWriter() *DummyWriter {
	return &DummyWriter{map[string][]string{}}
}

func (d *DummyWriter) Header() http.Header {
	return d.header
}

func (d *DummyWriter) Write(p []byte) (n int, err error) {
	return 0, nil
}

func (d *DummyWriter) WriteHeader(int) {
}

func (d *DummyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
}

// WriteHeader should delete current content-length in header
func TestWriteHeader(t *testing.T) {
	w := NewDummyWriter()
	gz := &gzipResponseWriter{gzip.NewWriter(w), w}

	gz.Header().Set("Content-Length", "80")
	gz.WriteHeader(400)
	// Content-Length should have been deleted in WriterHeader, resulting in empty string
	assert.Equal(t, "", gz.Header().Get("Content-Length"))
	assert.Equal(t, "gzip", gz.Header().Get("Content-Encoding"))
}

// Gzip handler function should set content-type to "gzip" if accept-encoding header contains gzip
func TestHandlerSetContent(t *testing.T) {
	rw := NewDummyWriter()
	handler := &DummyHandler{}
	req := &http.Request{}

	req.Header = map[string][]string{}
	handlerFunc := Gzip(handler)

	// test when accept-encoding only contains gzip
	req.Header.Set("Accept-Encoding", "gzip")
	handlerFunc.ServeHTTP(rw, req)
	assert.Equal(t, "gzip", rw.Header().Get("Content-Encoding"))

	// test when accept-encoding contains multiple options, including gzip
	req.Header.Set("Accept-Encoding", "json, xml, gzip")
	handlerFunc.ServeHTTP(rw, req)
	assert.Equal(t, "gzip", rw.Header().Get("Content-Encoding"))
}

// Gzip handler function should not change content-type if accept encoding does not contain gzip
func TestHandlerForNonGzip(t *testing.T) {
	rw := NewDummyWriter()
	handler := &DummyHandler{}
	req := &http.Request{}

	req.Header = map[string][]string{}
	handlerFunc := Gzip(handler)

	// test when there is no Accept-Encoding header
	handlerFunc.ServeHTTP(rw, req)
	assert.NotEqual(t, "gzip", rw.Header().Get("Content-Encoding"))

	// test when there are multiple Accept-Encoding header values, none of which are gzip
	req.Header.Set("Accept-Encoding", "json, xml")
	handlerFunc.ServeHTTP(rw, req)
	assert.NotEqual(t, "gzip", rw.Header().Get("Content-Encoding"))
}
