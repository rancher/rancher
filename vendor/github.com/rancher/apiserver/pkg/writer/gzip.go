package writer

import (
	"compress/gzip"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/rancher/apiserver/pkg/types"
)

type GzipWriter struct {
	types.ResponseWriter
}

func setup(apiOp *types.APIRequest) (*types.APIRequest, io.Closer) {
	if !strings.Contains(apiOp.Request.Header.Get("Accept-Encoding"), "gzip") {
		return apiOp, ioutil.NopCloser(nil)
	}

	apiOp.Response.Header().Set("Content-Encoding", "gzip")
	apiOp.Response.Header().Del("Content-Length")

	gz := gzip.NewWriter(apiOp.Response)
	gzw := &gzipResponseWriter{Writer: gz, ResponseWriter: apiOp.Response}

	newOp := *apiOp
	newOp.Response = gzw
	return &newOp, gz
}

func (g *GzipWriter) Write(apiOp *types.APIRequest, code int, obj types.APIObject) {
	apiOp, closer := setup(apiOp)
	defer closer.Close()
	g.ResponseWriter.Write(apiOp, code, obj)
}

func (g *GzipWriter) WriteList(apiOp *types.APIRequest, code int, obj types.APIObjectList) {
	apiOp, closer := setup(apiOp)
	defer closer.Close()
	g.ResponseWriter.WriteList(apiOp, code, obj)
}

type gzipResponseWriter struct {
	io.Writer
	http.ResponseWriter
}

func (g gzipResponseWriter) Write(b []byte) (int, error) {
	return g.Writer.Write(b)
}
