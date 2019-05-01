package responsewriter

import (
	"compress/gzip"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

// All other writers will attempt additional unnecessary logic
// Implements http.responseWriter and io.Writer
type DummyWriter struct {
	header map[string][]string
	buffer []byte
}

type DummyHandler struct {
}

type DummyHandlerWithWrite struct {
	DummyHandler
	next http.Handler
}

func NewDummyWriter() *DummyWriter {
	return &DummyWriter{map[string][]string{}, []byte{}}
}

func NewRequest(accept string) *http.Request {
	return &http.Request{
		Header: map[string][]string{"Accept-Encoding": {accept}},
	}
}

func (d *DummyWriter) Header() http.Header {
	return d.header
}

func (d *DummyWriter) Write(p []byte) (n int, err error) {
	d.buffer = append(d.buffer, p...)
	return 0, nil
}

func (d *DummyWriter) WriteHeader(int) {
}

func (d *DummyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
}

func (d *DummyHandlerWithWrite) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte{0, 0})
	if d.next != nil {
		d.next.ServeHTTP(w, r)
	}
}

// TestWriteHeader asserts content-length header is deleted and content-encoding header is set to gzip
func TestWriteHeader(t *testing.T) {
	assert := assert.New(t)

	w := NewDummyWriter()
	gz := &gzipResponseWriter{gzip.NewWriter(w), w}

	gz.Header().Set("Content-Length", "80")
	gz.WriteHeader(400)
	// Content-Length should have been deleted in WriterHeader, resulting in empty string
	assert.Equal("", gz.Header().Get("Content-Length"))
	assert.Equal(1, len(w.header["Content-Encoding"]))
	assert.Equal("gzip", gz.Header().Get("Content-Encoding"))
}

// TestSetContentWithoutWrite asserts content-encoding is NOT "gzip" if accept-encoding header does not contain gzip
func TestSetContentWithoutWrite(t *testing.T) {
	assert := assert.New(t)

	// Test content encoding header when write is not used
	handlerFunc := Gzip(&DummyHandler{})

	// Test when accept-encoding only contains gzip
	rw := NewDummyWriter()
	req := NewRequest("gzip")
	handlerFunc.ServeHTTP(rw, req)
	// Content encoding should be empty since write has not been used
	assert.Equal(0, len(rw.header["Content-Encoding"]))
	assert.Equal("", rw.Header().Get("Content-Encoding"))

	// Test when accept-encoding contains multiple options, including gzip
	rw = NewDummyWriter()
	req = NewRequest("json, xml, gzip")
	handlerFunc.ServeHTTP(rw, req)
	assert.Equal(0, len(rw.header["Content-Encoding"]))
	assert.Equal("", rw.Header().Get("Content-Encoding"))

	// Test when accept-encoding is empty
	req = NewRequest("")
	rw = NewDummyWriter()
	handlerFunc.ServeHTTP(rw, req)
	assert.Equal(0, len(rw.header["Content-Encoding"]))
	assert.Equal("", rw.Header().Get("Content-Encoding"))

	// Test when accept-encoding is is not empty but does not include gzip
	req = NewRequest("json, xml")
	rw = NewDummyWriter()
	handlerFunc.ServeHTTP(rw, req)
	assert.Equal(0, len(rw.header["Content-Encoding"]))
	assert.Equal("", rw.Header().Get("Content-Encoding"))
}

// TestSetContentWithWrite asserts content-encoding is "gzip" if accept-encoding header contains gzip
func TestSetContentWithWrite(t *testing.T) {
	assert := assert.New(t)

	// Test content encoding header when write is used
	handlerFunc := Gzip(&DummyHandlerWithWrite{})

	// Test when accept-encoding only contains gzip
	req := NewRequest("gzip")
	rw := NewDummyWriter()
	handlerFunc.ServeHTTP(rw, req)
	// Content encoding should be gzip since write has been used
	assert.Equal(1, len(rw.header["Content-Encoding"]))
	assert.Equal("gzip", rw.Header().Get("Content-Encoding"))

	// Test when accept-encoding contains multiple options, including gzip
	req = NewRequest("json, xml, gzip")
	rw = NewDummyWriter()
	handlerFunc.ServeHTTP(rw, req)
	// Content encoding should be gzip since write has been used
	assert.Equal(1, len(rw.header["Content-Encoding"]))
	assert.Equal("gzip", rw.Header().Get("Content-Encoding"))

	// Test when accept-encoding is empty
	req = NewRequest("")
	rw = NewDummyWriter()
	handlerFunc.ServeHTTP(rw, req)
	// Content encoding should be empty since gzip is not an accepted encoding
	assert.Equal(0, len(rw.header["Content-Encoding"]))
	assert.Equal("", rw.Header().Get("Content-Encoding"))

	// Test when accept-encoding is is not empty but does not include gzip
	req = NewRequest("json, xml")
	rw = NewDummyWriter()
	handlerFunc.ServeHTTP(rw, req)
	// Content encoding should be empty since gzip is not an accepted encoding
	assert.Equal(0, len(rw.header["Content-Encoding"]))
	assert.Equal("", rw.Header().Get("Content-Encoding"))
}

// TestMultipleWrites ensures that Write can be used multiple times
func TestMultipleWrites(t *testing.T) {
	assert := assert.New(t)

	// Handler function that contains one writing handler
	handlerFuncOneWrite := Gzip(&DummyHandlerWithWrite{})

	// Handler function that contains a chain of two writing handlers
	handlerFuncTwoWrites := Gzip(&DummyHandlerWithWrite{next: &DummyHandlerWithWrite{}})

	req := NewRequest("gzip")
	rw := NewDummyWriter()
	handlerFuncOneWrite.ServeHTTP(rw, req)
	oneWriteResult := rw.buffer

	req = NewRequest("gzip")
	rw = NewDummyWriter()
	handlerFuncTwoWrites.ServeHTTP(rw, req)
	multiWriteResult := rw.buffer

	// Content encoding should be gzip since write has been used (twice)
	assert.Equal(1, len(rw.header["Content-Encoding"]))
	assert.Equal("gzip", rw.Header().Get("Content-Encoding"))
	assert.NotEqual(multiWriteResult, oneWriteResult)
}
