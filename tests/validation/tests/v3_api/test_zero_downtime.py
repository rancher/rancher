from .common import *
import pytest
import time

from .test_rke_cluster_provisioning import create_custom_host_from_nodes

CLUSTER_NAME = "test1"
HOST_NAME = os.environ.get('RANCHER_HOST_NAME', "testZeroDT")
host = "test" + str(random_int(10000, 99999)) + ".com"
path = "/name.html"


@pytest.mark.skip(reason="waiting")
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
    # default_k8s_versions = ['v1.15.12-rancher2-2', 'v1.16.10-rancher2-1', 'v1.17.6-rancher2-1']
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
    node_ver = postupgrade_k8s.split("-")[0]
    # Check Ingress is up during update
    check_upgrade(nodes, client, p_client, cluster, workload, node_ver)
    # Validate update has went through
    for node in nodes:
        uuid = node.uuid
        nodes = client.list_node(uuid=uuid).data
        assert nodes[0]["info"]["kubernetes"]["kubeletVersion"] == node_ver
    # Go through each node for k8 version upgrade and ensure ingress is still up
    validate_ingress(p_client, cluster, [workload], host, path)
    print("Success!")
    delete_node(aws_nodes)
    delete_cluster(client, cluster)


def test_drain_upgrade():
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
    # default_k8s_versions = ['v1.15.12-rancher2-2', 'v1.16.10-rancher2-1', 'v1.17.6-rancher2-1']
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
    node_ver = postupgrade_k8s.split("-")[0]
    # Check Ingress is up during update
    check_node_upgrade(nodes, 1, client, p_client, cluster, workload, node_ver)
    # Validate update has went through
    for node in nodes:
        uuid = node.uuid
        nodes = client.list_node(uuid=uuid).data
        assert nodes[0]["info"]["kubernetes"]["kubeletVersion"] == node_ver
    # Go through each node for k8 version upgrade and ensure ingress is still up
    validate_ingress(p_client, cluster, [workload], host, path)
    print("Success!")
    delete_node(aws_nodes)
    delete_cluster(client, cluster)


def check_upgrade(nodes, client, p_client, cluster, workload, k8_version, timeout=2000):
    start = time.time()
    upgrade_nodes = set()
    print("nodes: ", nodes)
    while len(upgrade_nodes) != len(nodes):
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for K8 update")
        validate_ingress(p_client, cluster, [workload], host, path)
        for node in nodes:
            uuid = node.uuid
            node = client.list_node(uuid=uuid).data
            node_k8 = node[0]["info"]["kubernetes"]["kubeletVersion"]
            if node_k8 == k8_version:
                print("node: ", node, " is at ", node_k8)
                upgrade_nodes.add(uuid)
        time.sleep(10)
    print("upgrade nodes: ", upgrade_nodes)


# drain = true/false?
# next step would be have a func for nodes,
# that checks for node state = Cordoned/Draining during upgrade
# and making sure only maxUnavailable nodes are in those states
# cp: cordoned -> etcd -> worker: draining
def check_node_upgrade(nodes, max_unavailable, client, p_client, cluster, workload, k8_version):
    print("nodes: ", nodes)
    etcd_nodes = get_etcd_nodes(cluster, client)
    cp_nodes = get_cp_nodes(cluster, client)
    worker_nodes = get_worker_nodes(cluster, client)
    upgrade_nodes = []
    print("nodes: ", nodes)
    print("cp nodes: ")
    cp_nodes = validate_node_cordon(cp_nodes, k8_version,client, p_client, cluster, workload)
    upgrade_nodes.append(cp_nodes)
    print("upgrade nodes: ", upgrade_nodes)
    print("etcd: ")
    etcd_nodes = validate_node_cordon(etcd_nodes, k8_version,client, p_client, cluster, workload)
    upgrade_nodes.append(etcd_nodes)
    print("upgrade nodes: ", upgrade_nodes)
    print("worker: ")
    worker_nodes = validate_node_drain(worker_nodes, k8_version, client, p_client, cluster, workload,600,max_unavailable)
    print("upgrade nodes: ", upgrade_nodes)
    upgrade_nodes.append(worker_nodes)
    assert len(upgrade_nodes) == len(nodes), "Not all Nodes Upgraded"


def validate_node_cordon(nodes, k8_version, client, p_client, cluster, workload, timeout=600):
    start = time.time()
    in_state = set()
    upgraded_nodes = set()
    for node in nodes:
        node_k8 = node["info"]["kubernetes"]["kubeletVersion"]
        print("node: ", node.uuid)
        print("node version: ", node_k8)
        print("node state: ", node.state)
        while not(node_k8 == k8_version and node.state == "active"):
            print("node version: ", node_k8)
            print("node state: ", node.state)
            if time.time() - start > timeout:
                raise AssertionError(
                    "Timed out waiting for node upgrade")
            validate_ingress(p_client, cluster, [workload], host, path)
            if node.state == "cordoned":
                print("node state: ", node.state)
                in_state.add(node.uuid)
                print("added to in-state")
            time.sleep(1)
            node = client.reload(node)
            node_k8 = node["info"]["kubernetes"]["kubeletVersion"]
        if node_k8 == k8_version:
            print("node: ", node, " is at ", node_k8)
            upgraded_nodes.add(node.uuid)
            print("added to upgrade nodes")
        print("node: ", node)
        print(node_k8)
    print("final instate: ", in_state)
    print("final upgrade: ", upgraded_nodes)
    assert len(in_state) == len(nodes), "nodes failed to achieve desired state"
    return upgraded_nodes


def validate_node_drain(nodes, k8_version, client, p_client, cluster, workload, timeout=600, max_unavailable=1):
    start = time.time()
    upgrade_nodes = set()
    print("nodes: ", nodes)
    while len(upgrade_nodes) != len(nodes):
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for worker nodes to upgrade")
        validate_ingress(p_client, cluster, [workload], host, path)
        for node in nodes:
            uuid = node.uuid
            node = client.list_node(uuid=uuid).data
            node_k8 = node[0]["info"]["kubernetes"]["kubeletVersion"]
            if node_k8 == k8_version:
                print("node: ", node, " is at ", node_k8)
                upgrade_nodes.add(uuid)
        unavailable = set()
        for node in nodes:
            if node.state == "draining":
                unavailable.add(node.uuid)
        assert len(unavailable) <= max_unavailable, "Too many nodes unavailable"
        time.sleep(10)
    return upgrade_nodes


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
    rule = {"host": host,
            "paths":
                [{"workloadIds": [workload.id], "targetPort": "80"}]}
    ingress = p_client.create_ingress(name=name,
                                      namespaceId=ns.id,
                                      rules=[rule])
    wait_for_ingress_to_active(p_client, ingress)
    validate_ingress(p_client, cluster, [workload], host, path)
    return cluster, workload, ingress, p_client
