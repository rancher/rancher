package clusterrouter

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetClusterID(t *testing.T) {
	tests := map[string]struct {
		path string
		want string
	}{
		"k8s clusters path returns cluster ID": {
			path: "/k8s/clusters/c-abc123",
			want: "c-abc123",
		},
		"k8s cluster path returns cluster ID": {
			path: "/k8s/cluster/c-abc123",
			want: "c-abc123",
		},
		"k8s proxy path returns cluster ID": {
			path: "/k8s/proxy/c-abc123",
			want: "c-abc123",
		},
		"v3 clusters path returns cluster ID": {
			path: "/v3/clusters/c-abc123",
			want: "c-abc123",
		},
		"v3 cluster path returns cluster ID": {
			path: "/v3/cluster/c-abc123",
			want: "c-abc123",
		},
		"v3 proxy path returns cluster ID": {
			path: "/v3/proxy/c-abc123",
			want: "c-abc123",
		},
		"path with additional segments returns cluster ID": {
			path: "/k8s/clusters/c-abc123/api/v1/pods",
			want: "c-abc123",
		},
		"unknown element returns empty string": {
			path: "/k8s/nodes/c-abc123",
			want: "",
		},
		"path too short returns empty string": {
			path: "/k8s/clusters",
			want: "",
		},
		"empty path returns empty string": {
			path: "",
			want: "",
		},
		"root path returns empty string": {
			path: "/",
			want: "",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, "http://example.com"+tc.path, nil)
			if tc.path == "" {
				req, err = http.NewRequest(http.MethodGet, "http://example.com", nil)
			}
			assert.NoError(t, err)

			got := GetClusterID(req)
			assert.Equal(t, tc.want, got)
		})
	}
}
