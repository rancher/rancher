package middleware

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"net"
	"net/http"
	"reflect"
	"strings"
)

type wrapWriter struct {
	gzipResponseWriter

	code int
}

type gzipResponseWriter struct {
	io.Writer
	http.ResponseWriter
}

func (g gzipResponseWriter) Write(b []byte) (int, error) {
	// Header logic is kept here in case the user does not use WriteHeader
	g.Header().Set("Content-Encoding", "gzip")
	g.Header().Del("Content-Length")

	return g.Writer.Write(b)
}

// Close uses gzip to write gzip footer if message is gzip encoded
func (g gzipResponseWriter) Close(writer *gzip.Writer) {
	if g.Header().Get("Content-Encoding") == "gzip" {
		writer.Close()
	}
}

// WriteHeader sets gzip encoding and removes length. Should always be used when using gzip writer.
func (g gzipResponseWriter) WriteHeader(statusCode int) {
	g.Header().Set("Content-Encoding", "gzip")
	g.Header().Del("Content-Length")
	g.ResponseWriter.WriteHeader(statusCode)
}

// Gzip creates a gzip writer if gzip encoding is accepted.
func Gzip(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			handler.ServeHTTP(w, r)
			return
		}
		gz := gzip.NewWriter(w)

		gzw := &wrapWriter{gzipResponseWriter{Writer: gz, ResponseWriter: w}, http.StatusOK}
		defer gzw.Close(gz)

		// Content encoding will be set once Write or WriteHeader is called, to avoid gzipping empty messages
		handler.ServeHTTP(gzw, r)
	})
}

// Hijack must be implemented to properly chain with handlers expecting a hijacker handler to be passed
func (g *gzipResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := g.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, fmt.Errorf("Upstream ResponseWriter of type %v does not implement http.Hijacker", reflect.TypeOf(g.ResponseWriter))
}
