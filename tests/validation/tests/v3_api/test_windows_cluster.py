import os
from .common import TEST_IMAGE
from .common import TEST_IMAGE_NGINX
from .common import TEST_IMAGE_OS_BASE
from .common import cluster_cleanup
from .common import get_user_client
from .common import random_test_name
from .test_rke_cluster_provisioning import create_custom_host_from_nodes
from .test_rke_cluster_provisioning import HOST_NAME

from lib.aws import AmazonWebServices, AWS_DEFAULT_AMI, AWS_DEFAULT_USER, \
    WINDOWS_AWS_AMI, WINDOWS_AWS_USER

K8S_VERSION = os.environ.get('RANCHER_K8S_VERSION', "")


def provision_windows_nodes():
    node_roles_linux = [["controlplane"], ["etcd"], ["worker"]]
    node_roles_windows = [["worker"], ["worker"], ["worker"]]

    win_nodes = \
        AmazonWebServices().create_multiple_nodes(
            len(node_roles_windows), random_test_name(HOST_NAME),
            ami=WINDOWS_AWS_AMI, ssh_user=WINDOWS_AWS_USER)

    linux_nodes = \
        AmazonWebServices().create_multiple_nodes(
            len(node_roles_linux), random_test_name(HOST_NAME),
            ami=AWS_DEFAULT_AMI, ssh_user=AWS_DEFAULT_USER)

    nodes = linux_nodes + win_nodes
    node_roles = node_roles_linux + node_roles_windows
    return nodes, node_roles, win_nodes


def test_windows_provisioning_vxlan():
    cluster, nodes, win_nodes = create_windows_cluster()
    for node in win_nodes:
        pull_images(node)
    
    cluster_cleanup(get_user_client(), cluster, nodes)


def test_windows_provisioning_gw_host():
    cluster, nodes, win_nodes = create_windows_cluster(flannel_backend="host-gw")
    for node in win_nodes:
        pull_images(node)

    cluster_cleanup(get_user_client(), cluster, nodes)


def pull_images(node):
    print("Pulling images on node: " + node.host_name)
    pull_result = node.execute_command("docker pull " + TEST_IMAGE
                                       + " && " +
                                       "docker pull " + TEST_IMAGE_NGINX
                                       + " && " +
                                       "docker pull " + TEST_IMAGE_OS_BASE)
    print(pull_result)


def create_windows_cluster(version=K8S_VERSION, flannel_backend='vxlan'):
    nodes, node_roles, win_nodes = provision_windows_nodes()
    if flannel_backend == 'host-gw':
        for node in nodes:
            AmazonWebServices().disable_source_dest_check(node.provider_node_id)
    
    cluster, nodes = create_custom_host_from_nodes(nodes, node_roles,
                                                   windows=True,
                                                   windows_flannel_backend=flannel_backend,
                                                   version=version)
    return cluster, nodes, win_nodes     
