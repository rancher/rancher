from .common import *
import pytest
import time

from .test_rke_cluster_provisioning import create_and_validate_custom_host

# def validate_cluster(...):
# check_cluster_version(cluster, version):
# def cluster_template_create_edit(userToken):
# def check_cluster_version(cluster, version):
# node = wait_for_node_status(client, node, "active")
# @pytest.fixture(scope='module', autouse="True")

CLUSTER_NAME = "test1"


def test_cluster_upgrade():
    print("Deploying RKE Clusters")
    client = get_user_client()
    rancher_version = get_setting_value_by_name('server-version')
    print("rancher version:", rancher_version)
    if str(rancher_version).startswith('v2.2'):
        k8s_v = get_setting_value_by_name('k8s-version-to-images')
        default_k8s_versions = json.loads(k8s_v).keys()
    else:
        k8s_v = get_setting_value_by_name('k8s-versions-current')
        default_k8s_versions = k8s_v.split(",")
    # print(default_k8s_versions) #['v1.15.12-rancher2-2', 'v1.16.10-rancher2-1', 'v1.17.6-rancher2-1']
    # Create clusters
    preupgrade_k8s = default_k8s_versions[0]
    postupgrade_k8s = default_k8s_versions[1]
    zero_node_roles = [["etcd"], ["etcd"], ["controlplane"], ["controlplane"],
                       ["etcd"], ["worker"], ["worker"]]
    node_roles = [["etcd"], ["controlplane"], ["worker"]]
    cluster, aws_nodes = create_and_validate_custom_host(
        node_roles, random_cluster_name=False, version=preupgrade_k8s)
    cluster_id = cluster["id"]
    print(cluster["id"])
    cluster = client.by_id_cluster(cluster_id)
    cluster = client.update_by_id_cluster(
        id=cluster.id,
        name="test1",
        rancherKubernetesEngineConfig={
            "kubernetesVersion": postupgrade_k8s,
            "upgradeStrategy": {
                'drain': False,
                'maxUnavailableControlplane': '2',
                'maxUnavailableWorker': '10%',
                'type': '/v3/schemas/nodeUpgradeStrategy'}}
    )
    nodes = client.list_node(clusterId=cluster.id).data
    for node in nodes:
        wait_for_node_status(client, node, "active")
        print("node: ", node)
        node_ver = postupgrade_k8s.split("-")[0]
        wait_for_kubelet_version(node, client, node_ver)
    wait_for_k8_upgrade(cluster, cluster_id, client, postupgrade_k8s)
    check_cluster_version(cluster, postupgrade_k8s)
    print("Success!")


def wait_for_kubelet_version(node, client, k8_Version, timeout=1200):
    uuid = node.uuid
    start = time.time()
    nodes = client.list_node(uuid=uuid).data
    print("nodes: ", nodes)
    node_k8 = nodes[0]["info"]["kubernetes"]["kubeletVersion"]
    while node_k8 != k8_Version:
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for K8 update")
        time.sleep(60)
        nodes = client.list_node(uuid=uuid).data
        node_k8 = nodes[0]["info"]["kubernetes"]["kubeletVersion"]
        print("Node ver: ", node_k8)
    assert nodes[0]["info"]["kubernetes"]["kubeletVersion"] == k8_Version


def wait_for_k8_upgrade(cluster, cluster_id, client, k8_Version, timeout=1200):
    cluster_k8s_version = \
        cluster.appliedSpec["rancherKubernetesEngineConfig"][
            "kubernetesVersion"]
    start = time.time()
    while cluster_k8s_version != k8_Version:
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for K8 update")
        time.sleep(60)
        cluster = client.by_id_cluster(cluster_id)
        print("Cluster_Ver: ", cluster_k8s_version)
    assert cluster_k8s_version == k8_Version, \
        "cluster_k8s_version: " + cluster_k8s_version + \
        " Expected: " + k8_Version
