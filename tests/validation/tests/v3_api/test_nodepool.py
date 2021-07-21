import ast
import os
import pytest
from .common import get_user_client
from .common import get_user_client_and_cluster
from lib.aws import AmazonWebServices
from .test_rke_cluster_provisioning import random_node_name
from .test_rke_cluster_provisioning import node_template_ec2
from .test_rke_cluster_provisioning import create_and_validate_cluster
from .test_rke_cluster_provisioning import rke_config
from .test_ebs_volume_backed_instance import get_aws_nodes_from_nodepools
DRAIN_BEFORE_DELETE = ast.literal_eval(
    os.environ.get('RANCHER_DRAIN_BEFORE_DELETE', "False")
    )
cluster_details = { "cluster": None}    

def test_nodepool_delete():
    node_template = node_template_ec2()
    client = get_user_client()
    nodes = []
    node = get_node_pool(node_template, "controlPlane", 1)
    nodes.append(node)
    node = get_node_pool(node_template, "etcd", 1)
    nodes.append(node)
    node = get_node_pool(node_template, "worker", 3)
    nodes.append(node)
    node = get_node_pool(node_template, "worker", 2)
    nodes.append(node)
    cluster, node_pools = create_and_validate_cluster(
        client, nodes, rke_config
        )
    cluster_details["cluster"] = cluster
    #delete a worker nodepool
    for nodepool in node_pools:
        if nodepool["worker"]:
            nodes = client.list_node(nodePoolId=nodepool.id)
            aws_nodes = []
            for node in nodes:
                print("node.externalIpAddress: ", node.externalIpAddress)
                filters = [{'Name': 'ip-address','Values': [node.externalIpAddress]}]
                aws_nodes.append(AmazonWebServices().get_nodes(filters))
            client.delete(nodepool)
            # wait for nodes to be terminated in AWS
            assert len(aws_nodes[0]) > 0, "Nodes' info is not available"
            terminated_nodes = AmazonWebServices().wait_for_nodes_state(
                aws_nodes[0],state="terminated"
                )
            assert len(terminated_nodes) == len(nodes), "Nodes are not deleted in AWS"
            break


@pytest.fixture(scope='module', autouse="True")
def cleanup_cluster(request):

    def fin():
        client = get_user_client()
        client.delete(cluster_details["cluster"])

    request.addfinalizer(fin)

    
def get_node_pool(node_template, role, quantity):
    node_name = random_node_name()
    node = {"hostnamePrefix": node_name,
            "nodeTemplateId": node_template.id,
            "requestedHostname": node_name,
            "quantity": quantity}
    if role == "worker":
        node["worker"] = True
    elif role == "controlPlane":
        node["controlPlane"] = True
    else: 
        node["etcd"] = True
    return node
