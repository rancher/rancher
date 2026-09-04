package audit

import (
	"bytes"
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	auditlogv1 "github.com/rancher/rancher/pkg/apis/auditlog.cattle.io/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func TestMiddleware(t *testing.T) {
	requests := []http.Request{}
	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		requests = append(requests, *req)
	})
	out := &strings.Builder{}
	writerOpts := WriterOptions{}
	writer, err := NewWriter(out, writerOpts)
	require.NoError(t, err)

	m := NewAuditLogMiddleware(writer)
	handler := m(next)

	req := getRequest()
	handler.ServeHTTP(&response{
		header: http.Header{},
		body:   bytes.NewBuffer(nil),
	}, req)

	assert.Len(t, requests, 1, "handler did not forward request to next handler as expected")
}

func TestResolveVerbosityUsesOnlyAllowingPolicies(t *testing.T) {
	tests := []struct {
		name     string
		filters  []auditlogv1.Filter
		level    auditlogv1.Level
		uri      string
		expected auditlogv1.Level
	}{
		{
			name:     "filterless policy applies",
			level:    auditlogv1.LevelRequest,
			uri:      "/api/v1/pods",
			expected: auditlogv1.LevelRequest,
		},
		{
			name: "matching allow policy applies",
			filters: []auditlogv1.Filter{
				{Action: auditlogv1.FilterActionAllow, RequestURI: "/detailed"},
			},
			level:    auditlogv1.LevelRequestResponse,
			uri:      "/detailed",
			expected: auditlogv1.LevelRequestResponse,
		},
		{
			name: "unmatched allow policy is neutral",
			filters: []auditlogv1.Filter{
				{Action: auditlogv1.FilterActionAllow, RequestURI: "/detailed"},
			},
			level:    auditlogv1.LevelRequestResponse,
			uri:      "/api/v1/pods",
			expected: auditlogv1.LevelNull,
		},
		{
			name: "matched deny policy does not increase verbosity",
			filters: []auditlogv1.Filter{
				{Action: auditlogv1.FilterActionDeny, RequestURI: "/healthz"},
			},
			level:    auditlogv1.LevelRequestResponse,
			uri:      "/healthz",
			expected: auditlogv1.LevelNull,
		},
		{
			name: "unmatched deny policy is neutral",
			filters: []auditlogv1.Filter{
				{Action: auditlogv1.FilterActionDeny, RequestURI: "/healthz"},
			},
			level:    auditlogv1.LevelRequestResponse,
			uri:      "/api/v1/pods",
			expected: auditlogv1.LevelNull,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, w := setup(t, WriterOptions{
				DefaultPolicyLevel:     auditlogv1.LevelNull,
				DisableDefaultPolicies: true,
			})

			err := w.UpdatePolicy(&auditlogv1.AuditPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test-policy"},
				Spec: auditlogv1.AuditPolicySpec{
					Filters:   tt.filters,
					Verbosity: auditlogv1.LogVerbosity{Level: tt.level},
				},
			})
			require.NoError(t, err)

			handler := &LoggingHandler{writer: w}
			actual := handler.ResolveVerbosity(tt.uri)

			// ResolveVerbosity merges verbosities via mergeLogVerbosities, which does not propagate the Level
			// field (only the Request/Response flags it actually computes from). Normalize Level before
			// comparing so this test asserts on the flags that matter to callers.
			expected := verbosityForLevel(tt.expected)
			expected.Level = auditlogv1.LevelNull
			assert.Equal(t, expected, actual)
		})
	}
}
