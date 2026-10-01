# `cluster_node_count_test.go` Summary

Verifies that a cluster's node count accurately reflects management nodes as they are created and deleted.

## `TestClusterNodeCount`
**Arrange:**
- Creates a cluster, whose node count starts at 0.
- Waits for the cluster's management namespace to be created on the local cluster.

**Act 1:** Creates a node in the cluster's namespace via the management API.
**Assert 1:**
- Checks the node count increments to 1.

**Act 2:** Creates a second node in the cluster's namespace.
**Assert 2:**
- Checks the node count increments to 2.

**Act 3:** Deletes the second node.
**Assert 3:**
- Checks the node count drops back to 1.
