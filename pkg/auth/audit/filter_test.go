package audit

import (
	"regexp"
	"testing"

	auditlogv1 "github.com/rancher/rancher/pkg/apis/auditlog.cattle.io/v1"
	"github.com/stretchr/testify/assert"
)

func TestFilterActionForURI(t *testing.T) {
	tests := []struct {
		name       string
		filter     Filter
		requestURI string
		expected   auditlogv1.FilterAction
	}{
		{
			name: "allow filter matches",
			filter: Filter{
				action: auditlogv1.FilterActionAllow,
				uri:    regexp.MustCompile(".*pods.*"),
			},
			requestURI: "/api/v1/namespaces/default/pods",
			expected:   auditlogv1.FilterActionAllow,
		},
		{
			name: "deny filter matches",
			filter: Filter{
				action: auditlogv1.FilterActionDeny,
				uri:    regexp.MustCompile(".*secrets.*"),
			},
			requestURI: "/api/v1/namespaces/default/secrets/my-secret",
			expected:   auditlogv1.FilterActionDeny,
		},
		{
			name: "allow filter does not match",
			filter: Filter{
				action: auditlogv1.FilterActionAllow,
				uri:    regexp.MustCompile(".*secrets.*"),
			},
			requestURI: "/api/v1/namespaces/default/pods",
			expected:   auditlogv1.FilterActionUnknown,
		},
		{
			name: "deny filter does not match",
			filter: Filter{
				action: auditlogv1.FilterActionDeny,
				uri:    regexp.MustCompile("/healthz"),
			},
			requestURI: "/api/v1/namespaces/default/pods",
			expected:   auditlogv1.FilterActionUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := tt.filter.ActionForURI(tt.requestURI)
			assert.Equal(t, tt.expected, actual)
		})
	}
}
