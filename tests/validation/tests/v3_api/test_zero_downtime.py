from .common import *
import pytest
import time

from .test_rke_cluster_provisioning import create_custom_host_from_nodes

CLUSTER_NAME = "test1"
HOST_NAME = os.environ.get('RANCHER_HOST_NAME', "testZeroDT")
host = "test" + str(random_int(10000, 99999)) + ".com"
path = "/name.html"

namespace = {"p_client": None, "ns": None, "cluster": None, "project": None,
             "nodes": []}
backup_info = {"backupname": None, "backup_id": None, "workload": None,
               "backupfilename": None, "etcdbackupdata": None}
default_k8s_versions = ['v1.15.12-rancher2-2', 'v1.16.10-rancher2-1', 'v1.17.6-rancher2-1']
preupgrade_k8s = default_k8s_versions[0]
postupgrade_k8s = default_k8s_versions[1]
node_ver = postupgrade_k8s.split("-")[0]


@pytest.mark.skip(reason="tested")
def test_zdt():
    client = get_user_client()
    cluster = namespace["cluster"]
    cluster, workload, ingress = validate_cluster_and_ingress(client, cluster,
                                                              check_intermediate_state=False)
    # Update Cluster to k8 version + upgrade strategy maxUnavailable worker
    cluster = client.update_by_id_cluster(
        id=cluster.id,
        name="test1",
        rancherKubernetesEngineConfig={
            "kubernetesVersion": postupgrade_k8s,
            "upgradeStrategy": {
                'drain': False,
                'maxUnavailableWorker': '10%',
                'type': '/v3/schemas/nodeUpgradeStrategy'}}
    )
    nodes = client.list_node(clusterId=cluster.id).data
    # Check Ingress is up during update
    wait_for_node_upgrade(nodes, client, workload)
    # Validate update has went through
    for node in nodes:
        node = client.reload(node)
        assert node["info"]["kubernetes"]["kubeletVersion"] == node_ver
    # Go through each node for k8 version upgrade and ensure ingress is still up


def wait_for_node_upgrade(nodes, client, workload, timeout=2000):
    start = time.time()
    upgrade_nodes = set()
    p_client = namespace["p_client"]
    cluster = namespace["cluster"]
    while len(upgrade_nodes) != len(nodes):
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for K8 update")
        validate_ingress(p_client, cluster, [workload], host, path)
        for node in nodes:
            node = client.reload(node)
            node_k8 = node["info"]["kubernetes"]["kubeletVersion"]
            if node_k8 == node_ver:
                upgrade_nodes.add(node.uuid)
        time.sleep(5)


# @pytest.mark.skip(reason="tested")
def test_zdt_drain():
    client = get_user_client()
    cluster = namespace["cluster"]
    cluster, workload, ingress = validate_cluster_and_ingress(client, cluster,
                                                              check_intermediate_state=False,
                                                              )
    # Update Cluster to k8 version + upgrade strategy maxUnavailable worker
    cluster = client.update_by_id_cluster(
        id=cluster.id,
        name="test1",
        rancherKubernetesEngineConfig={
            "kubernetesVersion": postupgrade_k8s,
            "upgradeStrategy": {
                'drain': True,
                'maxUnavailableWorker': '10%',
                'type': '/v3/schemas/nodeUpgradeStrategy'}}
    )
    node_ver = postupgrade_k8s.split("-")[0]
    max_unavailable = 1
    upgrade_nodes = []
    etcd_nodes = get_etcd_nodes(cluster, client)
    cp_nodes = get_cp_nodes(cluster, client)
    worker_nodes = get_worker_nodes(cluster, client)
    cp_upgraded = validate_node_cordon(cp_nodes, workload)
    for upgraded in cp_upgraded:
        upgrade_nodes.append(upgraded)
    etcd_upgraded = validate_node_cordon(etcd_nodes, workload)
    for upgraded in etcd_upgraded:
        upgrade_nodes.append(upgraded)
    worker_nodes = validate_node_drain(worker_nodes, workload, 600,
                                       max_unavailable)
    for upgraded in worker_nodes:
        upgrade_nodes.append(upgraded)
    nodes = client.list_node(clusterId=cluster.id).data
    assert len(upgrade_nodes) == len(nodes), "Not all Nodes Upgraded"
    for node in nodes:
        assert node["info"]["kubernetes"]["kubeletVersion"] == node_ver, \
            "Not all Nodes Upgraded Correctly"


def validate_node_cordon(nodes, workload, timeout=600):
    client = get_user_client()
    start = time.time()
    in_state = set()
    upgrade_nodes = set()
    while len(upgrade_nodes) != len(nodes):
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for worker nodes to upgrade")
        validate_ingress(namespace["p_client"], namespace["cluster"], [workload], host, path)
        for node in nodes:
            node = client.reload(node)
            node_k8 = node["info"]["kubernetes"]["kubeletVersion"]
            if node.state == "cordoned":
                in_state.add(node.uuid)
            if node_k8 == node_ver:
                upgrade_nodes.add(node.uuid)
        time.sleep(.1)
    assert len(in_state) == len(nodes)
    assert len(upgrade_nodes) == len(nodes)
    return upgrade_nodes


def validate_node_drain(nodes, workload, timeout=600, max_unavailable=1):
    client = get_user_client()
    start = time.time()
    upgrade_nodes = set()
    in_state = set()
    while len(upgrade_nodes) != len(nodes):
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for worker nodes to upgrade")
        for node in nodes:
            node = client.reload(node)
            node_k8 = node["info"]["kubernetes"]["kubeletVersion"]
            if node.state == "draining" or node.state == "drained":
                in_state.add(node.uuid)
            if node_k8 == node_ver:
                upgrade_nodes.add(node.uuid)
        unavailable = set()
        for node in nodes:
            if node.state == "draining":
                unavailable.add(node.uuid)
        assert len(unavailable) <= max_unavailable, "Too many nodes unavailable"
        validate_ingress(namespace["p_client"], namespace["cluster"], [workload], host, path)
        time.sleep(.1)
    assert len(in_state) == len(nodes)
    assert len(upgrade_nodes) == len(nodes)
    return upgrade_nodes


def validate_cluster_and_ingress(client, cluster, intermediate_state="provisioning",
                                 check_intermediate_state=True,
                                 nodes_not_in_active_state=[]):
    time.sleep(5)
    cluster = validate_cluster_state(
        client, cluster,
        check_intermediate_state=check_intermediate_state,
        intermediate_state=intermediate_state,
        nodes_not_in_active_state=nodes_not_in_active_state)
    create_kubeconfig(cluster)
    check_cluster_version(cluster, preupgrade_k8s)
    if hasattr(cluster, 'rancherKubernetesEngineConfig'):
        check_cluster_state(len(get_role_nodes(cluster, "etcd", client)))
    ns = namespace["ns"]
    p_client = namespace["p_client"]
    con = [{"name": "test1",
            "image": TEST_IMAGE}]
    name = random_test_name("default")
    workload = p_client.create_workload(name=name,
                                        containers=con,
                                        namespaceId=ns.id,
                                        daemonSetConfig={})
    validate_workload(p_client, workload, "daemonSet", ns.name,
                      len(get_schedulable_nodes(cluster, client)))
    rule = {"host": host,
            "paths":
                [{"workloadIds": [workload.id], "targetPort": "80"}]}
    ingress = p_client.create_ingress(name=name,
                                      namespaceId=ns.id,
                                      rules=[rule])
    time.sleep(10)
    wait_for_ingress_to_active(p_client, ingress)
    validate_ingress(p_client, cluster, [workload], host, path)
    return cluster, workload, ingress


@pytest.fixture(scope='function', autouse="True")
def create_zdt_setup(request):
    preupgrade_k8s = default_k8s_versions[0]
    zero_node_roles = [["etcd"], ["controlplane"], ["controlplane"], ["worker"], ["worker"]]
    node_roles = [["controlplane"], ["etcd"],
                  ["worker"]]
    client = get_user_client()
    aws_nodes = \
        AmazonWebServices().create_multiple_nodes(
            len(zero_node_roles), random_test_name(HOST_NAME))
    cluster, nodes = create_custom_host_from_nodes(aws_nodes, zero_node_roles,
                                                   random_cluster_name=False,
                                                   version=preupgrade_k8s)
    p, ns = create_project_and_ns(USER_TOKEN, cluster, "testsecret" + str(random_int(10000, 99999)))
    p_client = get_project_client_for_token(p, USER_TOKEN)
    c_client = get_cluster_client_for_token(cluster, USER_TOKEN)
    namespace["p_client"] = p_client
    namespace["ns"] = ns
    namespace["cluster"] = cluster
    namespace["project"] = p
    namespace["c_client"] = c_client
    namespace["nodes"] = aws_nodes.copy()

    def fin():
        client.delete(p)
        cluster_cleanup(client, cluster, aws_nodes)

    request.addfinalizer(fin)
