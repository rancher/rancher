package audit

import (
	"encoding/json"
	"net/http"
	"testing"

	auditlogv1 "github.com/rancher/rancher/pkg/apis/auditlog.cattle.io/v1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type logWriter struct {
	logs []log
}

func (w *logWriter) Write(p []byte) (n int, err error) {
	l := log{}
	if err := json.Unmarshal(p, &l); err != nil {
		return 0, err
	}

	w.logs = append(w.logs, l)

	return len(p), nil
}

func setup(t *testing.T, opts WriterOptions) (*logWriter, *Writer) {
	lw := &logWriter{
		logs: []log{},
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

	err := w.UpdatePolicy(&auditlogv1.AuditLogPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "block-all",
			Namespace: "default",
		},
		Spec: auditlogv1.AuditLogPolicySpec{
			Filters: []auditlogv1.Filter{
				{
					Action:     auditlogv1.FilterActionDeny,
					RequestURI: ".*",
				},
			},
		},
	})
	assert.NoError(t, err)

	expected := []log{
		// {
		// 	RequestURI: "/api/v1/secrets",
		// 	Method:     http.MethodGet,
		// },
	}

	err = w.Write(&log{
		RequestURI: "/api/v1/secrets",
		Method:     http.MethodGet,
	})
	assert.NoError(t, err)
	assert.Equal(t, expected, logs.logs)

	err = w.UpdatePolicy(&auditlogv1.AuditLogPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "allow-secrets",
			Namespace: "default",
		},
		Spec: auditlogv1.AuditLogPolicySpec{
			Filters: []auditlogv1.Filter{
				{
					Action:     auditlogv1.FilterActionAllow,
					RequestURI: "/api/v1/secrets",
				},
			},
		},
	})
	assert.NoError(t, err)

	expected = []log{
		{
			RequestURI: "/api/v1/secrets",
			Method:     http.MethodGet,
		},
	}

	err = w.Write(&log{
		RequestURI: "/api/v1/secrets",
		Method:     http.MethodGet,
	})
	assert.NoError(t, err)
	assert.Equal(t, expected, logs.logs)
}

func TestBlockList(t *testing.T) {
	logs, w := setup(t, WriterOptions{
		DefaultPolicyLevel:     auditlogv1.LevelRequestResponse,
		DisableDefaultPolicies: true,
	})

	expected := []log{
		{
			RequestURI: "/api/v1/secrets",
			Method:     http.MethodGet,
		},
	}

	err := w.Write(&log{
		RequestURI: "/api/v1/secrets",
		Method:     http.MethodGet,
	})
	assert.NoError(t, err)
	assert.Equal(t, expected, logs.logs)

	err = w.UpdatePolicy(&auditlogv1.AuditLogPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "allow-secrets",
			Namespace: "default",
		},
		Spec: auditlogv1.AuditLogPolicySpec{
			Filters: []auditlogv1.Filter{
				{
					Action:     auditlogv1.FilterActionDeny,
					RequestURI: "/api/v1/secrets",
				},
			},
		},
	})
	assert.NoError(t, err)

	err = w.Write(&log{
		RequestURI: "/api/v1/secrets",
		Method:     http.MethodGet,
	})
	assert.NoError(t, err)
	assert.Equal(t, expected, logs.logs)
}
