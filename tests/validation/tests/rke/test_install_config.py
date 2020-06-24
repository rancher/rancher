from .conftest import *  # NOQA
from .common import *  # NOQA


def test_install_config_1(test_name, cloud_provider, rke_client, kubectl):
    """
    Node Address specified as just IP and using only this in the node spec
    """
    rke_template = 'cluster_install_config_11.yml.j2'
    nodes = cloud_provider.create_multiple_nodes(3, test_name)
    create_and_validate(
        cloud_provider, rke_client, kubectl, rke_template, nodes,
        remove_nodes=True)


def test_install_config_2(test_name, cloud_provider, rke_client, kubectl):
    """
    Node Address specified as FQDN and using only this in the node spec
    """
    rke_template = 'cluster_install_config_2.yml.j2'
    nodes = cloud_provider.create_multiple_nodes(3, test_name)
    create_and_validate(
        cloud_provider, rke_client, kubectl, rke_template, nodes,
        remove_nodes=True)


def test_install_config_3(test_name, cloud_provider, rke_client, kubectl):
    """
    Hostname override specified as a non resolvable name
    """
    rke_template = 'cluster_install_config_3.yml.j2'
    nodes = cloud_provider.create_multiple_nodes(3, test_name)
    # set node_name to non-resolvable name for hostname_override
    index = 0
    for node in nodes:
        node.node_name = "{0}-{1}".format(test_name, index)
        index += 1
    create_and_validate(
        cloud_provider, rke_client, kubectl, rke_template, nodes,
        remove_nodes=True)


def test_install_config_4(test_name, cloud_provider, rke_client, kubectl):
    """
    Hostname override specified as a resolvable name
    """
    rke_template = 'cluster_install_config_4.yml.j2'
    nodes = cloud_provider.create_multiple_nodes(3, test_name)
    # set node_name to the resolvable host_name for hostname_override
    for node in nodes:
        node.node_name = node.host_name
    create_and_validate(
        cloud_provider, rke_client, kubectl, rke_template, nodes,
        remove_nodes=True)


def test_install_config_5(test_name, cloud_provider, rke_client, kubectl):
    """
    Internal address provided
    """
    rke_template = 'cluster_install_config_5.yml.j2'
    nodes = cloud_provider.create_multiple_nodes(3, test_name)
    create_and_validate(
        cloud_provider, rke_client, kubectl, rke_template, nodes,
        remove_nodes=True, etcd_private_ip=True)


def test_install_config_6(test_name, cloud_provider, rke_client, kubectl):
    """
    Providing address, hostname override(resolvable) and internal address
    """
    rke_template = 'cluster_install_config_6.yml.j2'
    nodes = cloud_provider.create_multiple_nodes(3, test_name)
    # set node_name to the resolvable host_name for hostname_override
    for node in nodes:
        node.node_name = node.host_name
    create_and_validate(
        cloud_provider, rke_client, kubectl, rke_template, nodes,
        remove_nodes=True, etcd_private_ip=True)


def test_install_config_7(test_name, cloud_provider, rke_client, kubectl):
    """
    Providing address, hostname override(non-resolvable) and internal address
    """
    rke_template = 'cluster_install_config_7.yml.j2'
    nodes = cloud_provider.create_multiple_nodes(3, test_name)
    # set node_name to non-resolvable name for hostname_override
    index = 0
    for node in nodes:
        node.node_name = "{0}-{1}".format(test_name, index)
        index += 1
    create_and_validate(
        cloud_provider, rke_client, kubectl, rke_template, nodes,
        remove_nodes=True, etcd_private_ip=True)


def test_install_config_8(test_name, cloud_provider, rke_client, kubectl):
    """
    Create a cluster with minimum possible values, will use the defaulted
    network plugin for RKE
    """
    rke_template = 'cluster_install_config_8.yml.j2'
    nodes = cloud_provider.create_multiple_nodes(3, test_name)
    create_and_validate(
        cloud_provider, rke_client, kubectl, rke_template, nodes,
        remove_nodes=True)


def test_install_config_9(test_name, cloud_provider, rke_client, kubectl):
    """
    Launch a cluster with unencrypted ssh keys
    """
    key_name = 'install-config-9'
    rke_template = 'cluster_install_config_9.yml.j2'
    public_key = cloud_provider.generate_ssh_key(key_name)
    cloud_provider.import_ssh_key(key_name + '.pub', public_key)
    nodes = cloud_provider.create_multiple_nodes(
        3, test_name, key_name=key_name + '.pub')
    create_and_validate(
        cloud_provider, rke_client, kubectl, rke_template, nodes,
        remove_nodes=True)


# def test_install_config_10(test_name, cloud_provider, rke_client, kubectl):
#     """
#     Launch a cluster with encrypted ssh keys
#     """
#     rke_template = 'cluster_install_config_10.yml.j2'
#     nodes = cloud_provider.create_multiple_nodes(3, test_name)
#     create_rke_cluster(rke_client, kubectl, nodes, rke_template)
#     validate_rke_cluster(rke_client, kubectl, nodes)
#     for node in nodes:
#         cloud_provider.delete_node(node)
