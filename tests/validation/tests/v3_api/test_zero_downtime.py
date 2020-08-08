from .common import *
import pytest
import math
import time
from .test_rke_cluster_provisioning import create_custom_host_from_nodes

HOST_NAME = os.environ.get('RANCHER_HOST_NAME', "testZeroDT")
DRAIN = os.environ.get('RANCHER_DRAIN_INPUT', "False")
MAX_UNAVAILABLE_WORKER = os.environ.get("RANCHER_MAX_UNAVAILABLE_WORKER", "10%")
MAX_UNAVAILABLE_CONTROLPLANE = os.environ.get("RANCHER_MAX_UNAVAILABLE_CONTROLPLANE", "1")
NODE_COUNT_CLUSTER = os.environ.get("RANCHER_NODE_COUNT_CLUSTER", 3)
K8S_VERSION_PREUPGRADE = os.environ.get("RANCHER_K8S_VERSION_PREUPGRADE", "v1.15.12-rancher2-2")
K8S_VERSION_UPGRADE = os.environ.get("RANCHER_K8S_VERSION_UPGRADE", "v1.16.10-rancher2-1")
host = "test" + str(random_int(10000, 99999)) + ".com"
path = "/name.html"
namespace = {"p_client": None, "ns": None, "cluster": None, "project": None,
             "nodes": []}
backup_info = {"backupname": None, "backup_id": None, "workload": None,
               "backupfilename": None, "etcdbackupdata": None}
service_extra_args = {'etcd': {
    'backupConfig': {'enabled': True, 'intervalHours': 12, 'retention': 6, 's3BackupConfig': None,
                     'safeTimestamp': False, 'type': '/v3/schemas/backupConfig'}, 'creation': '12h',
    'extraArgs': {'election-timeout': '5000', 'heartbeat-interval': '500'},
    'gid': 0, 'retention': '72h', 'snapshot': False, 'type': '/v3/schemas/etcdService', 'uid': 0},
                      'kubeApi': {'alwaysPullImages': False, 'podSecurityPolicy': False,
                                  'serviceNodePortRange': '30000-32767', 'type': '/v3/schemas/kubeAPIService'},
                      'kubeController': {'type': '/v3/schemas/kubeControllerService'},
                      'kubelet': {'extraArgs': {"max-pods": "120"}, 'failSwapOn': False,
                                  'generateServingCertificate': False, 'type': '/v3/schemas/kubeletService'},
                      'kubeproxy': {'type': '/v3/schemas/kubeproxyService'},
                      'scheduler': {'type': '/v3/schemas/schedulerService'}, 'type': '/v3/schemas/rkeConfigServices'}


def test_zero_downtime():
    client, cluster, workload, ingress = get_and_validate_cluster()
    # Update Cluster to k8 version + upgrade strategy maxUnavailable worker
    rke_updated_config = get_default_rke_config(cluster)
    rke_updated_config["services"] = service_extra_args
    rke_updated_config["upgradeStrategy"] = {
        'drain': True,
        'maxUnavailableWorker': MAX_UNAVAILABLE_WORKER,
        'maxUnavailableControlplane': MAX_UNAVAILABLE_CONTROLPLANE,
        'type': '/v3/schemas/nodeUpgradeStrategy'}
    cluster = client.update(cluster,
                            name=cluster.name,
                            rancherKubernetesEngineConfig=rke_updated_config)
    upgraded_nodes = validate_nodes_after_upgrade(cluster, workload, k8version=False)
    nodes = client.list_node(clusterId=cluster.id).data
    assert len(upgraded_nodes) == len(nodes), "Not all Nodes Upgraded"


def test_zero_downtime_drain():
    client, cluster, workload, ingress = get_and_validate_cluster()
    # Update Cluster to k8 version with drain = True
    rke_updated_config = get_default_rke_config(cluster)
    rke_updated_config["kubernetesVersion"] = K8S_VERSION_UPGRADE
    rke_updated_config["upgradeStrategy"] = {
        'drain': True,
        'maxUnavailableWorker': MAX_UNAVAILABLE_WORKER,
        'maxUnavailableControlplane': MAX_UNAVAILABLE_CONTROLPLANE,
        'type': '/v3/schemas/nodeUpgradeStrategy'}
    cluster = client.update(cluster,
                            name=cluster.name,
                            rancherKubernetesEngineConfig=rke_updated_config)
    upgraded_nodes = validate_nodes_after_upgrade(cluster, workload)
    nodes = client.list_node(clusterId=cluster.id).data
    assert len(upgraded_nodes) == len(nodes), "Not all Nodes Upgraded"


def test_zero_downtime_cordon():
    client, cluster, workload, ingress = get_and_validate_cluster()
    # Update Cluster to k8 version with drain = False
    rke_updated_config = get_default_rke_config(cluster)
    rke_updated_config["kubernetesVersion"] = K8S_VERSION_UPGRADE
    rke_updated_config["upgradeStrategy"] = {
        'drain': False,
        'maxUnavailableWorker': MAX_UNAVAILABLE_WORKER,
        'maxUnavailableControlplane': MAX_UNAVAILABLE_CONTROLPLANE,
        'type': '/v3/schemas/nodeUpgradeStrategy'}
    cluster = client.update(cluster,
                            name=cluster.name,
                            rancherKubernetesEngineConfig=rke_updated_config)
    upgraded_nodes = validate_nodes_after_upgrade(cluster, workload)
    nodes = client.list_node(clusterId=cluster.id).data
    assert len(upgraded_nodes) == len(nodes), "Not all Nodes Upgraded"


def test_zero_downtime_backup():
    p_client = namespace["p_client"]
    ns = namespace["ns"]
    client, cluster, workload, ingress = get_and_validate_cluster()
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
    # Update cluster K8 Version
    rke_updated_config = get_default_rke_config(cluster)
    rke_updated_config["kubernetesVersion"] = K8S_VERSION_UPGRADE
    rke_updated_config["upgradeStrategy"] = {
        'drain': True,
        'maxUnavailableWorker': MAX_UNAVAILABLE_WORKER,
        'maxUnavailableControlplane': MAX_UNAVAILABLE_CONTROLPLANE,
        'type': '/v3/schemas/nodeUpgradeStrategy'}
    cluster = client.update(cluster,
                            name=cluster.name,
                            rancherKubernetesEngineConfig=rke_updated_config)
    cluster = validate_cluster_state(client, cluster,
                                     intermediate_state="updating")
    node_ver = K8S_VERSION_UPGRADE.split("-")[0]
    nodes = client.list_node(clusterId=cluster.id).data
    # Validate upgrade
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
    pre_node_ver = K8S_VERSION_PREUPGRADE.split("-")[0]
    wait_for_node_upgrade(nodes, p_client, cluster, workload, pre_node_ver)
    nodes = client.list_node(clusterId=cluster.id).data
    # Verify the ingress created before taking the snapshot
    validate_ingress(p_client, cluster, [backup_info["workload"]], host, path)
    # Verify the workload created after getting a snapshot does not exist after restore
    workload_list = p_client.list_workload(uuid=testworkload.uuid).data
    assert len(workload_list) == 0, "workload shouldn't exist after restore"
    for node in nodes:
        assert node["info"]["kubernetes"]["kubeletVersion"] == pre_node_ver, \
            "Not all Nodes Restored Correctly"


def test_zero_downtime_add_worker():
    client, cluster, workload, ingress = get_and_validate_cluster()
    p_client = namespace["p_client"]
    # Update Cluster to k8 version
    aws_node = \
        AmazonWebServices().create_node(random_test_name(HOST_NAME))
    rke_updated_config = get_default_rke_config(cluster)
    rke_updated_config["kubernetesVersion"] = K8S_VERSION_UPGRADE
    rke_updated_config["upgradeStrategy"] = {
        'drain': True,
        'maxUnavailableWorker': MAX_UNAVAILABLE_WORKER,
        'maxUnavailableControlplane': MAX_UNAVAILABLE_CONTROLPLANE,
        'type': '/v3/schemas/nodeUpgradeStrategy'}
    cluster = client.update(cluster,
                            name=cluster.name,
                            rancherKubernetesEngineConfig=rke_updated_config)
    nodes = client.list_node(clusterId=cluster.id).data
    original = len(nodes)
    # Add worker node
    docker_run_cmd = \
        get_custom_host_registration_cmd(client, cluster, ["worker"],
                                         aws_node)
    aws_node.roles.append("worker")
    result = aws_node.execute_command(docker_run_cmd)
    time.sleep(10)
    cluster = client.reload(cluster)
    nodes = client.list_node(clusterId=cluster.id).data
    # Check Ingress is up during update
    node_ver = K8S_VERSION_UPGRADE.split("-")[0]
    wait_for_node_upgrade(nodes, p_client, cluster, workload, node_ver)
    # Validate update has went through
    assert len(client.list_node(clusterId=cluster.id).data) == original + 1
    for node in nodes:
        node = client.reload(node)
        assert node["info"]["kubernetes"]["kubeletVersion"] == node_ver


def get_and_validate_cluster():
    client = get_user_client()
    cluster = namespace["cluster"]
    cluster, workload, ingress = validate_cluster_and_ingress(
        client, cluster,
        check_intermediate_state=False)
    return client, cluster, workload, ingress


def get_default_rke_config(cluster):
    rke_config = cluster.rancherKubernetesEngineConfig
    rke_updated_config = rke_config.copy()
    return rke_updated_config


def get_max_unavailable(num_nodes):
    if "%" in MAX_UNAVAILABLE_WORKER:
        max_unavailable = \
            math.floor(num_nodes * (int(MAX_UNAVAILABLE_WORKER.split("%")[0]) / 100))
    else:
        max_unavailable = math.floor(int(MAX_UNAVAILABLE_WORKER))
    if max_unavailable == 0:
        return 1
    return max_unavailable


def validate_nodes_after_upgrade(cluster, workload, k8version=False):
    client = get_user_client()
    completed_upgrade = []
    cp_nodes = get_role_nodes(cluster, "control")
    etcd_nodes = get_role_nodes(cluster, "etcd")
    worker_nodes = get_role_nodes(cluster, "worker")
    # Validate Node Upgrades by role
    cp_upgraded = validate_node_state(cp_nodes, workload, k8version)
    completed_upgrade.extend(cp_upgraded)
    etcd_upgraded = validate_node_state(etcd_nodes, workload, k8version)
    completed_upgrade.extend(etcd_upgraded)
    worker_upgraded = validate_node_state(worker_nodes, workload, k8version, DRAIN, 600,
                                          )
    completed_upgrade.extend(worker_upgraded)
    nodes = client.list_node(clusterId=cluster.id).data
    node_ver = K8S_VERSION_UPGRADE.split("-")[0]
    if k8version:
        for node in nodes:
            assert node["info"]["kubernetes"]["kubeletVersion"] == node_ver, \
                "Not all Nodes Upgraded Correctly"
    else:
        for node in nodes:
            assert node["capacity"]["pods"] == "120", \
                "Not all Nodes Upgraded Correctly"
    return completed_upgrade


def validate_node_state(nodes, workload, k8version=False, drain=False, timeout=100,
                        ):
    client = get_user_client()
    start = time.time()
    completed_upgrade = set()
    in_transition_state = set()
    node_ver = K8S_VERSION_UPGRADE.split("-")[0]
    while len(completed_upgrade) != len(nodes):
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for nodes to upgrade")
        unavailable = set()
        for node in nodes:
            # Reload for updated node state and availability
            node = client.reload(node)
            print("node: ", node.uuid)
            print("node state: ", node.state)
            if drain:
                if node.state == "draining" or node.state == "drained":
                    in_transition_state.add(node.uuid)
            else:
                if node.state == "cordoned":
                    in_transition_state.add(node.uuid)
            if k8version:
                node_k8 = node["info"]["kubernetes"]["kubeletVersion"]
                if node_k8 == node_ver:
                    completed_upgrade.add(node.uuid)
            else:
                node_pods = node["capacity"]["pods"]
                print("node pod: ", node_pods)
                if node_pods == 120 or node_pods == "120":
                    print("adding")
                    completed_upgrade.add(node.uuid)
            if node.state != "active":
                unavailable.add(node.uuid)
        if node.worker:
            assert len(unavailable) <= get_max_unavailable(len(nodes)), \
                "Too many nodes unavailable"
        validate_ingress(namespace["p_client"], namespace["cluster"],
                         [workload], host, path)
        time.sleep(.1)
    assert len(in_transition_state) == len(nodes)
    assert len(completed_upgrade) == len(nodes)
    return completed_upgrade


def wait_for_node_upgrade(nodes, p_client, cluster, workload, node_ver, timeout=600):
    client = get_user_client()
    start = time.time()
    completed_upgrade = set()
    while len(completed_upgrade) != len(nodes):
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for K8 update")
        for node in nodes:
            node = client.reload(node)
            node_k8 = node["info"]["kubernetes"]["kubeletVersion"]
            if node_k8 == node_ver:
                completed_upgrade.add(node.uuid)
        validate_ingress(p_client, cluster, [workload], host, path, active_nodes=True)
        time.sleep(5)


def validate_cluster_and_ingress(client, cluster,
                                 intermediate_state="provisioning",
                                 check_intermediate_state=True,
                                 nodes_not_in_active_state=[]):
    time.sleep(5)
    cluster = validate_cluster_state(
        client, cluster,
        check_intermediate_state=check_intermediate_state,
        intermediate_state=intermediate_state,
        nodes_not_in_active_state=nodes_not_in_active_state)
    create_kubeconfig(cluster)
    check_cluster_version(cluster, K8S_VERSION_PREUPGRADE)
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


@pytest.fixture(scope='module', autouse="True")
def create_zdt_setup(request):
    if NODE_COUNT_CLUSTER == 8:
        node_roles = [["etcd"], ["controlplane"], ["controlplane"],
                      ["worker"], ["worker"], ["worker"], ["worker"], ["worker"]]
    elif NODE_COUNT_CLUSTER == 5:
        node_roles = [["controlplane"], ["controlplane"], ["etcd"], ["worker"], ["worker"]]
    elif NODE_COUNT_CLUSTER == 1:
        node_roles = [["worker", "controlplane", "etcd"]]
    elif NODE_COUNT_CLUSTER == 2:
        node_roles = [["controlplane", "etcd", "worker"], ["worker"]]
    else:
        node_roles = [["controlplane"], ["etcd"], ["worker"]]
    client = get_user_client()
    aws_nodes = \
        AmazonWebServices().create_multiple_nodes(
            len(node_roles), random_test_name(HOST_NAME))
    cluster, nodes = create_custom_host_from_nodes(aws_nodes, node_roles,
                                                   random_cluster_name=False,
                                                   version=K8S_VERSION_PREUPGRADE)
    p, ns = create_project_and_ns(USER_TOKEN, cluster, "testsecret"
                                  + str(random_int(10000, 99999)))
    p_client = get_project_client_for_token(p, USER_TOKEN)
    c_client = get_cluster_client_for_token(cluster, USER_TOKEN)
    namespace["p_client"] = p_client
    namespace["ns"] = ns
    namespace["cluster"] = cluster
    namespace["project"] = p
    namespace["c_client"] = c_client
    namespace["nodes"] = aws_nodes

    def fin():
        client.delete(p)
        cluster_cleanup(client, cluster, aws_nodes)

    request.addfinalizer(fin)
