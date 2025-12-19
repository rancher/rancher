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

func TestAllowList(t *testing.T) {
	logs, w := setup(t, WriterOptions{
		DefaultPolicyLevel:     auditlogv1.LevelRequestResponse,
		DisableDefaultPolicies: true,
	})

	err := w.UpdatePolicy(&auditlogv1.AuditPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "block-all",
			Namespace: "default",
		},
		Spec: auditlogv1.AuditPolicySpec{
			Filters: []auditlogv1.Filter{
				{
					Action:     auditlogv1.FilterActionDeny,
					RequestURI: ".*",
				},
			},
		},
	})
	assert.NoError(t, err)

	expected := []logEntry{}

	err = w.Write(&logEntry{
		RequestURI: "/api/v1/secrets",
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
					Action:     auditlogv1.FilterActionAllow,
					RequestURI: "/api/v1/secrets",
				},
			},
		},
	})
	assert.NoError(t, err)

	expected = []logEntry{
		{
			RequestURI: "/api/v1/secrets",
		},
	}

	err = w.Write(&logEntry{
		RequestURI: "/api/v1/secrets",
	})
	assert.NoError(t, err)
	assert.Equal(t, expected, logs.logs)
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
