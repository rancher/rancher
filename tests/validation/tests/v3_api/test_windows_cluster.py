
from .common import TEST_IMAGE
from .common import TEST_IMAGE_NGINX
from .common import TEST_IMAGE_OS_BASE
from .common import cluster_cleanup
from .common import get_user_client
from .common import random_test_name
from .test_rke_cluster_provisioning import create_custom_host_from_nodes
from .test_rke_cluster_provisioning import HOST_NAME

from lib.aws import AmazonWebServices
from threading import Thread

def test_windows_provisioning():
    node_roles_linux = [["controlplane"], ["etcd"], ["worker"]]
    node_roles_windows = [["worker"], ["worker"], ["worker"]]

    win_nodes = \
        AmazonWebServices().create_multiple_nodes(
            len(node_roles_windows), random_test_name(HOST_NAME), os_version="windows-1903", 
            docker_version="19.03")

    aws_nodes = \
        AmazonWebServices().create_multiple_nodes(
            len(node_roles_linux), random_test_name(HOST_NAME))

    nodes = aws_nodes + win_nodes
    node_roles = node_roles_linux + node_roles_windows

    cluster, nodes = create_custom_host_from_nodes(nodes, 
                                                       node_roles, 
                                                       random_cluster_name=True, 
                                                       windows=True)
                                                       
    for node in win_nodes:
        pull_images(node)
        
    cluster_cleanup(get_user_client(), cluster, nodes)

def pull_images(node):
    print("Pulling images on node: " + node.host_name)
    pull_result = node.execute_command("docker pull " + TEST_IMAGE + " && " + "docker pull " + TEST_IMAGE_NGINX + " && " + "docker pull " + TEST_IMAGE_OS_BASE)
    print(pull_result)