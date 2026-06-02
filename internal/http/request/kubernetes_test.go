package request

import (
	"net/http"
	"net/url"
	"testing"
)

func TestIsPodExecRequest(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		// Valid pod exec requests
		{
			name:     "local cluster pod exec",
			path:     "/api/v1/namespaces/default/pods/nginx/exec",
			expected: true,
		},
		{
			name:     "local cluster pod exec with query params",
			path:     "/api/v1/namespaces/kube-system/pods/coredns-123/exec",
			expected: true,
		},
		{
			name:     "remote cluster pod exec",
			path:     "/k8s/clusters/c-m-abc123/api/v1/namespaces/default/pods/nginx/exec",
			expected: true,
		},
		{
			name:     "remote cluster pod exec with complex namespace",
			path:     "/k8s/clusters/c-m-abc123/api/v1/namespaces/cattle-system/pods/rancher-xyz/exec",
			expected: true,
		},

		{
			name:     "remote cluster pod exec with steve version",
			path:     "/k8s/clusters/c-m-abc123/v1/api/v1/namespaces/default/pods/nginx/exec",
			expected: true,
		},
		{
			name:     "remote cluster pod exec with steve version and complex namespace",
			path:     "/k8s/clusters/c-m-abc123/v1/api/v1/namespaces/cattle-system/pods/rancher-xyz/exec",
			expected: true,
		},

		// Invalid requests - different subresources
		{
			name:     "pod logs request",
			path:     "/api/v1/namespaces/default/pods/nginx/log",
			expected: false,
		},
		{
			name:     "pod attach request",
			path:     "/api/v1/namespaces/default/pods/nginx/attach",
			expected: false,
		},
		{
			name:     "pod portforward request",
			path:     "/api/v1/namespaces/default/pods/nginx/portforward",
			expected: false,
		},
		{
			name:     "get pod request",
			path:     "/api/v1/namespaces/default/pods/nginx",
			expected: false,
		},
		{
			name:     "list pods request",
			path:     "/api/v1/namespaces/default/pods",
			expected: false,
		},

		// Edge cases that expose over-zealous matching
		{
			name:     "wrong order - pods before namespaces",
			path:     "/api/v1/pods/nginx/namespaces/default/exec",
			expected: false, // Should be false - segment order does not match the pod exec pattern
		},
		{
			name:     "exec in middle of path, not at end",
			path:     "/api/v1/namespaces/default/exec/pods/nginx/status",
			expected: false, // Should be false since exec is not the suffix
		},
		{
			name:     "exec as part of pod name",
			path:     "/api/v1/namespaces/default/pods/my-executor-pod",
			expected: false, // Should be false - exec is in pod name, not subresource
		},
		{
			name:     "exec as part of namespace name",
			path:     "/api/v1/namespaces/exec-namespace/pods/nginx/logs",
			expected: false, // Should be false - exec in namespace name
		},
		{
			name:     "namespaces and pods in query string or fragment",
			path:     "/some/other/path/exec?namespaces=foo&pods=bar",
			expected: false, // Should be false - keywords in query params
		},
		{
			name:     "deployment exec (not a real k8s API)",
			path:     "/api/v1/namespaces/default/deployments/nginx/exec",
			expected: false, // Should be false - not a pod resource
		},
		{
			name:     "exec endpoint but missing namespaces",
			path:     "/api/v1/pods/nginx/exec",
			expected: false,
		},
		{
			name:     "exec endpoint but missing pods",
			path:     "/api/v1/namespaces/default/exec",
			expected: false,
		},

		// Malicious/crafted paths that might bypass checks
		{
			name:     "path with embedded keywords in unrelated context",
			path:     "/custom/namespaces/api/something/pods/data/exec",
			expected: false, // Not a valid k8s API path
		},
		{
			name:     "double slashes",
			path:     "/api/v1/namespaces//default//pods//nginx//exec",
			expected: false, // If URL is parsed with `path.Clean` it would match, but as is now it will not match (may need to verify)
		},

		// Empty and nil cases
		{
			name:     "empty path",
			path:     "",
			expected: false,
		},
		{
			name:     "root path",
			path:     "/",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &http.Request{
				URL: &url.URL{
					Path: tt.path,
				},
			}
			result := IsPodExecRequest(req)
			if result != tt.expected {
				t.Errorf("IsPodExecRequest() = %v, expected %v for path: %s", result, tt.expected, tt.path)
			}
		})
	}
}

func TestIsPodExecRequest_NilSafety(t *testing.T) {
	tests := []struct {
		name     string
		req      *http.Request
		expected bool
	}{
		{
			name:     "nil request",
			req:      nil,
			expected: false,
		},
		{
			name: "nil URL",
			req: &http.Request{
				URL: nil,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsPodExecRequest(tt.req)
			if result != tt.expected {
				t.Errorf("IsPodExecRequest() = %v, expected %v", result, tt.expected)
			}
		})
	}
}
