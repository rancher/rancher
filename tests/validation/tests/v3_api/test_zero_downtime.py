from .common import *
import pytest
import time
from .test_rke_cluster_provisioning import create_custom_host_from_nodes

HOST_NAME = os.environ.get('RANCHER_HOST_NAME', "testZeroDT")
MAX_UNAVAILABLE = os.environ.get("MAX_UNAVAILABLE", "1")
host = "test" + str(random_int(10000, 99999)) + ".com"
path = "/name.html"
namespace = {"p_client": None, "ns": None, "cluster": None, "project": None,
             "nodes": []}
backup_info = {"backupname": None, "backup_id": None, "workload": None,
               "backupfilename": None, "etcdbackupdata": None}
default_k8s_versions = ['v1.15.12-rancher2-2', 'v1.16.10-rancher2-1',
                        'v1.17.6-rancher2-1']
preupgrade_k8s = default_k8s_versions[0]
postupgrade_k8s = default_k8s_versions[1]
node_ver = postupgrade_k8s.split("-")[0]
pre_node_ver = preupgrade_k8s.split("-")[0]


@pytest.mark.skip(reason="tested")
def test_zdt():
    client = get_user_client()
    cluster = namespace["cluster"]
    p_client = namespace["p_client"]
    cluster, workload, ingress = validate_cluster_and_ingress(
        client, cluster,
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
    wait_for_node_upgrade(nodes, p_client, cluster, workload, node_ver)
    # Validate update has went through
    for node in nodes:
        node = client.reload(node)
        assert node["info"]["kubernetes"]["kubeletVersion"] == node_ver


def test_zdt_nodes():
    client = get_user_client()
    drain = True
    cluster = namespace["cluster"]
    cluster, workload, ingress = validate_cluster_and_ingress(
        client, cluster,
        check_intermediate_state=False)
    # Update Cluster to k8 version + upgrade strategy maxUnavailable worker
    cluster = client.update_by_id_cluster(
        id=cluster.id,
        name="test1",
        rancherKubernetesEngineConfig={
            "kubernetesVersion": postupgrade_k8s,
            "upgradeStrategy": {
                'drain': drain,
                'maxUnavailableWorker': MAX_UNAVAILABLE,
                'type': '/v3/schemas/nodeUpgradeStrategy'}}
    )
    node_ver = postupgrade_k8s.split("-")[0]
    upgrade_nodes = []
    cp_nodes = get_cp_nodes(cluster, client)
    etcd_nodes = get_etcd_nodes(cluster, client)
    worker_nodes = get_worker_nodes(cluster, client)
    # Validate Node Upgrades by job
    cp_upgraded = validate_node_cordon(cp_nodes, workload)
    for upgraded in cp_upgraded:
        upgrade_nodes.append(upgraded)
    etcd_upgraded = validate_node_cordon(etcd_nodes, workload)
    for upgraded in etcd_upgraded:
        upgrade_nodes.append(upgraded)
    if "%" in MAX_UNAVAILABLE:
        max_unavailable = len(worker_nodes) \
                          * int(MAX_UNAVAILABLE.split("%")[0]) / 100
    else:
        max_unavailable = int(MAX_UNAVAILABLE)

    worker_upgraded = validate_node_drain(worker_nodes, workload, True, 600,
                                          max_unavailable)
    for upgraded in worker_upgraded:
        upgrade_nodes.append(upgraded)
    nodes = client.list_node(clusterId=cluster.id).data
    assert len(upgrade_nodes) == len(nodes), "Not all Nodes Upgraded"
    for node in nodes:
        assert node["info"]["kubernetes"]["kubeletVersion"] == node_ver, \
            "Not all Nodes Upgraded Correctly"


# copy test -> cluster update -> pick 1/2 nodes stop them (aws.py)
# -> cluster upgrade does not succeed -> rollback
def test_zdt_backup():
    client = get_user_client()
    p_client = namespace["p_client"]
    ns = namespace["ns"]
    cluster = namespace["cluster"]
    nodes = client.list_node(clusterId=cluster.id).data
    cluster, workload, ingress = validate_cluster_and_ingress(
        client, cluster,
        check_intermediate_state=False)
    backup = cluster.backupEtcd()
    backup_info["backupname"] = backup['metadata']['name']
    wait_for_backup_to_active(cluster, backup_info["backupname"])
    # Get all the backup info
    etcdbackups = cluster.etcdBackups(name=backup_info["backupname"])
    backup_info["etcdbackupdata"] = etcdbackups['data']
    backup_info["backup_id"] = backup_info["etcdbackupdata"][0]['id']
    backup_info["workload"] = workload
    # Create workload after backup
    name = random_test_name("default")
    con = [{"name": "test1",
            "image": TEST_IMAGE}]
    testworkload = p_client.create_workload(name=name,
                                            containers=con,
                                            namespaceId=ns.id)

    validate_workload(p_client, testworkload, "deployment", ns.name)
    # update cluster
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
    cluster = validate_cluster_state(client, cluster,
                                     intermediate_state="updating")
    wait_for_node_upgrade(nodes, p_client, cluster, workload, node_ver)
    for node in nodes:
        node = client.reload(node)
        assert node["info"]["kubernetes"]["kubeletVersion"] == node_ver, \
            "Not all Nodes Upgraded Correctly"
    # Perform Full Restore
    cluster.restoreFromEtcdBackup(etcdBackupId=backup_info["backup_id"],
                                  restoreRkeConfig="all")
    cluster = client.reload(cluster)
    cluster = validate_cluster_state(
        client, cluster,
        check_intermediate_state=True,
        intermediate_state="updating",
    )
    wait_for_node_upgrade(nodes, p_client, cluster, workload, pre_node_ver)
    # Verify the ingress created before taking the snapshot
    validate_ingress(p_client, cluster, [backup_info["workload"]], host, path)
    # Verify the workload created after getting a snapshot does not exist after restore
    workload_list = p_client.list_workload(uuid=testworkload.uuid).data
    assert len(workload_list) == 0, "workload shouldn't exist after restore"
    for node in nodes:
        assert node["info"]["kubernetes"]["kubeletVersion"] == pre_node_ver, \
            "Not all Nodes Restored Correctly"


def test_zdt_add_worker():
    client = get_user_client()
    cluster = namespace["cluster"]
    p_client = namespace["p_client"]
    cluster, workload, ingress = validate_cluster_and_ingress(
        client, cluster,
        check_intermediate_state=False)
    # Update Cluster to k8 version + upgrade strategy maxUnavailable worker
    aws_node = \
        AmazonWebServices().create_node(random_test_name(HOST_NAME))
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
    original = len(nodes)
    docker_run_cmd = \
        get_custom_host_registration_cmd(client, cluster, ["worker"],
                                         aws_node)
    aws_node.roles.append("worker")
    result = aws_node.execute_command(docker_run_cmd)
    time.sleep(10)
    cluster = client.reload(cluster)
    nodes = client.list_node(clusterId=cluster.id).data
    # Check Ingress is up during update
    wait_for_node_upgrade(nodes, p_client, cluster, workload, node_ver)
    # Validate update has went through
    assert len(client.list_node(clusterId=cluster.id).data) == original + 1
    for node in nodes:
        node = client.reload(node)
        assert node["info"]["kubernetes"]["kubeletVersion"] == node_ver


def validate_node_cordon(nodes, workload, timeout=600):
    client = get_user_client()
    start = time.time()
    in_state = set()
    upgrade_nodes = set()
    while len(upgrade_nodes) != len(nodes):
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for worker nodes to upgrade")
        cluster = namespace["cluster"]
        cluster = client.reload(cluster)
        validate_ingress(namespace["p_client"], cluster,
                         [workload], host, path)
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


def validate_node_drain(nodes, workload, drain=False, timeout=600,
                        max_unavailable=1):
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
            if drain:
                if node.state == "draining" or node.state == "drained":
                    in_state.add(node.uuid)
            else:
                if node.state == "cordoned":
                    in_state.add(node.uuid)
            if node_k8 == node_ver:
                upgrade_nodes.add(node.uuid)
        unavailable = set()
        for node in nodes:
            if node.state != "active":
                unavailable.add(node.uuid)
        assert len(unavailable) <= max_unavailable, \
            "Too many nodes unavailable"
        validate_ingress(namespace["p_client"], namespace["cluster"],
                         [workload], host, path)
        time.sleep(.1)
    assert len(in_state) == len(nodes)
    assert len(upgrade_nodes) == len(nodes)
    return upgrade_nodes


def wait_for_node_upgrade(nodes, p_client, cluster, workload, node_ver, timeout=600):
    client = get_user_client()
    start = time.time()
    upgrade_nodes = set()
    while len(upgrade_nodes) != len(nodes):
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for K8 update")
        for node in nodes:
            node = client.reload(node)
            node_k8 = node["info"]["kubernetes"]["kubeletVersion"]
            if node_k8 == node_ver:
                upgrade_nodes.add(node.uuid)
        validate_ingress(p_client, cluster, [workload], host, path)
        time.sleep(5)


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


def validate_ingress(p_client, cluster, workloads, host, path,
                     insecure_redirect=False):
    time.sleep(10)
    curl_args = " "
    if (insecure_redirect):
        curl_args = " -L --insecure "
    if len(host) > 0:
        curl_args += " --header 'Host: " + host + "'"
    nodes = get_schedulable_active_nodes(cluster, os_type="linux")
    target_name_list = get_target_names(p_client, workloads)
    for node in nodes:
        host_ip = resolve_node_ip(node)
        url = "http://" + host_ip + path
        if not insecure_redirect:
            wait_until_ok(url, timeout=300, headers={
                "Host": host
            })
        cmd = curl_args + " " + url
        validate_http_response(cmd, target_name_list)


def get_schedulable_active_nodes(cluster, client=None, os_type=TEST_OS):
    if not client:
        client = get_user_client()
    nodes = client.list_node(clusterId=cluster.id).data
    schedulable_nodes = []
    for node in nodes:
        if node.worker and (not node.unschedulable) and node.state == "active":
            for key, val in node.labels.items():
                # Either one of the labels should be present on the node
                if key == 'kubernetes.io/os' or key == 'beta.kubernetes.io/os':
                    if val == os_type:
                        schedulable_nodes.append(node)
                        break
        # Including master in list of nodes as master is also schedulable
        if 'k3s' in cluster.version["gitVersion"] and node.controlPlane:
            schedulable_nodes.append(node)
    return schedulable_nodes


@pytest.fixture(scope='function', autouse="True")
def create_zdt_setup(request):
    preupgrade_k8s = default_k8s_versions[0]
    # zero_node_roles = [["etcd"], ["controlplane"], ["controlplane"],
    # ["worker"], ["worker"]]
    zero_node_roles = [["controlplane"], ["etcd"], ["worker"]]
    client = get_user_client()
    aws_nodes = \
        AmazonWebServices().create_multiple_nodes(
            len(zero_node_roles), random_test_name(HOST_NAME))
    cluster, nodes = create_custom_host_from_nodes(aws_nodes, zero_node_roles,
                                                   random_cluster_name=False,
                                                   version=preupgrade_k8s)
    p, ns = create_project_and_ns(USER_TOKEN, cluster, "testsecret"
                                  + str(random_int(10000, 99999)))
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
