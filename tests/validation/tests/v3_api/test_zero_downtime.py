from .common import *
import pytest
import time

from .test_rke_cluster_provisioning import create_and_validate_custom_host, create_custom_host_from_nodes

CLUSTER_NAME = "test1"
HOST_NAME = os.environ.get('RANCHER_HOST_NAME', "testZeroDT")
host = "test" + str(random_int(10000, 99999)) + ".com"
path = "/name.html"


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
    #default_k8s_versions = ['v1.15.12-rancher2-2', 'v1.16.10-rancher2-1', 'v1.17.6-rancher2-1']
    # Create cluster
    preupgrade_k8s = default_k8s_versions[0]
    postupgrade_k8s = default_k8s_versions[1]
    zero_node_roles = [["etcd"], ["etcd"], ["controlplane"], ["controlplane"],
                       ["etcd"], ["worker"], ["worker"]]
    node_roles = [["etcd"], ["controlplane"], ["worker"]]
    aws_nodes = \
        AmazonWebServices().create_multiple_nodes(
            len(node_roles), random_test_name(HOST_NAME))
    cluster, nodes = create_custom_host_from_nodes(aws_nodes, node_roles,
                                                   random_cluster_name=False,
                                                   version=preupgrade_k8s)
    print("cluster: ", cluster)
    cluster, workload, ingress, p_client = validate_cluster_and_ingress(client, cluster,
                                                              check_intermediate_state=False,
                                                              k8s_version=preupgrade_k8s)
    #Update Cluster to k8 version + upgrade strategy maxUnavailable worker
    cluster = client.update_by_id_cluster(
        id=cluster.id,
        name="test1",
        rancherKubernetesEngineConfig={
            "kubernetesVersion": postupgrade_k8s,
            "upgradeStrategy": {
                'drain': False,
                'maxUnavailableControlplane': '1',
                'maxUnavailableWorker': '20%',
                'type': '/v3/schemas/nodeUpgradeStrategy'}}
    )
    nodes = client.list_node(clusterId=cluster.id).data
    #Go through each node for k8 version upgrade and ensure ingress is still up
    for node in nodes:
        wait_for_node_status(client, node, "active")
        print("node: ", node)
        node_ver = postupgrade_k8s.split("-")[0]
        wait_for_kubelet_version(node, client, node_ver)
        validate_ingress(p_client, cluster, [workload], host, path)
    # wait_for_k8_upgrade(cluster, cluster.id, client, postupgrade_k8s)
    check_cluster_version(cluster, postupgrade_k8s)
    print("Success!")


def wait_for_kubelet_version(node, client, k8_Version, timeout=2000):
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


def wait_for_k8_upgrade(cluster, cluster_id, client, k8_Version, timeout=1800):
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


def validate_cluster_and_ingress(client, cluster, intermediate_state="provisioning",
                                 check_intermediate_state=True,
                                 nodes_not_in_active_state=[], k8s_version="",
                                 userToken=USER_TOKEN):
    time.sleep(5)
    cluster = validate_cluster_state(
        client, cluster,
        check_intermediate_state=check_intermediate_state,
        intermediate_state=intermediate_state,
        nodes_not_in_active_state=nodes_not_in_active_state)
    print("cluster: ", cluster)
    create_kubeconfig(cluster)
    if k8s_version != "":
        check_cluster_version(cluster, k8s_version)
    if hasattr(cluster, 'rancherKubernetesEngineConfig'):
        check_cluster_state(len(get_role_nodes(cluster, "etcd", client)))
    project, ns = create_project_and_ns(userToken, cluster)
    p_client = get_project_client_for_token(project, userToken)
    con = [{"name": "test1",
            "image": TEST_IMAGE}]
    name = random_test_name("default")
    workload = p_client.create_workload(name=name,
                                        containers=con,
                                        namespaceId=ns.id,
                                        daemonSetConfig={})
    validate_workload(p_client, workload, "daemonSet", ns.name,
                      len(get_schedulable_nodes(cluster, client)))
    pods = p_client.list_pod(workloadId=workload["id"]).data
    print("cluster: ", cluster)
    print("pods: ", pods)
    print("workload: ", workload)
    scale = len(pods)
    rule = {"host": host,
            "paths":
                [{"workloadIds": [workload.id], "targetPort": "80"}]}
    ingress = p_client.create_ingress(name=name,
                                      namespaceId=ns.id,
                                      rules=[rule])
    wait_for_ingress_to_active(p_client, ingress)
    validate_ingress(p_client, cluster, [workload], host, path)
    return cluster, workload, ingress, p_client
