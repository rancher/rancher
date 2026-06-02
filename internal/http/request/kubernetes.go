package request

import (
	"net/http"
	"strings"
)

// IsPodExecRequest returns true when the request path targets the pod exec subresource.
// This matches requests to execute commands in pods via WebSocket.
// Matches patterns like:
//   - /api/v1/namespaces/{namespace}/pods/{pod}/exec (local cluster)
//   - /k8s/clusters/{cluster}/api/v1/namespaces/{namespace}/pods/{pod}/exec (remote cluster without steve version)
//   - /k8s/clusters/{cluster}/v1/api/v1/namespaces/{namespace}/pods/{pod}/exec (remote cluster with steve version)
func IsPodExecRequest(req *http.Request) bool {
	if req == nil || req.URL == nil {
		return false
	}
	path := req.URL.Path

	// Trim leading/trailing slashes so a trailing "/" doesn't shift indices.
	segments := strings.Split(strings.Trim(path, "/"), "/")
	n := len(segments)

	// The exec subresource always ends in:
	//   namespaces/{ns}/pods/{pod}/exec
	// We can exclude any URLs with paths that have less segments.
	if n < 5 {
		return false
	}

	hasLocalPrefix := segments[0] == "api" && segments[1] == "v1"

	// Remote cluster paths can have an optional steve version (e.g., /v1/) after the cluster ID
	// Pattern 1: /k8s/clusters/{cluster}/api/v1/...
	// Pattern 2: /k8s/clusters/{cluster}/v1/api/v1/... (with steve version)
	hasRemotePrefix := false
	if n >= 6 && segments[0] == "k8s" && segments[1] == "clusters" {
		// Check for pattern without steve version: /k8s/clusters/{cluster}/api/v1/...
		if segments[3] == "api" && segments[4] == "v1" {
			hasRemotePrefix = true
		}
		// Check for pattern with steve version: /k8s/clusters/{cluster}/v1/api/v1/...
		if n >= 7 && segments[4] == "api" && segments[5] == "v1" {
			hasRemotePrefix = true
		}
	}

	hasExecSuffix := segments[n-5] == "namespaces" &&
		segments[n-3] == "pods" &&
		segments[n-1] == "exec"

	return hasExecSuffix && (hasLocalPrefix || hasRemotePrefix)
}
