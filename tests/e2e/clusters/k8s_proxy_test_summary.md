# `k8s_proxy_test.go` Summary

Verifies that the Kubernetes proxy correctly routes HTTP requests to both local and downstream clusters and returns appropriate responses.

## `TestK8sProxyFetchesNamespacesFromLocalCluster`
Makes an HTTP GET request to the proxy endpoint for the local cluster's namespaces API.
- Checks the proxy returns HTTP 200.
- Checks the response is a valid NamespaceList with an items field.

## `TestK8sProxyFetchesNamespacesFromDownstreamCluster`
Finds a ready downstream cluster, then makes an HTTP GET request to the proxy endpoint for that cluster's namespaces API.
- Checks the proxy eventually returns HTTP 200 (with retries to handle transient unavailability).
- Checks the response is a valid NamespaceList with an items field.

## `TestProxyK8sV1PathReturnsNotFound`
Makes an HTTP GET request to a malformed k8s proxy path for a downstream cluster.
- Checks the proxy returns HTTP 404 for the invalid path.
