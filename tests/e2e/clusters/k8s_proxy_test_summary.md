# `k8s_proxy_test.go` Summary

Verifies that the Kubernetes proxy correctly routes HTTP requests to both the local and downstream clusters and rejects malformed proxy paths.

## `TestK8sProxyFetchesNamespacesFromLocalCluster`
**Act:** Sends an HTTP GET request to the k8s proxy endpoint for the local cluster's namespaces API.

**Assert:**
- Checks the response status is HTTP 200.
- Checks the response body is a `NamespaceList` with an `items` field.

## `TestK8sProxyFetchesNamespacesFromDownstreamCluster`
**Arrange:**
- Finds an active downstream cluster with a `Ready` condition.

**Act:** Sends an HTTP GET request to the k8s proxy endpoint for the downstream cluster's namespaces API (retried to tolerate transient proxy unavailability).

**Assert:**
- Checks the response eventually returns HTTP 200.
- Checks the response body is a `NamespaceList` with an `items` field.

## `TestProxyK8sV1PathReturnsNotFound`
**Arrange:**
- Finds an active downstream cluster with a `Ready` condition.

**Act:** Sends an HTTP GET request to a malformed k8s proxy path (`/v1`) for the downstream cluster.

**Assert:**
- Checks the response status is HTTP 404.
