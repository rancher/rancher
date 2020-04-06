from .common import random_str
from .conftest import wait_for
from kubernetes.client import CustomObjectsApi


def test_cluster_node_count(admin_mc, remove_resource,
                            raw_remove_custom_resource):
    """Test that the cluster node count gets updated as nodes are added"""
    client = admin_mc.client
    cluster = client.create_cluster(
        name=random_str(),
        rancherKubernetesEngineConfig={
            "accessKey": "junk"
        }
    )
    remove_resource(cluster)

    def _check_node_count(cluster, nodes):
        c = client.reload(cluster)
        return c.nodeCount == nodes

    def _node_count_fail(cluster, nodes):
        c = client.reload(cluster)
        s = "cluster {} failed to have proper node count, expected: {} has: {}"
        return s.format(c.id, nodes, c.nodeCount)

    node_count = 0
    wait_for(lambda: _check_node_count(cluster, node_count),
             fail_handler=lambda: _node_count_fail(cluster, node_count))

    # Nodes have to be created manually through k8s client to attach to a
    # pending cluster
    k8s_dynamic_client = CustomObjectsApi(admin_mc.k8s_client)
    body = {
        "metadata": {
            "name": random_str(),
            "namespace": cluster.id,
        },
        "kind": "Node",
        "apiVersion": "management.cattle.io/v3",
    }

    dynamic_nt = k8s_dynamic_client.create_namespaced_custom_object(
        "management.cattle.io", "v3", cluster.id, 'nodes', body)
    raw_remove_custom_resource(dynamic_nt)

    node_count = 1
    wait_for(lambda: _check_node_count(cluster, node_count),
             fail_handler=lambda: _node_count_fail(cluster, node_count))

    # Create node number 2
    body['metadata']['name'] = random_str()

    dynamic_nt1 = k8s_dynamic_client.create_namespaced_custom_object(
        "management.cattle.io", "v3", cluster.id, 'nodes', body)
    raw_remove_custom_resource(dynamic_nt1)

    node_count = 2
    wait_for(lambda: _check_node_count(cluster, node_count),
             fail_handler=lambda: _node_count_fail(cluster, node_count))

    # Delete a node
    k8s_dynamic_client.delete_namespaced_custom_object(
        "management.cattle.io", "v3", cluster.id, 'nodes',
        dynamic_nt1['metadata']['name'], {})

    node_count = 1
    wait_for(lambda: _check_node_count(cluster, node_count),
             fail_handler=lambda: _node_count_fail(cluster, node_count))
