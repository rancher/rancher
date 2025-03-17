package audit

import (
	"bytes"
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"
)

var _ http.ResponseWriter = &response{}

type response struct {
	header http.Header
	body   *bytes.Buffer
	status int
}

func (r *response) Header() http.Header {
	return r.header
}

func (r *response) Write(b []byte) (int, error) {
	return r.body.Write(b)
}

func (r *response) WriteHeader(statusCode int) {
	r.status = statusCode
}

func getRequest() *http.Request {
	r := &http.Request{
		Method: http.MethodGet,
		Header: http.Header{},
		URL: &url.URL{
			Scheme: "http",
			Host:   "localhost",
		},
	}

	var u user.Info = &user.DefaultInfo{
		Name:   "bilbo",
		UID:    "123",
		Groups: []string{"fellowship"},
		Extra:  map[string][]string{},
	}

	ctx := request.WithUser(context.Background(), u)
	r = r.WithContext(ctx)

	return r
}

func TestMisddleware(t *testing.T) {
	requests := []http.Request{}
	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		r := *req
		r.WithContext(context.Background())
		requests = append(requests, r)
	})
	out := &strings.Builder{}
	writer, err := NewWriter(out, WriterOptions{})
	require.NoError(t, err)

	m := NewAuditLogMiddleware(writer)
	handler := m(next)

	req := getRequest()
	handler.ServeHTTP(&response{
		header: http.Header{},
		body:   bytes.NewBuffer(nil),
	}, req)

	req.WithContext(context.Background())

	assert.Len(t, requests, 1)
}
