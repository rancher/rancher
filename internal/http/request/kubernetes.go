package request

import (
	"net/http"
	"strings"
)

// IsPodExecRequest returns true when the request path targets the pod exec subresource.
// This matches requests to execute commands in pods via WebSocket.
// Matches any path ending in: api/v1/namespaces/{namespace}/pods/{pod}/exec
func IsPodExecRequest(req *http.Request) bool {
	if req == nil || req.URL == nil {
		return false
	}

	segments := strings.Split(strings.Trim(req.URL.Path, "/"), "/")
	n := len(segments)

	if n < 7 {
		return false
	}

	return segments[n-7] == "api" &&
		segments[n-6] == "v1" &&
		segments[n-5] == "namespaces" &&
		segments[n-3] == "pods" &&
		segments[n-1] == "exec"
}
