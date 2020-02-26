import pytest
from .common import *  # NOQA
from .test_rke_cluster_provisioning import create_and_validate_cluster
from .test_rke_cluster_provisioning import node_template_ec2
from .test_rke_cluster_provisioning import node_template_ec2_with_provider
from .test_rke_cluster_provisioning import random_node_name
from .test_rke_cluster_provisioning import rke_config_aws_provider
from .test_rke_cluster_provisioning import rke_config

namespace = {'client': None, 'cluster': None}

def test_self_healing_nodepool_ec2_provider_nodelete(create_cluster_ec2_with_provider):
    client = namespace['client']
    pool_nodelete = get_nodepool_without_delete()
    assert len(pool_nodelete) == 1
    nodes = client.list_node(nodePoolId=pool_nodelete[0].id).data
    terminate_node_and_check_ec2(nodes, pool_nodelete[0].id)

def test_self_healing_nodepool_ec2_provider_delete(create_cluster_ec2_with_provider):
    client = namespace['client']
    pool_withdelete = get_nodepool_with_delete()
    assert len(pool_withdelete) == 1
    nodes = client.list_node(nodePoolId=pool_withdelete[0].id).data
    stop_node_and_check_ec2(nodes, pool_withdelete[0].id)

def test_self_healing_nodepool_ec2_noprovider_delete(create_cluster_do_without_provider):
    client = namespace['client']
    pool_withdelete = get_nodepool_with_delete()
    assert len(pool_withdelete) == 1
    nodes = client.list_node(nodePoolId=pool_withdelete[0].id).data
    stop_node_and_check_ec2(nodes, pool_withdelete[0].id)

def test_self_healing_nodepool_ec2_noprovider_nodelete(create_cluster_do_without_provider):
    client = namespace['client']
    pool_nodelete = get_nodepool_without_delete()
    assert len(pool_nodelete) == 1
    nodes = client.list_node(nodePoolId=pool_nodelete[0].id).data
    stop_node_and_check_no_heal_ec2(nodes, pool_nodelete[0].id)

def get_nodepool_with_delete():
    cluster = namespace['cluster']
    worker_node_pools = cluster.nodePools(worker="true")
    # get the worker nodepool with deleteNotReadyAfterSecs is set to 10 seconds
    return [data for data in worker_node_pools.data
            if data["deleteNotReadyAfterSecs"] == 10]

def get_nodepool_without_delete():
    cluster = namespace['cluster']
    worker_node_pools = cluster.nodePools(worker="true")
    # get the worker nodepool with deleteNotReadyAfterSecs is set to 0 seconds
    return [data for data in worker_node_pools.data
            if data["deleteNotReadyAfterSecs"] == 0]

def get_nodes_to_change_status(nodes):
    client = namespace['client']
    cluster = namespace['cluster']

    # Filter the nodes and get the only the worker node
    worker_node = [node for node in nodes if node.worker == True][0]
    # Get the worker node Ip address
    node_ip = worker_node.externalIpAddress

    filters = [
        {'Name': 'ip-address',
         'Values': [node_ip]}
    ]
    # filter the cloud provider nodes by the ip address
    return AmazonWebServices().get_nodes(filters), worker_node

def terminate_node_and_check_ec2(nodes, pool_id):
    client = namespace['client']
    cluster = namespace['cluster']

    aws_nodes, worker_node = get_nodes_to_change_status(nodes)
    original_worker_node_id = worker_node.id

    # stop the node in the cloud
    AmazonWebServices().delete_node(aws_nodes[0], True)
    # Check node is deleted from Rancher
    wait_for_node_to_be_deleted(client, worker_node)

    nodes = wait_for_nodepool_to_have_nodes(client, cluster, pool_id)

    # Filter the nodes and get the only the worker node
    worker_node = [node for node in nodes if node.worker == True][0]

    # wait for this node to become active
    worker_node = wait_for_node_status(client, worker_node, "active")

    # make sure the new worker node is entirely new
    assert original_worker_node_id != worker_node.id

def stop_node_and_check_ec2(nodes, pool_id):
    client = namespace['client']
    cluster = namespace['cluster']

    aws_nodes, worker_node = get_nodes_to_change_status(nodes)
    original_worker_node_id = worker_node.id

    # stop the node in the cloud
    AmazonWebServices().stop_node(aws_nodes[0], True)
    # Check node is deleted from Rancher
    wait_for_node_to_be_deleted(client, worker_node)
    # Check that the node is finally deleted in the cloud
    AmazonWebServices().wait_for_node_state(aws_nodes[0], 'terminated')

    # After shutting down the worker node self-healing should trigger
    # by deleting the unresponsive node and provisioning a new node

    nodes = wait_for_nodepool_to_have_nodes(client, cluster, pool_id)

    # Filter the nodes and get the only the worker node
    worker_node = [node for node in nodes if node.worker == True][0]

    # wait for this node to become active
    worker_node = wait_for_node_status(client, worker_node, "active")

    # make sure the new worker node is entirely new
    assert original_worker_node_id != worker_node.id

def stop_node_and_check_no_heal_ec2(nodes, pool_id):
    client = namespace['client']

    # Filter the nodes and get the only the worker node
    aws_nodes, worker_node = get_nodes_to_change_status(nodes)
    original_worker_node_id = worker_node.id

    # stop the node in the cloud
    AmazonWebServices().stop_node(aws_nodes[0], True)

    # Give time so we make sure the deleteNotReadyAfterSecs do not trigger self-heal
    worker_node = wait_for_node_status(client, worker_node, 'unavailable')
    time.sleep(11)
    nodes = client.list_node(nodePoolId=pool_id).data
    worker_node = [node for node in nodes if node.worker == True][0]
    assert worker_node.state == 'unavailable'

def wait_for_nodepool_to_have_nodes(client, cluster, pool_id,
                                    timeout=DEFAULT_MULTI_CLUSTER_APP_TIMEOUT):
    # Get the nodes again
    client.reload(cluster)
    nodes = client.list_node(nodePoolId=pool_id).data
    start = time.time()
    time.sleep(5)
    while len(nodes) == 0:
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for nodepool to have nodes")
        time.sleep(.5)
        client.reload(cluster)
        nodes = client.list_node(nodePoolId=pool_id).data
    return nodes


@pytest.fixture(scope='module')
def create_cluster_ec2_with_provider(request, node_template_ec2_with_provider):
    client = get_user_client()
    nodes = []
    node_name = random_node_name()
    node = {"hostnamePrefix": node_name,
            "nodeTemplateId": node_template_ec2_with_provider.id,
            "requestedHostname": node_name,
            "controlPlane": True,
            "quantity": 1}
    nodes.append(node)
    node_name = random_node_name()
    node = {"hostnamePrefix": node_name,
            "nodeTemplateId": node_template_ec2_with_provider.id,
            "requestedHostname": node_name,
            "etcd": True,
            "quantity": 1}
    nodes.append(node)
    node_name = random_node_name()
    node = {"hostnamePrefix": node_name,
            "nodeTemplateId": node_template_ec2_with_provider.id,
            "requestedHostname": node_name,
            "worker": True,
            "quantity": 1}
    nodes.append(node)
    node_name = random_node_name()
    node = {"hostnamePrefix": node_name,
            "nodeTemplateId": node_template_ec2_with_provider.id,
            "requestedHostname": node_name,
            "deleteNotReadyAfterSecs": 10,
            "worker": True,
            "quantity": 1}
    nodes.append(node)
    cluster, node_pools = create_and_validate_cluster(
        client, nodes, rke_config_aws_provider, CLUSTER_NAME + "-provider")

    namespace['cluster'] = cluster
    namespace['client'] = client


    def fin():
        cluster_cleanup(client, cluster)
    request.addfinalizer(fin)

@pytest.fixture(scope='module')
def create_cluster_do_without_provider(request, node_template_ec2):
    client = get_user_client()
    nodes = []
    node_name = random_node_name()
    node = {"hostnamePrefix": node_name,
            "nodeTemplateId": node_template_ec2.id,
            "requestedHostname": node_name,
            "controlPlane": True,
            "quantity": 1}
    nodes.append(node)
    node_name = random_node_name()
    node = {"hostnamePrefix": node_name,
            "nodeTemplateId": node_template_ec2.id,
            "requestedHostname": node_name,
            "etcd": True,
            "quantity": 1}
    nodes.append(node)
    node_name = random_node_name()
    node = {"hostnamePrefix": node_name,
            "nodeTemplateId": node_template_ec2.id,
            "requestedHostname": node_name,
            "worker": True,
            "quantity": 1}
    nodes.append(node)
    node_name = random_node_name()
    node = {"hostnamePrefix": node_name,
            "nodeTemplateId": node_template_ec2.id,
            "requestedHostname": node_name,
            "deleteNotReadyAfterSecs": 10,
            "worker": True,
            "quantity": 1}
    nodes.append(node)
    cluster, node_pools = create_and_validate_cluster(
        client, nodes, rke_config, CLUSTER_NAME + "-noprovider")

    namespace['cluster'] = cluster
    namespace['client'] = client

    def fin():
        cluster_cleanup(client, cluster)
    request.addfinalizer(fin)
