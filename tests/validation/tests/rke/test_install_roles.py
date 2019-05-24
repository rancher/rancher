from .conftest import *  # NOQA
from .common import *  # NOQA


def test_install_roles_1(test_name, cloud_provider, rke_client, kubectl):
    """
    Create cluster with single node with roles controlplane, worker, etcd
    """
    rke_template = 'cluster_install_roles_1.yml.j2'
    nodes = cloud_provider.create_multiple_nodes(1, test_name)
    create_and_validate(
        cloud_provider, rke_client, kubectl, rke_template, nodes,
        remove_nodes=True)


def test_install_roles_2(test_name, cloud_provider, rke_client, kubectl):
    """
    Create cluster with 3 nodes having each with single role:
    node0 - controlplane, node1 - etcd, node2 - worker
    """
    rke_template = 'cluster_install_roles_2.yml.j2'
    nodes = cloud_provider.create_multiple_nodes(3, test_name)
    create_and_validate(
        cloud_provider, rke_client, kubectl, rke_template, nodes,
        remove_nodes=True)


def test_install_roles_3(test_name, cloud_provider, rke_client, kubectl):
    """
    Create cluster with 3 nodes having all three roles
    """
    rke_template = 'cluster_install_roles_3.yml.j2'
    nodes = cloud_provider.create_multiple_nodes(3, test_name)
    create_and_validate(
        cloud_provider, rke_client, kubectl, rke_template, nodes,
        remove_nodes=True)


def test_install_roles_4(test_name, cloud_provider, rke_client, kubectl):
    """
    Create a 4 node node cluster with nodes having these roles:
    node0 - etcd, controlplane
    node1 - etcd, worker
    node2 - controlplane, worker
    node3 - controlplane, etcd, worker
    """
    rke_template = 'cluster_install_roles_4.yml.j2'
    nodes = cloud_provider.create_multiple_nodes(4, test_name)
    create_and_validate(
        cloud_provider, rke_client, kubectl, rke_template, nodes,
        remove_nodes=True)
