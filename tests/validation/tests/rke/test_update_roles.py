from .conftest import *  # NOQA
from .common import *  # NOQA


def test_update_roles_1(test_name, cloud_provider, rke_client, kubectl):
    """
    Update cluster adding a worker node
    Before Update: Create three node cluster, each node with a single role
    node0 - controlplane
    node1 - etcd
    node2 - worker
    After Update: Adds a worker
    node0 - controlplane
    node1 - etcd
    node2 - worker
    node3 - worker
    """
    all_nodes = cloud_provider.create_multiple_nodes(4, test_name)

    # Only use three nodes at first
    before_update_nodes = all_nodes[0:-1]
    rke_template = 'cluster_update_roles_1_1.yml.j2'
    network, dns_discovery = create_and_validate(
        cloud_provider, rke_client, kubectl, rke_template, before_update_nodes,
        base_namespace='beforeupdate')

    # Update adding worker node, rerun on existing validation pods
    rke_template = 'cluster_update_roles_1_2.yml.j2'
    create_and_validate(
        cloud_provider, rke_client, kubectl, rke_template, all_nodes,
        base_namespace='beforeupdate', network_validation=network,
        dns_validation=dns_discovery)
    # Create another validation setup on updated cluster
    validate_rke_cluster(rke_client, kubectl, all_nodes, 'afterupdate')
    delete_nodes(cloud_provider, all_nodes)


def test_update_roles_2(test_name, cloud_provider, rke_client, kubectl):
    """
    Update cluster adding a worker node, then remove original worker node
    Before Update: Create three node cluster, each node with a single role
    node0 - controlplane
    node1 - etcd
    node2 - worker
    After Update: Adds a worker
    node0 - controlplane
    node1 - etcd
    node2 - worker <- will be deleted on next update
    node3 - worker
    After 2nd Update: Deletes original worker
    node0 - controlplane
    node1 - etcd
    node3 - worker
    """
    all_nodes = cloud_provider.create_multiple_nodes(4, test_name)
    before_update_nodes = all_nodes[0:-1]
    removed_node_nodes = all_nodes[0:2] + all_nodes[3:]
    # all_nodes[0:2] = [node0, node1]
    # all_nodes[3:] = [node3]

    # Only use three nodes at first
    rke_template = 'cluster_update_roles_2_1.yml.j2'
    network, dns_discovery = create_and_validate(
        cloud_provider, rke_client, kubectl, rke_template, before_update_nodes,
        base_namespace='beforeupdate')

    # Update adding worker node, rerun on existing validation pods
    rke_template = 'cluster_update_roles_2_2.yml.j2'
    network, dns_discovery = create_and_validate(
        cloud_provider, rke_client, kubectl, rke_template, all_nodes,
        base_namespace='beforeupdate', network_validation=network,
        dns_validation=dns_discovery)
    # Create another validation setup on updated cluster
    network_update1, dns_discovery_update1 = validate_rke_cluster(
        rke_client, kubectl, all_nodes, 'afterupdate1')

    # Update removing original worker node, rerun on existing validation pods
    rke_template = 'cluster_update_roles_2_3.yml.j2'
    create_and_validate(
        cloud_provider, rke_client, kubectl, rke_template, removed_node_nodes,
        base_namespace='beforeupdate', network_validation=network,
        dns_validation=dns_discovery)
    # Create another validation setup on updated cluster
    validate_rke_cluster(
        rke_client, kubectl, removed_node_nodes, 'afterupdate1',
        network_validation=network_update1,
        dns_validation=dns_discovery_update1)
    validate_rke_cluster(
        rke_client, kubectl, removed_node_nodes, 'afterupdate2')
    delete_nodes(cloud_provider, all_nodes)


def test_update_roles_3(test_name, cloud_provider, rke_client, kubectl):
    """
    Update cluster adding a controlplane node
    Before Update: Create three node cluster, each node with a single role
    node0 - controlplane
    node1 - etcd
    node2 - worker
    After Update: Adds a controlplane
    node0 - controlplane
    node1 - etcd
    node2 - worker
    node3 - controlplane
    """
    all_nodes = cloud_provider.create_multiple_nodes(4, test_name)
    before_update_nodes = all_nodes[0:-1]  # only use three nodes at first

    # Only use three nodes at first
    rke_template = 'cluster_update_roles_3_1.yml.j2'
    network, dns_discovery = create_and_validate(
        cloud_provider, rke_client, kubectl, rke_template, before_update_nodes,
        base_namespace='beforeupdate')

    # Update adding controlplane node, rerun on existing validation pods
    rke_template = 'cluster_update_roles_3_2.yml.j2'
    create_and_validate(
        cloud_provider, rke_client, kubectl, rke_template, all_nodes,
        base_namespace='beforeupdate', network_validation=network,
        dns_validation=dns_discovery)

    # Create another validation setup on updated cluster
    validate_rke_cluster(rke_client, kubectl, all_nodes, 'afterupdate')
    delete_nodes(cloud_provider, all_nodes)


def test_update_roles_4(test_name, cloud_provider, rke_client, kubectl):
    """
    Update cluster adding a controlplane node, remove original controlplane
    Before Update: Create three node cluster, each node with a single role
    node0 - controlplane
    node1 - etcd
    node2 - worker
    After Update: Adds a controlplane
    node0 - controlplane <- will be deleted on next update
    node1 - etcd
    node2 - worker
    node3 - controlplane
    After 2nd Update: Deletes original controlplane
    node1 - etcd
    node2 - worker
    node3 - controlplane
    """
    all_nodes = cloud_provider.create_multiple_nodes(4, test_name)
    before_update_nodes = all_nodes[0:-1]  # only use three nodes at first
    removed_node_nodes = all_nodes[1:]

    rke_template = 'cluster_update_roles_4_1.yml.j2'
    network, dns_discovery = create_and_validate(
        cloud_provider, rke_client, kubectl, rke_template, before_update_nodes,
        base_namespace='beforeupdate')

    # Update adding controlplane node, rerun on existing validation pods
    rke_template = 'cluster_update_roles_4_2.yml.j2'
    network, dns_discovery = create_and_validate(
        cloud_provider, rke_client, kubectl, rke_template, all_nodes,
        base_namespace='beforeupdate', network_validation=network,
        dns_validation=dns_discovery)
    # Create another validation setup on updated cluster
    network_update1, dns_discovery_update1 = validate_rke_cluster(
        rke_client, kubectl, all_nodes, 'afterupdate1')

    # Update removing original controlplane node
    # rerun on existing validation pods
    # all_nodes[1:] = [node1, node2, node3]
    rke_template = 'cluster_update_roles_4_3.yml.j2'
    create_and_validate(
        cloud_provider, rke_client, kubectl, rke_template, removed_node_nodes,
        base_namespace='beforeupdate', network_validation=network,
        dns_validation=dns_discovery)
    # Create another validation setup on updated cluster
    validate_rke_cluster(
        rke_client, kubectl, removed_node_nodes, 'afterupdate1',
        network_validation=network_update1,
        dns_validation=dns_discovery_update1)
    validate_rke_cluster(
        rke_client, kubectl, removed_node_nodes, 'afterupdate2')
    delete_nodes(cloud_provider, all_nodes)


def test_update_roles_5(test_name, cloud_provider, rke_client, kubectl):
    """
    Update cluster adding a etcd node to a single node cluster
    Before Update: Create single node cluster with all roles
    node0 - controlplane, etcd, worker
    After Update: Adds a etcd
    node0 - controlplane, etcd, worker
    node1 - etcd
    """
    all_nodes = cloud_provider.create_multiple_nodes(2, test_name)
    before_update_nodes = all_nodes[0:-1]  # only use one node at first

    rke_template = 'cluster_update_roles_5_1.yml.j2'
    network, dns_discovery = create_and_validate(
        cloud_provider, rke_client, kubectl, rke_template, before_update_nodes,
        base_namespace='beforeupdate')

    # Update adding etcd node, rerun on existing validation pods
    rke_template = 'cluster_update_roles_5_2.yml.j2'
    network, dns_discovery = create_and_validate(
        cloud_provider, rke_client, kubectl, rke_template, all_nodes,
        base_namespace='beforeupdate', network_validation=network,
        dns_validation=dns_discovery)
    # Create another validation setup on updated cluster
    validate_rke_cluster(rke_client, kubectl, all_nodes, 'afterupdate')
    delete_nodes(cloud_provider, all_nodes)


def test_update_roles_6(test_name, cloud_provider, rke_client, kubectl):
    """
    Update cluster adding two etcd nodes to a single node cluster
    Before Update: Create single node cluster with all roles
    node0 - controlplane, etcd, worker
    After Update: Adds two etcd nodes
    node0 - controlplane, etcd, worker
    node1 - etcd
    node2 - etcd
    """
    all_nodes = cloud_provider.create_multiple_nodes(3, test_name)
    before_update_nodes = all_nodes[0:-2]  # only use one node at first

    rke_template = 'cluster_update_roles_6_1.yml.j2'
    network, dns_discovery = create_and_validate(
        cloud_provider, rke_client, kubectl, rke_template, before_update_nodes,
        base_namespace='beforeupdate')

    # Update adding 2 etcd nodes, rerun on existing validation pods
    rke_template = 'cluster_update_roles_6_2.yml.j2'
    create_and_validate(
        cloud_provider, rke_client, kubectl, rke_template, all_nodes,
        base_namespace='beforeupdate', network_validation=network,
        dns_validation=dns_discovery)

    # Create another validation setup on updated cluster
    validate_rke_cluster(rke_client, kubectl, all_nodes, 'afterupdate')
    delete_nodes(cloud_provider, all_nodes)


def test_update_roles_7(test_name, cloud_provider, rke_client, kubectl):
    """
    Update cluster deleting one node with all roles in three node cluster
    Before Update: Create three node cluster with all roles
    node0 - controlplane, etcd, worker
    node1 - worker
    node2 - etcd
    After Update: Remove last node
    node0 - controlplane, etcd, worker
    node1 - worker
    """
    all_nodes = cloud_provider.create_multiple_nodes(3, test_name)
    removed_node_nodes = all_nodes[0:-1]

    rke_template = 'cluster_update_roles_7_1.yml.j2'
    network, dns_discovery = create_and_validate(
        cloud_provider, rke_client, kubectl, rke_template, all_nodes,
        base_namespace='beforeupdate')

    # Update remove etcd node, rerun on existing validation pods
    rke_template = 'cluster_update_roles_7_2.yml.j2'
    create_and_validate(
        cloud_provider, rke_client, kubectl, rke_template, removed_node_nodes,
        base_namespace='beforeupdate', network_validation=network,
        dns_validation=dns_discovery)

    # Create another validation setup on updated cluster
    validate_rke_cluster(
        rke_client, kubectl, removed_node_nodes, 'afterupdate')
    delete_nodes(cloud_provider, all_nodes)


def test_update_roles_8(test_name, cloud_provider, rke_client, kubectl):
    """
    Create a single node cluster, add second node with all roles, and then
    delete the original node
    Before Update: Create single node cluster with all roles
    node0 - controlplane, etcd, worker
    After Update: Add second node with all roles
    node0 - controlplane, etcd, worker
    node1 - controlplane, etcd, worker
    After second Update: Remove original node0
    node1 - controlplane, etcd, worker
    """
    all_nodes = cloud_provider.create_multiple_nodes(2, test_name)
    before_update_nodes = all_nodes[0:-1]
    removed_node_nodes = all_nodes[1:]

    # Inital cluster
    rke_template = 'cluster_update_roles_8_1.yml.j2'
    network, dns_discovery = create_and_validate(
        cloud_provider, rke_client, kubectl, rke_template, before_update_nodes,
        base_namespace='beforeupdate')

    # Update create a second node will all roles
    rke_template = 'cluster_update_roles_8_2.yml.j2'
    network, dns_discovery = create_and_validate(
        cloud_provider, rke_client, kubectl, rke_template, all_nodes,
        base_namespace='beforeupdate', network_validation=network,
        dns_validation=dns_discovery)
    # Create another validation setup on updated cluster
    network_update1, dns_discovery_update1 = validate_rke_cluster(
        rke_client, kubectl, all_nodes, 'afterupdate1')

    # Update remove original node with all roles
    rke_template = 'cluster_update_roles_8_1.yml.j2'
    create_and_validate(
        cloud_provider, rke_client, kubectl, rke_template, removed_node_nodes,
        base_namespace='beforeupdate', network_validation=network,
        dns_validation=dns_discovery)
    validate_rke_cluster(
        rke_client, kubectl, removed_node_nodes, 'afterupdate1',
        network_validation=network_update1,
        dns_validation=dns_discovery_update1)
    # Create another validation setup on updated cluster
    validate_rke_cluster(
        rke_client, kubectl, removed_node_nodes, 'afterupdate2')

    delete_nodes(cloud_provider, all_nodes)


def test_update_roles_9(test_name, cloud_provider, rke_client, kubectl):
    """
    Update cluster adding a controlplane,worker node
    Before Update: Create three node cluster, each node with a single role
    node0 - controlplane
    node1 - etcd
    node2 - worker
    After Update: Adds a controlplane, worker
    node0 - controlplane
    node1 - etcd
    node2 - worker
    node3 - controlplane, worker
    """
    all_nodes = cloud_provider.create_multiple_nodes(4, test_name)
    before_update_nodes = all_nodes[0:-1]  # only use three nodes at first

    # Inital cluster
    rke_template = 'cluster_update_roles_9_1.yml.j2'
    network, dns_discovery = create_and_validate(
        cloud_provider, rke_client, kubectl, rke_template, before_update_nodes,
        base_namespace='beforeupdate')

    # Update adds node with roles [controlplane,worker]
    rke_template = 'cluster_update_roles_9_2.yml.j2'
    create_and_validate(
        cloud_provider, rke_client, kubectl, rke_template, all_nodes,
        base_namespace='beforeupdate', network_validation=network,
        dns_validation=dns_discovery)
    # Create another validation setup on updated cluster
    validate_rke_cluster(rke_client, kubectl, all_nodes, 'afterupdate')

    delete_nodes(cloud_provider, all_nodes)


def test_update_roles_10(test_name, cloud_provider, rke_client, kubectl):
    """
    Update cluster adding a controlplane,worker node
    Before Update: Create three node cluster, each node with a single role
    node0 - controlplane
    node1 - etcd
    node2 - worker
    After Update: Adds a controlplane, etcd
    node0 - controlplane
    node1 - etcd
    node2 - worker
    node3 - controlplane, etcd
    """
    all_nodes = cloud_provider.create_multiple_nodes(4, test_name)
    before_update_nodes = all_nodes[0:-1]  # only use three nodes at first

    # Inital cluster
    rke_template = 'cluster_update_roles_10_1.yml.j2'
    network, dns_discovery = create_and_validate(
        cloud_provider, rke_client, kubectl, rke_template, before_update_nodes,
        base_namespace='beforeupdate')

    # Update adds node with roles [controlplane,etcd]
    rke_template = 'cluster_update_roles_10_2.yml.j2'
    create_and_validate(
        cloud_provider, rke_client, kubectl, rke_template, all_nodes,
        base_namespace='beforeupdate', network_validation=network,
        dns_validation=dns_discovery)
    # Create another validation setup on updated cluster
    validate_rke_cluster(rke_client, kubectl, all_nodes, 'afterupdate')

    delete_nodes(cloud_provider, all_nodes)


def test_update_roles_11(test_name, cloud_provider, rke_client, kubectl):
    """
    Update cluster adding a worker, etcd node
    Before Update: Create three node cluster, each node with a single role
    node0 - controlplane
    node1 - etcd
    node2 - worker
    After Update: Adds a etcd, worker
    node0 - controlplane
    node1 - etcd
    node2 - worker
    node3 - worker, etcd
    """
    all_nodes = cloud_provider.create_multiple_nodes(4, test_name)
    before_update_nodes = all_nodes[0:-1]  # only use three nodes at first

    # Inital cluster
    rke_template = 'cluster_update_roles_11_1.yml.j2'
    network, dns_discovery = create_and_validate(
        cloud_provider, rke_client, kubectl, rke_template, before_update_nodes,
        base_namespace='beforeupdate')

    # Update adds node with roles [worker,etcd]
    rke_template = 'cluster_update_roles_11_2.yml.j2'
    create_and_validate(
        cloud_provider, rke_client, kubectl, rke_template, all_nodes,
        base_namespace='beforeupdate', network_validation=network,
        dns_validation=dns_discovery)
    # Create another validation setup on updated cluster
    validate_rke_cluster(rke_client, kubectl, all_nodes, 'afterupdate')

    delete_nodes(cloud_provider, all_nodes)


def test_update_roles_12(test_name, cloud_provider, rke_client, kubectl):
    """
    Update cluster adding a controlplane,worker node
    Before Update:
    node0 - etcd
    node1 - worker
    node2 - controlplane
    node3 - controlplane
    node4 - controlplane
    After Update: Adds a controlplane, worker
    node0 - etcd
    node1 - worker
    node5 - controlplane <- New contolplane node
    """
    all_nodes = cloud_provider.create_multiple_nodes(6, test_name)
    before_update_nodes = all_nodes[0:-1]  # only use five nodes at first
    # all_nodes[0:2] = [node0, node1]
    # all_nodes[5:] = [node5]
    removed_node_nodes = all_nodes[0:2] + all_nodes[5:]

    # Inital cluster
    rke_template = 'cluster_update_roles_12_1.yml.j2'
    network, dns_discovery = create_and_validate(
        cloud_provider, rke_client, kubectl, rke_template, before_update_nodes,
        base_namespace='beforeupdate')

    # Update removes all existing controlplane nodes,
    # adds node with [controlplane]
    rke_template = 'cluster_update_roles_12_2.yml.j2'
    create_and_validate(
        cloud_provider, rke_client, kubectl, rke_template, removed_node_nodes,
        base_namespace='beforeupdate', network_validation=network,
        dns_validation=dns_discovery)
    # Create another validation setup on updated cluster
    validate_rke_cluster(
        rke_client, kubectl, removed_node_nodes, 'afterupdate')

    delete_nodes(cloud_provider, all_nodes)


def test_update_roles_13(test_name, cloud_provider, rke_client, kubectl):
    """
    Update cluster changing a worker role to controlplane
    Before Update: Create four node cluster, each node with these roles:
    node0 - controlplane
    node1 - controlplane, etcd
    node2 - worker, etcd
    node3 - worker, etcd
    After Update: Change node2 worker to controlplane
    node0 - controlplane
    node1 - controlplane, etcd
    node2 - controlplane, etcd
    node3 - worker, etcd
    """
    all_nodes = cloud_provider.create_multiple_nodes(4, test_name)

    rke_template = 'cluster_update_roles_13_1.yml.j2'
    network, dns_discovery = create_and_validate(
        cloud_provider, rke_client, kubectl, rke_template, all_nodes,
        base_namespace='beforeupdate')

    # Update changes role worker to controlplane on node2
    rke_template = 'cluster_update_roles_13_2.yml.j2'
    create_and_validate(
        cloud_provider, rke_client, kubectl, rke_template, all_nodes,
        base_namespace='beforeupdate', network_validation=network,
        dns_validation=dns_discovery)
    # Create another validation setup on updated cluster
    validate_rke_cluster(rke_client, kubectl, all_nodes, 'afterupdate')

    delete_nodes(cloud_provider, all_nodes)


def test_update_roles_14(test_name, cloud_provider, rke_client, kubectl):
    """
    Update cluster changing a controlplane, etcd to etcd only
    Before Update: Create four node cluster, each node with these roles:
    node0 - controlplane
    node1 - controlplane, etcd
    node2 - worker, etcd
    node3 - worker, etcd
    After Update: Change node1 controlplane, etcd to etcd
    node0 - controlplane
    node1 - etcd
    node2 - worker, etcd
    node3 - worker, etcd
    """
    all_nodes = cloud_provider.create_multiple_nodes(4, test_name)

    rke_template = 'cluster_update_roles_14_1.yml.j2'
    network, dns_discovery = create_and_validate(
        cloud_provider, rke_client, kubectl, rke_template, all_nodes,
        base_namespace='beforeupdate')

    # Update remove controlplane on node1
    rke_template = 'cluster_update_roles_14_2.yml.j2'
    create_and_validate(
        cloud_provider, rke_client, kubectl, rke_template, all_nodes,
        base_namespace='beforeupdate', network_validation=network,
        dns_validation=dns_discovery)
    # Create another validation setup on updated cluster
    validate_rke_cluster(rke_client, kubectl, all_nodes, 'afterupdate')

    delete_nodes(cloud_provider, all_nodes)


def test_update_roles_15(test_name, cloud_provider, rke_client, kubectl):
    """
    Update cluster remove worker role from [worker,etcd] node
    Before Update: Create four node cluster, each node with these roles:
    node0 - controlplane
    node1 - controlplane, etcd
    node2 - worker, etcd
    node3 - worker, etcd
    After Update: Change remove worker role node2
    node0 - controlplane
    node1 - controlplane, etcd
    node2 - etcd
    node3 - worker, etcd
    """
    all_nodes = cloud_provider.create_multiple_nodes(4, test_name)

    rke_template = 'cluster_update_roles_15_1.yml.j2'
    network, dns_discovery = create_and_validate(
        cloud_provider, rke_client, kubectl, rke_template, all_nodes,
        base_namespace='beforeupdate')

    # Update remove worker role node2
    rke_template = 'cluster_update_roles_15_2.yml.j2'
    create_and_validate(
        cloud_provider, rke_client, kubectl, rke_template, all_nodes,
        base_namespace='beforeupdate', network_validation=network,
        dns_validation=dns_discovery)
    # Create another validation setup on updated cluster
    validate_rke_cluster(rke_client, kubectl, all_nodes, 'afterupdate')

    delete_nodes(cloud_provider, all_nodes)
