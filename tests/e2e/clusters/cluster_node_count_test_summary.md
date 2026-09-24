# `cluster_node_count_test.go` Summary

Verifies that cluster node count is accurately maintained as management nodes are created and deleted.

## `TestClusterNodeCount`
Creates an import cluster, waits for its management namespace to be created on the local cluster, then adds and removes nodes via the management API while verifying the node count updates correctly.
- Checks the cluster node count starts at 0.
- Checks that the cluster's management namespace is created in the local cluster.
- Checks the node count increments to 1 after creating the first node.
- Checks the node count increments to 2 after creating a second node.
- Checks the node count decrements back to 1 after deleting a node.
