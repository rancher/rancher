package urlbuilder

import (
	"bytes"
	"net/http"
	"net/url"
	"strings"
)

func RedirectRewrite(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		prefix := req.Header.Get(PrefixHeader)
		if prefix == "" {
			next.ServeHTTP(rw, req)
			return
		}
		r := &redirector{
			ResponseWriter: rw,
			prefix:         prefix,
		}
		if h, ok := rw.(http.Hijacker); ok {
			r.Hijacker = h
		}
		next.ServeHTTP(r, req)
		r.Close()
	})
}

type redirector struct {
	http.ResponseWriter
	http.Hijacker
	prefix     string
	from, to   string
	tempBuffer *bytes.Buffer
}

func (r *redirector) Write(content []byte) (int, error) {
	if r.tempBuffer == nil {
		return r.ResponseWriter.Write(content)
	}
	return r.tempBuffer.Write(content)
}

func (r *redirector) Close() error {
	if r.tempBuffer == nil || r.from == "" || r.to == "" {
		return nil
	}

	content := bytes.Replace(r.tempBuffer.Bytes(), []byte(r.from), []byte(r.to), -1)
	_, err := r.ResponseWriter.Write(content)
	r.tempBuffer = nil
	return err
}

func (r *redirector) WriteHeader(statusCode int) {
	defer func() {
		// the anonymous func is so that we take the new value of statusCode,
		// not copy it at invocation
		r.ResponseWriter.WriteHeader(statusCode)
	}()

	if statusCode != http.StatusMovedPermanently && statusCode != http.StatusFound {
		return
	}

	l := r.Header().Get("Location")
	if l == "" {
		return
	}

	u, _ := url.Parse(l)
	if !strings.HasPrefix(u.Path, r.prefix) {
		r.from = u.Path
		u.Path = r.prefix + u.Path
		r.Header().Set("Location", u.String())
		r.to = u.Path
		r.tempBuffer = &bytes.Buffer{}
	}

	statusCode = http.StatusFound
}
