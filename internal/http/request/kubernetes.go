package request

import (
	"net/http"
	"strings"
)

// IsPodExecRequest returns true when the request path targets the pod exec subresource.
// This matches requests to execute commands in pods via WebSocket.
// Matches patterns like:
//   - /api/v1/namespaces/{namespace}/pods/{pod}/exec (local cluster)
//   - /k8s/clusters/{cluster}/v1/api/v1/namespaces/{namespace}/pods/{pod}/exec (remote cluster)
func IsPodExecRequest(req *http.Request) bool {
	if req == nil || req.URL == nil {
		return false
	}
	path := req.URL.Path
	return strings.Contains(path, "/namespaces/") &&
		strings.Contains(path, "/pods/") &&
		strings.HasSuffix(path, "/exec")
}
