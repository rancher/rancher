package audit

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"encoding/json"
	"net/http"
	"testing"

	auditlogv1 "github.com/rancher/rancher/pkg/apis/auditlog.cattle.io/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type logWriter struct {
	logs []logEntry
}

func (w *logWriter) Write(p []byte) (n int, err error) {
	var l logEntry
	if err := json.Unmarshal(p, &l); err != nil {
		return 0, err
	}

	w.logs = append(w.logs, l)

	return len(p), nil
}

func setup(t *testing.T, opts WriterOptions) (*logWriter, *Writer) {
	lw := &logWriter{
		logs: []logEntry{},
	}

	w, err := NewWriter(lw, opts)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}

	return lw, w
}

func TestDenyTakesPrecedenceAcrossPolicies(t *testing.T) {
	logs, w := setup(t, WriterOptions{
		DefaultPolicyLevel:     auditlogv1.LevelRequestResponse,
		DisableDefaultPolicies: true,
	})

	err := w.UpdatePolicy(&auditlogv1.AuditPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "deny-secrets"},
		Spec: auditlogv1.AuditPolicySpec{
			Filters: []auditlogv1.Filter{
				{Action: auditlogv1.FilterActionDeny, RequestURI: "/api/v1/secrets"},
			},
		},
	})
	require.NoError(t, err)

	err = w.UpdatePolicy(&auditlogv1.AuditPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "allow-secrets"},
		Spec: auditlogv1.AuditPolicySpec{
			Filters: []auditlogv1.Filter{
				{Action: auditlogv1.FilterActionAllow, RequestURI: "/api/v1/secrets"},
			},
		},
	})
	require.NoError(t, err)

	err = w.Write(&logEntry{RequestURI: "/api/v1/secrets"})
	require.NoError(t, err)
	assert.Empty(t, logs.logs)

	err = w.Write(&logEntry{RequestURI: "/api/v1/pods"})
	require.NoError(t, err)
	require.Len(t, logs.logs, 1)
	assert.Equal(t, "/api/v1/pods", logs.logs[0].RequestURI)
}

func TestPolicyActionForURI(t *testing.T) {
	tests := []struct {
		name     string
		filters  []auditlogv1.Filter
		uri      string
		expected auditlogv1.FilterAction
	}{
		{
			name:     "no filters allow all",
			uri:      "/api/v1/pods",
			expected: auditlogv1.FilterActionAllow,
		},
		{
			name: "allow filter matches",
			filters: []auditlogv1.Filter{
				{Action: auditlogv1.FilterActionAllow, RequestURI: ".*secrets.*"},
			},
			uri:      "/api/v1/secrets",
			expected: auditlogv1.FilterActionAllow,
		},
		{
			name: "allow-only policy does not match",
			filters: []auditlogv1.Filter{
				{Action: auditlogv1.FilterActionAllow, RequestURI: ".*secrets.*"},
			},
			uri:      "/api/v1/pods",
			expected: auditlogv1.FilterActionUnknown,
		},
		{
			name: "deny filter matches",
			filters: []auditlogv1.Filter{
				{Action: auditlogv1.FilterActionDeny, RequestURI: "/healthz"},
			},
			uri:      "/healthz",
			expected: auditlogv1.FilterActionDeny,
		},
		{
			name: "deny-only policy does not match",
			filters: []auditlogv1.Filter{
				{Action: auditlogv1.FilterActionDeny, RequestURI: "/healthz"},
			},
			uri:      "/api/v1/pods",
			expected: auditlogv1.FilterActionUnknown,
		},
		{
			name: "allow overrides deny in same policy - deny first",
			filters: []auditlogv1.Filter{
				{Action: auditlogv1.FilterActionDeny, RequestURI: ".*"},
				{Action: auditlogv1.FilterActionAllow, RequestURI: ".*login.*"},
			},
			uri:      "/v3-public/localProviders/local?action=login",
			expected: auditlogv1.FilterActionAllow,
		},
		{
			name: "allow overrides deny in same policy - allow first",
			filters: []auditlogv1.Filter{
				{Action: auditlogv1.FilterActionAllow, RequestURI: ".*login.*"},
				{Action: auditlogv1.FilterActionDeny, RequestURI: ".*"},
			},
			uri:      "/v3-public/localProviders/local?action=login",
			expected: auditlogv1.FilterActionAllow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy, err := PolicyFromAuditPolicy(&auditlogv1.AuditPolicy{
				Spec: auditlogv1.AuditPolicySpec{Filters: tt.filters},
			})
			require.NoError(t, err)
			assert.Equal(t, tt.expected, policy.actionForUri(tt.uri))
		})
	}
}

func TestAllowTakesPrecedenceWithinPolicy(t *testing.T) {
	filterOrders := []struct {
		name    string
		filters []auditlogv1.Filter
	}{
		{
			name: "deny before allow",
			filters: []auditlogv1.Filter{
				{Action: auditlogv1.FilterActionDeny, RequestURI: ".*"},
				{Action: auditlogv1.FilterActionAllow, RequestURI: ".*login.*"},
			},
		},
		{
			name: "allow before deny",
			filters: []auditlogv1.Filter{
				{Action: auditlogv1.FilterActionAllow, RequestURI: ".*login.*"},
				{Action: auditlogv1.FilterActionDeny, RequestURI: ".*"},
			},
		},
	}

	for _, tt := range filterOrders {
		t.Run(tt.name, func(t *testing.T) {
			logs, w := setup(t, WriterOptions{
				DefaultPolicyLevel:     auditlogv1.LevelRequestResponse,
				DisableDefaultPolicies: true,
			})

			err := w.UpdatePolicy(&auditlogv1.AuditPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "login-only"},
				Spec:       auditlogv1.AuditPolicySpec{Filters: tt.filters},
			})
			require.NoError(t, err)

			err = w.Write(&logEntry{RequestURI: "/v3-public/localProviders/local?action=login"})
			require.NoError(t, err)
			require.Len(t, logs.logs, 1)

			err = w.Write(&logEntry{RequestURI: "/api/v1/pods"})
			require.NoError(t, err)
			require.Len(t, logs.logs, 1)
		})
	}
}

func TestUserDenyOverridesDefaultPolicies(t *testing.T) {
	logs, w := setup(t, WriterOptions{
		DefaultPolicyLevel:     auditlogv1.LevelRequestResponse,
		DisableDefaultPolicies: false,
	})

	err := w.UpdatePolicy(&auditlogv1.AuditPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "deny-healthz"},
		Spec: auditlogv1.AuditPolicySpec{
			Filters: []auditlogv1.Filter{
				{Action: auditlogv1.FilterActionDeny, RequestURI: "/healthz"},
			},
		},
	})
	require.NoError(t, err)

	err = w.Write(&logEntry{RequestURI: "/healthz"})
	require.NoError(t, err)
	assert.Empty(t, logs.logs)

	err = w.Write(&logEntry{RequestURI: "/api/v1/pods"})
	require.NoError(t, err)

	// Also exercise Rancher's allow-only default secrets policy.
	err = w.Write(&logEntry{RequestURI: "/api/v1/secrets"})
	require.NoError(t, err)

	require.Len(t, logs.logs, 2)
	assert.Equal(t, "/api/v1/pods", logs.logs[0].RequestURI)
	assert.Equal(t, "/api/v1/secrets", logs.logs[1].RequestURI)
}

func TestBlockList(t *testing.T) {
	logs, w := setup(t, WriterOptions{
		DefaultPolicyLevel:     auditlogv1.LevelRequestResponse,
		DisableDefaultPolicies: true,
	})

	expected := []logEntry{
		{
			RequestURI: "/api/v1/secrets",
			Method:     http.MethodGet,
		},
	}

	err := w.Write(&logEntry{
		RequestURI: "/api/v1/secrets",
		Method:     http.MethodGet,
	})
	assert.NoError(t, err)
	assert.Equal(t, expected, logs.logs)

	err = w.UpdatePolicy(&auditlogv1.AuditPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "allow-secrets",
			Namespace: "default",
		},
		Spec: auditlogv1.AuditPolicySpec{
			Filters: []auditlogv1.Filter{
				{
					Action:     auditlogv1.FilterActionDeny,
					RequestURI: "/api/v1/secrets",
				},
			},
		},
	})
	assert.NoError(t, err)

	err = w.Write(&logEntry{
		RequestURI: "/api/v1/secrets",
		Method:     http.MethodGet,
	})
	assert.NoError(t, err)
	assert.Equal(t, expected, logs.logs)

	expected = append(expected, logEntry{
		RequestURI: "/api/v1/configmaps",
		Method:     http.MethodGet,
	})

	err = w.Write(&logEntry{
		RequestURI: "/api/v1/configmaps",
		Method:     http.MethodGet,
	})
	assert.NoError(t, err)
	assert.Equal(t, expected, logs.logs)
}

func TestUnmatchedDenyPolicyDoesNotApplyRedactions(t *testing.T) {
	logs, w := setup(t, WriterOptions{
		DefaultPolicyLevel:     auditlogv1.LevelRequestResponse,
		DisableDefaultPolicies: true,
	})

	err := w.UpdatePolicy(&auditlogv1.AuditPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "deny-healthz-redact-cache"},
		Spec: auditlogv1.AuditPolicySpec{
			Filters: []auditlogv1.Filter{
				{Action: auditlogv1.FilterActionDeny, RequestURI: "/healthz"},
			},
			AdditionalRedactions: []auditlogv1.Redaction{
				{Headers: []string{"Cache.*"}},
			},
		},
	})
	require.NoError(t, err)

	err = w.Write(&logEntry{
		RequestURI: "/api/v1/pods",
		ResponseHeader: http.Header{
			"Cache-Control": []string{"no-cache"},
			"Content-Type":  []string{contentTypeJSON},
		},
	})
	require.NoError(t, err)
	require.Len(t, logs.logs, 1)
	assert.Equal(t, []string{"no-cache"}, logs.logs[0].ResponseHeader["Cache-Control"])

	err = w.Write(&logEntry{RequestURI: "/healthz"})
	require.NoError(t, err)
	require.Len(t, logs.logs, 1)
}

func TestNewWriterDropsImpersonateGroups(t *testing.T) {
	tests := []struct {
		name           string
		level          auditlogv1.Level
		excludeGroups  bool
		expectPolicy   bool
		expectedHeader []string
	}{
		{
			name:           "no-groups-level",
			level:          auditlogv1.LevelRequestResponse,
			excludeGroups:  true,
			expectPolicy:   true,
			expectedHeader: []string{redacted},
		},
		{
			name:           "request-response-level",
			level:          auditlogv1.LevelRequestResponse,
			expectedHeader: []string{"keycloakoidc_group://testers", "keycloakoidc_group://developers"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lw := &logWriter{logs: []logEntry{}}

			w, err := NewWriter(lw, WriterOptions{
				DefaultPolicyLevel:     tt.level,
				DisableDefaultPolicies: true,
				ExcludeGroups:          tt.excludeGroups,
			})
			assert.NoError(t, err)

			_, ok := w.GetPolicy("drop-impersonation-groups")
			assert.Equal(t, tt.expectPolicy, ok)

			err = w.Write(&logEntry{
				RequestURI: "/api/v1/secrets",
				RequestHeader: http.Header{
					"Impersonate-Group": []string{"keycloakoidc_group://testers", "keycloakoidc_group://developers"},
				},
			})
			assert.NoError(t, err)

			expected := []logEntry{
				{
					RequestURI: "/api/v1/secrets",
					RequestHeader: http.Header{
						"Impersonate-Group": tt.expectedHeader,
					},
				},
			}

			assert.Equal(t, expected, lw.logs)
		})
	}
}

func TestHigherVerbosityForPolicy(t *testing.T) {
	bodyContent := []byte(`{"password":"password"}`)
	headers := map[string][]string{
		"foo":          {"bar"},
		"baz":          {"qux"},
		"Content-Type": {contentTypeJSON},
	}
	logs, w := setup(t, WriterOptions{
		DefaultPolicyLevel:     auditlogv1.LevelNull,
		DisableDefaultPolicies: true,
	})

	w.UpdatePolicy(&auditlogv1.AuditPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "set-higher-default-verbosity",
		},
		Spec: auditlogv1.AuditPolicySpec{
			Verbosity: auditlogv1.LogVerbosity{
				Level: auditlogv1.LevelRequest,
			},
		},
	})

	w.UpdatePolicy(&auditlogv1.AuditPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "set-higher-custom-default-verbosity-for-specific-url",
		},
		Spec: auditlogv1.AuditPolicySpec{
			Filters: []auditlogv1.Filter{
				{
					Action:     auditlogv1.FilterActionAllow,
					RequestURI: "/my/endopint",
				},
			},
			Verbosity: auditlogv1.LogVerbosity{
				Level: auditlogv1.LevelRequestResponse,
			},
		},
	})

	entry1 := &logEntry{
		RequestURI:     "/some/endopint",
		RequestHeader:  headers,
		ResponseHeader: headers,
	}
	// This entry should get LevelRequest verbosity (request body only)
	prepareLogEntry(entry1, &testLogData{
		verbosity:  verbosityForLevel(auditlogv1.LevelRequest),
		resHeaders: headers,
		rawResBody: bodyContent,
		reqHeaders: headers,
		rawReqBody: bodyContent,
	})
	err := w.Write(entry1)
	assert.NoError(t, err)

	entry2 := &logEntry{
		RequestURI:     "/my/endopint",
		RequestHeader:  headers,
		ResponseHeader: headers,
	}
	// This entry should get LevelRequestResponse verbosity (both bodies)
	prepareLogEntry(entry2, &testLogData{
		verbosity:  verbosityForLevel(auditlogv1.LevelRequestResponse),
		resHeaders: headers,
		rawResBody: bodyContent,
		reqHeaders: headers,
		rawReqBody: bodyContent,
	})
	err = w.Write(entry2)
	assert.NoError(t, err)

	expected := []logEntry{
		{
			RequestURI:     "/some/endopint",
			RequestHeader:  headers,
			ResponseHeader: headers,
			RequestBody: map[string]any{
				"password": "password",
			},
		},
		{
			RequestURI:     "/my/endopint",
			RequestHeader:  headers,
			ResponseHeader: headers,
			RequestBody: map[string]any{
				"password": "password",
			},
			ResponseBody: map[string]any{
				"password": "password",
			},
		},
	}

	assert.Equal(t, expected, logs.logs)
}

func TestCompressedGzip(t *testing.T) {
	logs, w := setup(t, WriterOptions{
		DefaultPolicyLevel:     auditlogv1.LevelRequestResponse,
		DisableDefaultPolicies: true,
	})

	buffer := bytes.NewBuffer(nil)
	compressor := gzip.NewWriter(buffer)
	compressor.Write([]byte(`{"foo":"bar"}`))
	compressor.Close()

	body := buffer.Bytes()

	entry := &logEntry{}
	// Prepare the response body similar to a production path
	prepareLogEntry(entry, &testLogData{
		verbosity: verbosityForLevel(auditlogv1.LevelRequestResponse),
		resHeaders: http.Header{
			"Content-Encoding": []string{contentEncodingGZIP},
			"Content-Type":     []string{contentTypeJSON},
		},
		rawResBody: body,
	})

	err := w.Write(entry)
	assert.NoError(t, err)

	expected := []logEntry{
		{
			ResponseHeader: http.Header{
				"Content-Encoding": []string{contentEncodingGZIP},
				"Content-Type":     []string{contentTypeJSON},
			},
			ResponseBody: map[string]any{
				"foo": "bar",
			},
		},
	}

	assert.Equal(t, expected, logs.logs)
}

func TestCompressedZLib(t *testing.T) {
	logs, w := setup(t, WriterOptions{
		DefaultPolicyLevel:     auditlogv1.LevelRequestResponse,
		DisableDefaultPolicies: true,
	})

	buffer := bytes.NewBuffer(nil)
	compressor := zlib.NewWriter(buffer)
	compressor.Write([]byte(`{"foo":"bar"}`))
	compressor.Close()

	body := buffer.Bytes()

	entry := &logEntry{}
	// Prepare the response body similar to production path
	prepareLogEntry(entry, &testLogData{
		verbosity: verbosityForLevel(auditlogv1.LevelRequestResponse),
		resHeaders: http.Header{
			"Content-Encoding": []string{contentEncodingZLib},
			"Content-Type":     []string{contentTypeJSON},
		},
		rawResBody: body,
	})

	err := w.Write(entry)
	assert.NoError(t, err)

	expected := []logEntry{
		{
			ResponseHeader: http.Header{
				"Content-Encoding": []string{contentEncodingZLib},
				"Content-Type":     []string{contentTypeJSON},
			},
			ResponseBody: map[string]any{
				"foo": "bar",
			},
		},
	}

	assert.Equal(t, expected, logs.logs)
}
