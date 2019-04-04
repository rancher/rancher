package writer

import (
	"compress/gzip"
	"github.com/rancher/norman/types"
	"io"
	"net/http"
	"strings"
)

type responseNoWriter interface {
	Header() http.Header
	WriteHeader(statusCode int)
}

type GzipResponseEncoder struct {
	EncodingResponseWriter
}
type GzipResponseWriter struct {
	responseNoWriter
	noHeaderGz
}

type noHeaderGz interface {
	Write(p []byte) (int, error)
	Flush() error
	Close() error
}


func newGZ(rw http.ResponseWriter) *GzipResponseWriter {
	a := &GzipResponseWriter{
		rw,
		gzip.NewWriter(rw),
	}
	a.Flush()
	// a.Close()
	return a
}

func (g *GzipResponseEncoder) Write(apiContext *types.APIContext, code int, obj interface{}) {
	if apiContext.ResponseFormat == "json" && strings.Contains(apiContext.Request.Header.Get("Accept-Encoding"), "gzip") {
		//gz := gzip.NewWriter(apiContext.Response)
		gzrw := newGZ(apiContext.Response)

		newCtx := *apiContext
		newCtx.Response.Header().Del("Content-Length")
		newCtx.Response.Header().Set("Content-Encoding", "gzip")
		newCtx.Response = gzrw
		g.EncodingResponseWriter.Write(&newCtx, code, obj)
		gzrw.Close()

	} else {
		g.EncodingResponseWriter.Write(apiContext, code, obj)
	}
	//json.NewEncoder(gz).Encode(obj)
}

func (g *GzipResponseWriter) gzipEncoder(writer io.Writer, v interface{}) error {
	return nil
}


