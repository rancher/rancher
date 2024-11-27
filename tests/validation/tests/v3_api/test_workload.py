import pytest

from .common import *  # NOQA
from rancher import ApiError

namespace = {"p_client": None, "ns": None, "cluster": None, "project": None}
RBAC_ROLES = [CLUSTER_OWNER, PROJECT_MEMBER, PROJECT_OWNER,
              PROJECT_READ_ONLY, CLUSTER_MEMBER]
WORKLOAD_TYPES = ["daemonSet", "statefulSet", "cronJob", "job"]

if_check_lb = os.environ.get('RANCHER_CHECK_FOR_LB', "False")
if_check_lb = pytest.mark.skipif(
    if_check_lb != "True",
    reason='Lb test case skipped')

ENABLE_HOST_NODE_PORT_TESTS = ast.literal_eval(
    os.environ.get('RANCHER_ENABLE_HOST_NODE_PORT_TESTS', "True"))
skip_host_node_port = pytest.mark.skipif(
    not ENABLE_HOST_NODE_PORT_TESTS,
    reason='Tests Skipped for AKS,GKE,EKS Clusters')

#Converted to go test in TestWorkloadSideKick
def test_wl_sidekick():
    p_client = namespace["p_client"]
    ns = namespace["ns"]
    con = [{"name": "test1",
            "image": TEST_IMAGE}]
    name = random_test_name("sidekick")
    workload = p_client.create_workload(name=name,
                                        containers=con,
                                        namespaceId=ns.id)
    validate_workload(p_client, workload, "deployment", ns.name)

    side_con = {"name": "test2",
                "image": TEST_IMAGE_REDIS,
                "stdin": True,
                "tty": True}
    con.append(side_con)
    workload = p_client.update(workload,
                               containers=con)
    time.sleep(90)
    validate_workload_with_sidekicks(
        p_client, workload, "deployment", ns.name)

#Converted to go test in TestWorkloadDeployment
def test_wl_deployment():
    p_client = namespace["p_client"]
    ns = namespace["ns"]
    con = [{"name": "test1",
            "image": TEST_IMAGE}]
    name = random_test_name("default")
    workload = p_client.create_workload(name=name,
                                        containers=con,
                                        namespaceId=ns.id)
    validate_workload(p_client, workload, "deployment", ns.name)

#Converted to go test in TestWorkloadStatefulset
def test_wl_statefulset():
    p_client = namespace["p_client"]
    ns = namespace["ns"]
    con = [{"name": "test1",
            "image": TEST_IMAGE}]
    name = random_test_name("default")
    workload = p_client.create_workload(name=name,
                                        containers=con,
                                        namespaceId=ns.id,
                                        statefulSetConfig={}
                                        )
    validate_workload(p_client, workload, "statefulSet", ns.name)

#Converted to go test in TestWorkloadDaemonSet
def test_wl_daemonset():
    p_client = namespace["p_client"]
    ns = namespace["ns"]
    cluster = namespace["cluster"]
    con = [{"name": "test1",
            "image": TEST_IMAGE}]
    name = random_test_name("default")
    workload = p_client.create_workload(name=name,
                                        containers=con,
                                        namespaceId=ns.id,
                                        daemonSetConfig={})
    schedulable_node_count = len(get_schedulable_nodes(cluster))
    validate_workload(p_client, workload, "daemonSet",
                      ns.name, schedulable_node_count)

#Converted to go test in TestWorkloadCronjob
def test_wl_cronjob():
    p_client = namespace["p_client"]
    ns = namespace["ns"]
    con = [{"name": "test1",
            "image": TEST_IMAGE}]
    name = random_test_name("default")
    workload = p_client.create_workload(name=name,
                                        containers=con,
                                        namespaceId=ns.id,
                                        cronJobConfig={
                                            "concurrencyPolicy": "Allow",
                                            "failedJobsHistoryLimit": 10,
                                            "schedule": "*/1 * * * *",
                                            "successfulJobsHistoryLimit": 10})
    validate_workload(p_client, workload, "cronJob", ns.name)

#Converted to go test in TestDeploymentUpgrade
def test_wl_upgrade():
    p_client = namespace["p_client"]
    ns = namespace["ns"]
    con = [{"name": "test1",
            "image": TEST_IMAGE}]
    name = random_test_name("default")
    workload = p_client.create_workload(name=name,
                                        containers=con,
                                        namespaceId=ns.id,
                                        scale=2)
    wait_for_pods_in_workload(p_client, workload, 2)
    validate_workload(p_client, workload, "deployment", ns.name, 2)
    revisions = workload.revisions()
    assert len(revisions) == 1
    for revision in revisions:
        if revision["containers"][0]["image"] == TEST_IMAGE:
            firstrevision = revision.id

    con = [{"name": "test1",
            "image": TEST_IMAGE_REDIS}]
    p_client.update(workload, containers=con)
    wait_for_pod_images(p_client, workload, ns.name, TEST_IMAGE_REDIS, 2)
    wait_for_pods_in_workload(p_client, workload, 2)
    validate_workload(p_client, workload, "deployment", ns.name, 2)
    validate_workload_image(p_client, workload, TEST_IMAGE_REDIS, ns)
    revisions = workload.revisions()
    assert len(revisions) == 2
    for revision in revisions:
        if revision["containers"][0]["image"] == TEST_IMAGE_REDIS:
            secondrevision = revision.id

    con = [{"name": "test1",
            "image": TEST_IMAGE_OS_BASE,
            "tty": True,
            "stdin": True}]
    p_client.update(workload, containers=con)
    wait_for_pod_images(p_client, workload, ns.name, TEST_IMAGE_OS_BASE, 2)
    wait_for_pods_in_workload(p_client, workload, 2)
    validate_workload(p_client, workload, "deployment", ns.name, 2)
    validate_workload_image(p_client, workload, TEST_IMAGE_OS_BASE, ns)
    revisions = workload.revisions()
    assert len(revisions) == 3
    for revision in revisions:
        if revision["containers"][0]["image"] == TEST_IMAGE_OS_BASE:
            thirdrevision = revision.id

    p_client.action(workload, "rollback", replicaSetId=firstrevision)
    wait_for_pod_images(p_client, workload, ns.name, TEST_IMAGE, 2)
    wait_for_pods_in_workload(p_client, workload, 2)
    validate_workload(p_client, workload, "deployment", ns.name, 2)
    validate_workload_image(p_client, workload, TEST_IMAGE, ns)

    p_client.action(workload, "rollback", replicaSetId=secondrevision)
    wait_for_pod_images(p_client, workload, ns.name, TEST_IMAGE_REDIS, 2)
    wait_for_pods_in_workload(p_client, workload, 2)
    validate_workload(p_client, workload, "deployment", ns.name, 2)
    validate_workload_image(p_client, workload, TEST_IMAGE_REDIS, ns)

    p_client.action(workload, "rollback", replicaSetId=thirdrevision)
    wait_for_pod_images(p_client, workload, ns.name, TEST_IMAGE_OS_BASE, 2)
    wait_for_pods_in_workload(p_client, workload, 2)
    validate_workload(p_client, workload, "deployment", ns.name, 2)
    validate_workload_image(p_client, workload, TEST_IMAGE_OS_BASE, ns)

#Converted to go test in TestDeploymentPodScaleUp
def test_wl_pod_scale_up():
    p_client = namespace["p_client"]
    ns = namespace["ns"]
    con = [{"name": "test1",
            "image": TEST_IMAGE}]
    name = random_test_name("default")
    workload = p_client.create_workload(name=name,
                                        containers=con,
                                        namespaceId=ns.id)
    workload = wait_for_wl_to_active(p_client, workload)
    for key, value in workload.workloadLabels.items():
        label = key + "=" + value
    get_pods = "get pods -l" + label + " -n " + ns.name
    allpods = execute_kubectl_cmd(get_pods)
    wait_for_pods_in_workload(p_client, workload, 1)

    p_client.update(workload, scale=2, containers=con)
    validate_workload(p_client, workload, "deployment", ns.name, 2)
    validate_pods_are_running_by_id(allpods, workload, ns.name)

    for key, value in workload.workloadLabels.items():
        label = key + "=" + value
    allpods = execute_kubectl_cmd(get_pods)
    wait_for_pods_in_workload(p_client, workload, 2)
    p_client.update(workload, scale=3, containers=con)
    validate_workload(p_client, workload, "deployment", ns.name, 3)
    validate_pods_are_running_by_id(allpods, workload, ns.name)

#Converted to go test in TestDeploymentPodScaleDown
def test_wl_pod_scale_down():
    p_client = namespace["p_client"]
    ns = namespace["ns"]
    con = [{"name": "test1",
            "image": TEST_IMAGE}]
    name = random_test_name("default")
    workload = p_client.create_workload(name=name,
                                        containers=con,
                                        namespaceId=ns.id,
                                        scale=3)
    wait_for_wl_to_active(p_client, workload)
    wait_for_pods_in_workload(p_client, workload, 3)

    p_client.update(workload, scale=2, containers=con)
    wait_for_pods_in_workload(p_client, workload, 2)
    for key, value in workload.workloadLabels.items():
        label = key + "=" + value
    get_pods = "get pods -l" + label + " -n " + ns.name
    allpods = execute_kubectl_cmd(get_pods)
    validate_workload(p_client, workload, "deployment", ns.name, 2)
    validate_pods_are_running_by_id(allpods, workload, ns.name)

    p_client.update(workload, scale=1, containers=con)
    wait_for_pods_in_workload(p_client, workload, 1)
    for key, value in workload.workloadLabels.items():
        label = key + "=" + value
    allpods = execute_kubectl_cmd(get_pods)
    validate_workload(p_client, workload, "deployment", ns.name)
    validate_pods_are_running_by_id(allpods, workload, ns.name)

#Converted to go test in TestDeploymentPauseOrchestration
def test_wl_pause_orchestration():
    p_client = namespace["p_client"]
    ns = namespace["ns"]
    con = [{"name": "test1",
            "image": TEST_IMAGE}]
    name = random_test_name("default")
    workload = p_client.create_workload(name=name,
                                        containers=con,
                                        namespaceId=ns.id,
                                        scale=2)
    workload = wait_for_wl_to_active(p_client, workload)
    wait_for_pods_in_workload(p_client, workload, 2)
    p_client.action(workload, "pause")
    validate_workload_paused(p_client, workload, True)
    con = [{"name": "test1",
            "image": TEST_IMAGE_REDIS}]
    p_client.update(workload, containers=con)
    validate_pod_images(TEST_IMAGE, workload, ns.name)
    p_client.action(workload, "resume")
    workload = wait_for_wl_to_active(p_client, workload)
    wait_for_pods_in_workload(p_client, workload, 2)
    validate_workload_paused(p_client, workload, False)
    validate_pod_images(TEST_IMAGE_REDIS, workload, ns.name)


# Windows could not support host port for now.
#Converted to go test in TestHostPort
@skip_test_windows_os
@skip_host_node_port
def test_wl_with_hostPort():
    p_client = namespace["p_client"]
    ns = namespace["ns"]
    source_port = 9999
    port = {"containerPort": TEST_IMAGE_PORT,
            "type": "containerPort",
            "kind": "HostPort",
            "protocol": "TCP",
            "sourcePort": source_port}
    con = [{"name": "test1",
            "image": TEST_IMAGE,
            "ports": [port]}]
    name = random_test_name("default")

    workload = p_client.create_workload(name=name,
                                        containers=con,
                                        namespaceId=ns.id,
                                        daemonSetConfig={})
    workload = wait_for_wl_to_active(p_client, workload)
    validate_hostPort(p_client, workload, source_port, namespace["cluster"])

#Converted to go test in TestNodePort
@skip_host_node_port
def test_wl_with_nodePort():
    p_client = namespace["p_client"]
    ns = namespace["ns"]
    source_port = 30456
    port = {"containerPort": TEST_IMAGE_PORT,
            "type": "containerPort",
            "kind": "NodePort",
            "protocol": "TCP",
            "sourcePort": source_port}
    con = [{"name": "test1",
            "image": TEST_IMAGE,
            "ports": [port]}]
    name = random_test_name("default")

    workload = p_client.create_workload(name=name,
                                        containers=con,
                                        namespaceId=ns.id,
                                        daemonSetConfig={})

    workload = wait_for_wl_to_active(p_client, workload)
    validate_nodePort(p_client, workload, namespace["cluster"], source_port)

#Converted to go test in TestClusterIP
def test_wl_with_clusterIp():
    p_client = namespace["p_client"]
    ns = namespace["ns"]
    source_port = 30458
    port = {"containerPort": TEST_IMAGE_PORT,
            "type": "containerPort",
            "kind": "ClusterIP",
            "protocol": "TCP",
            "sourcePort": source_port}
    con = [{"name": "test1",
            "image": TEST_IMAGE,
            "ports": [port]}]
    name = random_test_name("default")

    workload = p_client.create_workload(name=name,
                                        containers=con,
                                        namespaceId=ns.id,
                                        daemonSetConfig={})
    workload = wait_for_wl_to_active(p_client, workload)

    # Get cluster Ip
    sd_records = p_client.list_dns_record(name=name).data
    assert len(sd_records) == 1
    cluster_ip = sd_records[0].clusterIp

    # Deploy test pods used for clusteIp resolution check
    wlname = random_test_name("testclusterip-client")
    con = [{"name": "test1",
            "image": TEST_IMAGE}]

    workload_for_test = p_client.create_workload(name=wlname,
                                                 containers=con,
                                                 namespaceId=ns.id,
                                                 scale=2)
    wait_for_wl_to_active(p_client, workload_for_test)
    test_pods = wait_for_pods_in_workload(p_client, workload_for_test, 2)
    validate_clusterIp(p_client, workload, cluster_ip, test_pods, source_port)

#Converted to go test in TestLoadBalance
@if_check_lb
def test_wl_with_lb():
    p_client = namespace["p_client"]
    ns = namespace["ns"]
    source_port = 9001
    port = {"containerPort": TEST_IMAGE_PORT,
            "type": "containerPort",
            "kind": "LoadBalancer",
            "protocol": "TCP",
            "sourcePort": source_port}
    con = [{"name": "test1",
            "image": TEST_IMAGE,
            "ports": [port]}]
    name = random_test_name("default")

    workload = p_client.create_workload(name=name,
                                        containers=con,
                                        namespaceId=ns.id,
                                        daemonSetConfig={})
    workload = wait_for_wl_to_active(p_client, workload)
    validate_lb(p_client, workload, source_port)

#Converted to go test in TestClusterIPScaleAndUpgrade
def test_wl_with_clusterIp_scale_and_upgrade():
    p_client = namespace["p_client"]
    ns = namespace["ns"]
    source_port = 30459
    port = {"containerPort": TEST_IMAGE_PORT,
            "type": "containerPort",
            "kind": "ClusterIP",
            "protocol": "TCP",
            "sourcePort": source_port}
    con = [{"name": "test-cluster-ip",
            "image": TEST_IMAGE,
            "ports": [port]}]
    name = random_test_name("cluster-ip-scale-upgrade")
    workload = p_client.create_workload(name=name,
                                        containers=con,
                                        namespaceId=ns.id,
                                        scale=1)
    workload = wait_for_wl_to_active(p_client, workload)
    wait_for_pods_in_workload(p_client, workload, 1)
    sd_records = p_client.list_dns_record(name=name).data
    assert len(sd_records) == 1
    cluster_ip = sd_records[0].clusterIp
    # get test pods
    wlname = random_test_name("testclusterip-client")
    wl_con = [{"name": "test1", "image": TEST_IMAGE}]
    workload_for_test = p_client.create_workload(name=wlname,
                                                 containers=wl_con,
                                                 namespaceId=ns.id,
                                                 scale=2)
    wait_for_wl_to_active(p_client, workload_for_test)
    test_pods = wait_for_pods_in_workload(p_client, workload_for_test, 2)
    validate_clusterIp(p_client, workload, cluster_ip, test_pods, source_port)

    # scale up
    p_client.update(workload, scale=3, caontainers=con)
    workload = wait_for_wl_to_active(p_client, workload)
    wait_for_pods_in_workload(p_client, workload, 3)
    validate_clusterIp(p_client, workload, cluster_ip, test_pods, source_port)

    # scale down
    p_client.update(workload, scale=2, containers=con)
    workload = wait_for_wl_to_active(p_client, workload)
    wait_for_pods_in_workload(p_client, workload, 2)
    validate_clusterIp(p_client, workload, cluster_ip, test_pods, source_port)
    # upgrade
    con = [{"name": "test-cluster-ip-upgrade-new",
            "image": TEST_IMAGE,
            "ports": [port]}]
    p_client.update(workload, containers=con)
    workload = wait_for_wl_to_active(p_client, workload)
    wait_for_pods_in_workload(p_client, workload, 2)
    validate_clusterIp(p_client, workload, cluster_ip, test_pods, source_port)

#Converted to go test in TestNodePortScaleAndUpgrade
@skip_host_node_port
def test_wl_with_nodePort_scale_and_upgrade():
    p_client = namespace["p_client"]
    ns = namespace["ns"]
    source_port = 30457
    port = {"containerPort": TEST_IMAGE_PORT,
            "type": "containerPort",
            "kind": "NodePort",
            "protocol": "TCP",
            "sourcePort": source_port}
    con = [{"name": "test1",
            "image": TEST_IMAGE,
            "ports": [port]}]
    name = random_test_name("test-node-port-scale-upgrade")
    workload = p_client.create_workload(name=name,
                                        containers=con,
                                        namespaceId=ns.id,
                                        scale=1)
    workload = wait_for_wl_to_active(p_client, workload)
    wait_for_pods_in_workload(p_client, workload, 1)
    validate_nodePort(p_client, workload, namespace["cluster"], source_port)

    # scale up
    p_client.update(workload, scale=3, containers=con)
    workload = wait_for_wl_to_active(p_client, workload)
    wait_for_pods_in_workload(p_client, workload, 3)
    validate_nodePort(p_client, workload, namespace["cluster"], source_port)

    # scale down
    p_client.update(workload, scale=2, containers=con)
    workload = wait_for_wl_to_active(p_client, workload)
    wait_for_pods_in_workload(p_client, workload, 2)
    validate_nodePort(p_client, workload, namespace["cluster"], source_port)

    # upgrade
    con = [{"name": "test-node-port-scale-upgrade-new",
            "image": TEST_IMAGE,
            "ports": [port]}]
    p_client.update(workload, containers=con)
    workload = wait_for_wl_to_active(p_client, workload)
    wait_for_pods_in_workload(p_client, workload, 2)
    validate_nodePort(p_client, workload, namespace["cluster"], source_port)

#Converted to go test in TestHostPortScaleAndUpgrade
# Windows could not support host port for now.
@skip_test_windows_os
@skip_host_node_port
def test_wl_with_hostPort_scale_and_upgrade():
    p_client = namespace["p_client"]
    ns = namespace["ns"]
    source_port = 8888
    port = {"containerPort": TEST_IMAGE_PORT,
            "type": "containerPort",
            "kind": "HostPort",
            "protocol": "TCP",
            "sourcePort": source_port}
    con = [{"name": "test-host-port-upgrade",
            "image": TEST_IMAGE,
            "ports": [port]}]
    name = random_test_name("hostport-scale")

    workload = p_client.create_workload(name=name,
                                        containers=con,
                                        namespaceId=ns.id,
                                        scale=1)
    workload = wait_for_wl_to_active(p_client, workload)
    wait_for_pods_in_workload(p_client, workload, 1)
    validate_hostPort(p_client, workload, source_port, namespace["cluster"])

    # scale up
    p_client.update(workload, scale=2, containers=con)
    workload = wait_for_wl_to_active(p_client, workload)
    wait_for_pods_in_workload(p_client, workload, 2)
    validate_hostPort(p_client, workload, source_port, namespace["cluster"])

    # scale down
    p_client.update(workload, scale=1, containers=con)
    workload = wait_for_wl_to_active(p_client, workload)
    wait_for_pods_in_workload(p_client, workload, 1)
    validate_hostPort(p_client, workload, source_port, namespace["cluster"])
    # From my observation, it is necessary to wait until
    # the number of pod equals to the expected number,
    # since the workload's state is 'active' but pods
    # are not ready yet especially after scaling down and upgrading.

    # upgrade
    con = [{"name": "test-host-port-upgrade-new",
            "image": TEST_IMAGE,
            "ports": [port]}]
    p_client.update(workload, containers=con)
    workload = wait_for_wl_to_active(p_client, workload)
    wait_for_pods_in_workload(p_client, workload, 1)
    validate_hostPort(p_client, workload, source_port, namespace["cluster"])

#Converted to go test in TestLoadBalanceScaleAndUpgrade
@if_check_lb
def test_wl_with_lb_scale_and_upgrade():
    p_client = namespace["p_client"]
    ns = namespace["ns"]
    source_port = 9001
    port = {"containerPort": TEST_IMAGE_PORT,
            "type": "containerPort",
            "kind": "LoadBalancer",
            "protocol": "TCP",
            "sourcePort": source_port}
    con = [{"name": "test1",
            "image": TEST_IMAGE,
            "ports": [port]}]
    name = random_test_name("lb-scale-upgrade")

    workload = p_client.create_workload(name=name,
                                        containers=con,
                                        namespaceId=ns.id,
                                        scale=1)
    workload = wait_for_wl_to_active(p_client, workload)
    wait_for_pods_in_workload(p_client, workload, 1)
    validate_lb(p_client, workload, source_port)

    # scale up
    p_client.update(workload, scale=3, containers=con)
    workload = wait_for_wl_to_active(p_client, workload)
    wait_for_pods_in_workload(p_client, workload, 3)
    validate_lb(p_client, workload, source_port)

    # scale down
    p_client.update(workload, scale=2, containers=con)
    workload = wait_for_wl_to_active(p_client, workload)
    wait_for_pods_in_workload(p_client, workload, 2)
    validate_lb(p_client, workload, source_port)

    # upgrade
    con = [{"name": "test-load-balance-upgrade-new",
            "image": TEST_IMAGE,
            "ports": [port]}]
    p_client.update(workload, containers=con)
    workload = wait_for_wl_to_active(p_client, workload)
    wait_for_pods_in_workload(p_client, workload, 2)
    validate_lb(p_client, workload, source_port)


# --------------------- rbac tests for cluster owner -----------------------
@if_test_rbac
def test_rbac_cluster_owner_wl_create(remove_resource):
    # cluster owner can create project and deploy workload in it
    p_client, project, ns, workload = setup_project_by_role(CLUSTER_OWNER,
                                                            remove_resource)
    validate_workload(p_client, workload, "deployment", ns.name)


@if_test_rbac
def test_rbac_cluster_owner_wl_create_2(remove_resource):
    # cluster owner can deploy workload in any project in the cluster
    user_token = rbac_get_user_token_by_role(CLUSTER_OWNER)
    p2 = rbac_get_unshared_project()
    p_client2 = get_project_client_for_token(p2, user_token)
    ns2 = rbac_get_unshared_ns()
    name = random_test_name("default")
    con = [{"name": "test1",
            "image": TEST_IMAGE}]
    wl = p_client2.create_workload(name=name, containers=con,
                                   namespaceId=ns2.id)
    validate_workload(p_client2, wl, "deployment", ns2.name)

    remove_resource(wl)


@if_test_rbac
def test_rbac_cluster_owner_wl_edit(remove_resource):
    p_client, project, ns, workload = setup_project_by_role(CLUSTER_OWNER,
                                                            remove_resource)
    validate_workload(p_client, workload, "deployment", ns.name)
    # cluster owner can edit workload in the project
    p_client.update(workload, scale=2)
    validate_workload(p_client, workload, "deployment", ns.name, 2)
    con = [{"name": "test1",
            "image": "nginx"}]
    p_client.update(workload, containers=con)
    validate_workload(p_client, workload, "deployment", ns.name, 2)
    validate_workload_image(p_client, workload, "nginx", ns)


@if_test_rbac
def test_rbac_cluster_owner_wl_delete(remove_resource):
    p_client, project, ns, workload = setup_project_by_role(CLUSTER_OWNER,
                                                            remove_resource)
    validate_workload(p_client, workload, "deployment", ns.name)
    # cluster owner can delete workload in the project
    p_client.delete(workload)
    assert len(p_client.list_workload(uuid=workload.uuid).data) == 0


# --------------------- rbac tests for cluster member -----------------------
@if_test_rbac
def test_rbac_cluster_member_wl_create(remove_resource):
    # cluster member can create project and deploy workload in it
    p_client, project, ns, workload = setup_project_by_role(CLUSTER_MEMBER,
                                                            remove_resource)
    validate_workload(p_client, workload, "deployment", ns.name)


@if_test_rbac
def test_rbac_cluster_member_wl_create_2():
    user_token = rbac_get_user_token_by_role(CLUSTER_MEMBER)
    name = random_test_name("default")
    con = [{"name": "test1", "image": TEST_IMAGE}]
    # cluster member can NOT deploy workload in the project he can NOT access
    with pytest.raises(ApiError) as e:
        p2 = rbac_get_unshared_project()
        ns2 = rbac_get_unshared_ns()
        new_p_client = get_project_client_for_token(p2, user_token)
        new_p_client.create_workload(name=name, containers=con,
                                     namespaceId=ns2.id)
    assert e.value.error.status == 403
    assert e.value.error.code == 'Forbidden'


@if_test_rbac
def test_rbac_cluster_member_wl_edit(remove_resource):
    p_client, project, ns, workload = setup_project_by_role(CLUSTER_MEMBER,
                                                            remove_resource)
    validate_workload(p_client, workload, "deployment", ns.name)
    # cluster member can edit workload in the project
    p_client.update(workload, scale=2)
    validate_workload(p_client, workload, "deployment", ns.name, 2)
    con = [{"name": "test1", "image": "nginx"}]
    p_client.update(workload, containers=con)
    validate_workload(p_client, workload, "deployment", ns.name, 2)
    validate_workload_image(p_client, workload, "nginx", ns)


@if_test_rbac
def test_rbac_cluster_member_wl_delete(remove_resource):
    p_client, project, ns, workload = setup_project_by_role(CLUSTER_MEMBER,
                                                            remove_resource)
    validate_workload(p_client, workload, "deployment", ns.name)
    # cluster member can delete workload in the project
    p_client.delete(workload)
    assert len(p_client.list_workload(uuid=workload.uuid).data) == 0


# --------------------- rbac tests for project member -----------------------
@if_test_rbac
def test_rbac_project_member_wl_create(remove_resource):
    # project member can deploy workload in his project
    p_client, project, ns, workload = setup_project_by_role(PROJECT_MEMBER,
                                                            remove_resource)
    validate_workload(p_client, workload, "deployment", ns.name)


@if_test_rbac
def test_rbac_project_member_wl_create_2():
    # project member can NOT deploy workload in the project he can NOT access
    user_token = rbac_get_user_token_by_role(PROJECT_MEMBER)
    name = random_test_name("default")
    con = [{"name": "test1", "image": TEST_IMAGE}]
    with pytest.raises(ApiError) as e:
        p2 = rbac_get_unshared_project()
        ns2 = rbac_get_unshared_ns()
        new_p_client = get_project_client_for_token(p2, user_token)
        new_p_client.create_workload(name=name, containers=con,
                                     namespaceId=ns2.id)
    assert e.value.error.status == 403
    assert e.value.error.code == 'Forbidden'


@if_test_rbac
def test_rbac_project_member_wl_edit(remove_resource):
    p_client, project, ns, workload = setup_project_by_role(PROJECT_MEMBER,
                                                            remove_resource)
    validate_workload(p_client, workload, "deployment", ns.name)
    # project member can edit workload in the project
    p_client.update(workload, scale=2)
    validate_workload(p_client, workload, "deployment", ns.name, 2)
    con = [{"name": "test1", "image": "nginx"}]
    p_client.update(workload, containers=con)
    validate_workload(p_client, workload, "deployment", ns.name, 2)
    validate_workload_image(p_client, workload, "nginx", ns)


@if_test_rbac
def test_rbac_project_member_wl_delete(remove_resource):
    p_client, project, ns, workload = setup_project_by_role(PROJECT_MEMBER,
                                                            remove_resource)
    validate_workload(p_client, workload, "deployment", ns.name)
    # project member can delete workload in the project
    p_client.delete(workload)
    assert len(p_client.list_workload(uuid=workload.uuid).data) == 0


# --------------------- rbac tests for project owner -----------------------
@if_test_rbac
def test_rbac_project_owner_wl_create(remove_resource):
    # project owner can deploy workload in his project
    p_client, project, ns, workload = setup_project_by_role(PROJECT_OWNER,
                                                            remove_resource)
    validate_workload(p_client, workload, "deployment", ns.name)


@if_test_rbac
def test_rbac_project_owner_wl_create_2():
    # project owner can NOT deploy workload in the project he can NOT access
    user_token = rbac_get_user_token_by_role(PROJECT_OWNER)
    name = random_test_name("default")
    con = [{"name": "test1", "image": TEST_IMAGE}]
    with pytest.raises(ApiError) as e:
        p2 = rbac_get_unshared_project()
        ns2 = rbac_get_unshared_ns()
        new_p_client = get_project_client_for_token(p2, user_token)
        new_p_client.create_workload(name=name, containers=con,
                                     namespaceId=ns2.id)
    assert e.value.error.status == 403
    assert e.value.error.code == 'Forbidden'


@if_test_rbac
def test_rbac_project_owner_wl_edit(remove_resource):
    p_client, project, ns, workload = setup_project_by_role(PROJECT_OWNER,
                                                            remove_resource)
    validate_workload(p_client, workload, "deployment", ns.name)
    # project owner can edit workload in his project
    p_client.update(workload, scale=2)
    validate_workload(p_client, workload, "deployment", ns.name, 2)
    con = [{"name": "test1", "image": "nginx"}]
    p_client.update(workload, containers=con)
    validate_workload(p_client, workload, "deployment", ns.name, 2)
    validate_workload_image(p_client, workload, "nginx", ns)


@if_test_rbac
def test_rbac_project_owner_wl_delete(remove_resource):
    p_client, project, ns, workload = setup_project_by_role(PROJECT_OWNER,
                                                            remove_resource)
    validate_workload(p_client, workload, "deployment", ns.name)
    # project owner can delete workload in his project
    p_client.delete(workload)
    assert len(p_client.list_workload(uuid=workload.uuid).data) == 0


# --------------------- rbac tests for project read-only --------------------
@if_test_rbac
def test_rbac_project_read_only_wl_create():
    # project read-only can NOT deploy workloads in the project
    project = rbac_get_project()
    user_token = rbac_get_user_token_by_role(PROJECT_READ_ONLY)
    p_client = get_project_client_for_token(project, user_token)
    ns = rbac_get_namespace()
    con = [{"name": "test1", "image": TEST_IMAGE}]
    name = random_test_name("default")
    with pytest.raises(ApiError) as e:
        p_client.create_workload(name=name, containers=con,
                                 namespaceId=ns.id)
    assert e.value.error.status == 403
    assert e.value.error.code == 'Forbidden'


@if_test_rbac
def test_rbac_project_read_only_wl_edit(remove_resource):
    project = rbac_get_project()
    user_token = rbac_get_user_token_by_role(PROJECT_READ_ONLY)
    p_client = get_project_client_for_token(project, user_token)
    # deploy a workload as cluster owner
    cluster_owner_token = rbac_get_user_token_by_role(CLUSTER_OWNER)
    cluster_owner_p_client = get_project_client_for_token(project,
                                                          cluster_owner_token)
    ns = rbac_get_namespace()
    con = [{"name": "test1", "image": TEST_IMAGE}]
    name = random_test_name("default")
    workload = cluster_owner_p_client.create_workload(name=name,
                                                      containers=con,
                                                      namespaceId=ns.id)
    # project read-only can NOT edit existing workload
    with pytest.raises(ApiError) as e:
        p_client.update(workload, scale=2)
    assert e.value.error.status == 403
    assert e.value.error.code == 'Forbidden'

    remove_resource(workload)


@if_test_rbac
def test_rbac_project_read_only_wl_list():
    # project read-only can NOT see workloads in the project he has no access
    p2 = rbac_get_unshared_project()
    user_token = rbac_get_user_token_by_role(PROJECT_READ_ONLY)
    p_client = get_project_client_for_token(p2, user_token)
    workloads = p_client.list_workload().data
    assert len(workloads) == 0


@pytest.fixture(scope='module', autouse="True")
def create_project_client(request):
    client, cluster = get_user_client_and_cluster()
    create_kubeconfig(cluster)
    p, ns = create_project_and_ns(
        USER_TOKEN, cluster, random_test_name("testworkload"))
    p_client = get_project_client_for_token(p, USER_TOKEN)
    namespace["p_client"] = p_client
    namespace["ns"] = ns
    namespace["cluster"] = cluster
    namespace["project"] = p

    def fin():
        client = get_user_client()
        client.delete(namespace["project"])
    request.addfinalizer(fin)


def setup_project_by_role(role, remove_resource):
    """ set up a project for a specific role used for rbac testing

    - for cluster owner or cluster member:
      it creates a project and namespace, then deploys a workload
    - for project owner or project member:
      it deploys a workload to the existing project and namespace
    """
    user_token = rbac_get_user_token_by_role(role)
    con = [{"name": "test1", "image": TEST_IMAGE}]
    name = random_test_name("default")

    if role in [CLUSTER_OWNER, CLUSTER_MEMBER]:
        project, ns = create_project_and_ns(user_token, namespace["cluster"],
                                            random_test_name("test-rbac"))
        p_client = get_project_client_for_token(project, user_token)
        workload = p_client.create_workload(name=name, containers=con,
                                            namespaceId=ns.id)

        remove_resource(project)
        remove_resource(ns)
        remove_resource(workload)
        return p_client, project, ns, workload

    elif role in [PROJECT_OWNER, PROJECT_MEMBER]:
        project = rbac_get_project()
        ns = rbac_get_namespace()
        p_client = get_project_client_for_token(project, user_token)
        workload = p_client.create_workload(name=name, containers=con,
                                            namespaceId=ns.id)

        remove_resource(workload)
        return p_client, project, ns, workload

    else:
        return None, None, None, None

# --------------------- rbac tests by workload types -----------------------

@if_test_rbac
@pytest.mark.parametrize("role", RBAC_ROLES)
@pytest.mark.parametrize("config", WORKLOAD_TYPES)
def test_rbac_wl_parametarize_create(role, config, remove_resource):
    p_client, project, ns = setup_wl_project_by_role(role)
    cluster = namespace["cluster"]
    con = [{"name": "test1", "image": TEST_IMAGE}]
    name = random_test_name("default")
    if role != PROJECT_READ_ONLY:
        workload = create_workload_by_type(p_client, name, con, ns, config)
        wait_for_wl_to_active(p_client, workload)
        remove_resource(workload)
        if role == CLUSTER_MEMBER:
            remove_resource(project)
        return None
    else:
        with pytest.raises(ApiError) as e:
            workload = create_workload_by_type(p_client, name, con, ns, config)
        assert e.value.error.status == 403
        assert e.value.error.code == 'Forbidden'


@if_test_rbac
@pytest.mark.parametrize("role", RBAC_ROLES)
@pytest.mark.parametrize("config", WORKLOAD_TYPES)
def test_rbac_wl_parametrize_create_negative(role, remove_resource, config):
    if role == CLUSTER_OWNER:
        # cluster owner can deploy workloads in any project in the cluster
        user_token = rbac_get_user_token_by_role(CLUSTER_OWNER)
        p2 = rbac_get_unshared_project()
        p_client2 = get_project_client_for_token(p2, user_token)
        ns2 = rbac_get_unshared_ns()
        name = random_test_name("default")
        con = [{"name": "test1", "image": TEST_IMAGE}]
        wl = create_workload_by_type(p_client2, name, con, ns2, config)
        wait_for_wl_to_active(p_client2, wl)
        remove_resource(wl)
    else:
        # roles cannot deploy workloads in projects they cannot access
        user_token = rbac_get_user_token_by_role(role)
        name = random_test_name("default")
        con = [{"name": "test1", "image": TEST_IMAGE}]
        with pytest.raises(ApiError) as e:
            p2 = rbac_get_unshared_project()
            ns2 = rbac_get_unshared_ns()
            new_p_client = get_project_client_for_token(p2, user_token)
            workload = create_workload_by_type(new_p_client, name, con, ns2, config)
        assert e.value.error.status == 403
        assert e.value.error.code == 'Forbidden'


@if_test_rbac
@pytest.mark.parametrize("role", RBAC_ROLES)
@pytest.mark.parametrize("config", WORKLOAD_TYPES)
def test_rbac_wl_parametrize_list(role, remove_resource, config):
    if role == CLUSTER_MEMBER:
        p_client, project, ns = setup_wl_project_by_role(role)
    else:
        p_client, project, ns = setup_wl_project_by_role(CLUSTER_OWNER)
    con = [{"name": "test1", "image": TEST_IMAGE}]
    name = random_test_name("default")
    workload = create_workload_by_type(p_client, name, con, ns, config)
    wait_for_wl_to_active(p_client, workload)
    # switch to rbac role
    user_token = rbac_get_user_token_by_role(role)
    p_client_rbac = get_project_client_for_token(project, user_token)
    assert len(p_client_rbac.list_workload(uuid=workload.uuid).data) == 1
    remove_resource(workload)


@if_test_rbac
@pytest.mark.parametrize("role", RBAC_ROLES)
@pytest.mark.parametrize("config", WORKLOAD_TYPES)
def test_rbac_wl_parametrize_list_negative(role, remove_resource, config):
    unshared_project = rbac_get_unshared_project()
    ns = rbac_get_unshared_ns()
    user_token = rbac_get_user_token_by_role(CLUSTER_OWNER)
    p_client = get_project_client_for_token(unshared_project, user_token)
    con = [{"name": "test1", "image": TEST_IMAGE}]
    name = random_test_name("default")
    workload = p_client.create_workload(name=name,
                                        containers=con,
                                        namespaceId=ns.id,
                                        daemonSetConfig={})
    wait_for_wl_to_active(p_client, workload)

    # switch to rbac role
    user_token = rbac_get_user_token_by_role(role)
    p_client_rbac = get_project_client_for_token(unshared_project, user_token)
    if role != CLUSTER_OWNER:
        assert len(p_client_rbac.list_workload(uuid=workload.uuid).data) == 0
    else:
        assert len(p_client_rbac.list_workload(uuid=workload.uuid).data) == 1
    remove_resource(workload)


@if_test_rbac
@pytest.mark.parametrize("role", RBAC_ROLES)
@pytest.mark.parametrize("config", WORKLOAD_TYPES)
def test_rbac_wl_parametrize_update(role, remove_resource, config):
    # workloads of type job cannot be edited
    if config == "job":
        return
    p_client, project, ns = setup_wl_project_by_role(role)
    con = [{"name": "test1", "image": TEST_IMAGE}]
    name = random_test_name("default")
    if role != PROJECT_READ_ONLY:
        workload = create_workload_by_type(p_client, name, con, ns, config)
        wait_for_wl_to_active(p_client, workload)
        con = [{"name": "test1", "image": os.environ.get('RANCHER_TEST_IMAGE',
                                                         "nginx")}]
        p_client.update(workload, containers=con)
        remove_resource(workload)
        if role == CLUSTER_MEMBER:
            remove_resource(project)
    else:
        user_token = rbac_get_user_token_by_role(CLUSTER_OWNER)
        p_client = get_project_client_for_token(project, user_token)
        ns = rbac_get_namespace()
        workload = create_workload_by_type(p_client, name, con, ns, config)
        wait_for_wl_to_active(p_client, workload)
        with pytest.raises(ApiError) as e:
            user_token = rbac_get_user_token_by_role(role)
            p_client = get_project_client_for_token(project, user_token)
            con = [{"name": "test1", "image": os.environ.get('RANCHER_TEST_IMAGE',
                                                         "nginx")}]
            p_client.update(workload, containers=con)
            wait_for_pods_in_workload(p_client, workload)
            validate_workload(p_client, workload, config, ns.name)
        assert e.value.error.status == 403
        assert e.value.error.code == 'Forbidden'
        remove_resource(workload)


@if_test_rbac
@pytest.mark.parametrize("role", RBAC_ROLES)
@pytest.mark.parametrize("config", WORKLOAD_TYPES)
def test_rbac_wl_parametrize_update_negative(role, remove_resource, config):
    # workloads of type job cannot be edited
    if config == "job":
        return
    if role == CLUSTER_OWNER:
        # cluster owner can edit workloads in any project in the cluster
        user_token = rbac_get_user_token_by_role(CLUSTER_OWNER)
        p_client, project, ns = setup_wl_project_by_role(role)
        name = random_test_name("default")
        con = [{"name": "test1", "image": TEST_IMAGE}]
        workload = create_workload_by_type(p_client, name, con, ns, config)
        wait_for_wl_to_active(p_client, workload)
        con = [{"name": "test1", "image": "nginx"}]
        p_client.update(workload, containers=con)
        remove_resource(workload)
    else:
        project2 = rbac_get_unshared_project()
        user_token = rbac_get_user_token_by_role(role)
        # roles cannot edit workloads in projects they cannot access
        # deploy a workload as cluster owner
        cluster_owner_token = rbac_get_user_token_by_role(CLUSTER_OWNER)
        cluster_owner_p_client = get_project_client_for_token(
                                project2, cluster_owner_token)
        ns = rbac_get_unshared_ns()
        con = [{"name": "test1", "image": TEST_IMAGE}]
        name = random_test_name("default")
        workload = create_workload_by_type(cluster_owner_p_client,
                                           name, con, ns, config)
        with pytest.raises(ApiError) as e:
            p_client = get_project_client_for_token(project2, user_token)
            con = [{"name": "test1", "image": "nginx"}]
            p_client.update(workload, containers=con)
        assert e.value.error.status == 403
        assert e.value.error.code == 'Forbidden'
        remove_resource(workload)


@if_test_rbac
@pytest.mark.parametrize("role", RBAC_ROLES)
@pytest.mark.parametrize("config", WORKLOAD_TYPES)
def test_rbac_wl_parametrize_delete(role, remove_resource, config):
    p_client, project, ns = setup_wl_project_by_role(role)
    con = [{"name": "test1", "image": TEST_IMAGE}]
    name = random_test_name("default")
    if role != PROJECT_READ_ONLY:
        workload = create_workload_by_type(p_client, name, con, ns, config)
        wait_for_wl_to_active(p_client, workload)
        p_client.delete(workload)
        assert len(p_client.list_workload(uuid=workload.uuid).data) == 0
        remove_resource(workload)
    else:
        user_token = rbac_get_user_token_by_role(CLUSTER_OWNER)
        p_client = get_project_client_for_token(project, user_token)
        ns = rbac_get_namespace()
        workload = create_workload_by_type(p_client, name, con, ns, config)
        wait_for_wl_to_active(p_client, workload)
        user_token = rbac_get_user_token_by_role(role)
        p_client = get_project_client_for_token(project, user_token)
        with pytest.raises(ApiError) as e:
            p_client.delete(workload)
        assert e.value.error.status == 403
        assert e.value.error.code == 'Forbidden'
        remove_resource(workload)
        if role == CLUSTER_MEMBER:
            remove_resource(project)


@if_test_rbac
@pytest.mark.parametrize("role", RBAC_ROLES)
@pytest.mark.parametrize("config", WORKLOAD_TYPES)
def test_rbac_wl_parametrize_delete_negative(role, remove_resource, config):
    if role == CLUSTER_OWNER:
        # cluster owner can delete workloads in any project in the cluster
        user_token = rbac_get_user_token_by_role(CLUSTER_OWNER)
        project = rbac_get_unshared_project()
        p_client = get_project_client_for_token(project, user_token)
        ns = rbac_get_namespace()
        name = random_test_name("default")
        con = [{"name": "test1", "image": TEST_IMAGE}]
        workload = create_workload_by_type(p_client, name, con, ns, config)
        p_client.delete(workload)
    else:
        project = rbac_get_unshared_project()
        user_token = rbac_get_user_token_by_role(role)
        # roles cannot delete workloads in projects they cannot access
        # deploy a workload as cluster owner
        cluster_owner_token = rbac_get_user_token_by_role(CLUSTER_OWNER)
        cluster_owner_p_client = get_project_client_for_token(
                                project, cluster_owner_token)
        ns = rbac_get_unshared_ns()
        con = [{"name": "test1", "image": TEST_IMAGE}]
        name = random_test_name("default")
        workload = create_workload_by_type(cluster_owner_p_client,
                                           name, con, ns, config)
        p_client = get_project_client_for_token(project, user_token)
        with pytest.raises(ApiError) as e:
            p_client.delete(workload)
        assert e.value.error.status == 403
        assert e.value.error.code == 'Forbidden'
        remove_resource(workload)


def setup_wl_project_by_role(role):
    if role == CLUSTER_MEMBER:
        user_token = rbac_get_user_token_by_role(role)
        project, ns = create_project_and_ns(user_token, namespace["cluster"],
                                            random_test_name("test-rbac"))
        p_client = get_project_client_for_token(project, user_token)
        return p_client, project, ns
    else:
        project = rbac_get_project()
        user_token = rbac_get_user_token_by_role(role)
        p_client = get_project_client_for_token(project, user_token)
        ns = rbac_get_namespace()
        return p_client, project, ns

def create_workload_by_type(client, name, con, ns, config):
    if config == "daemonSet":
        return client.create_workload(name=name,
                                      containers=con,
                                      namespaceId=ns.id,
                                      daemonSetConfig={})
    elif config == "statefulSet":
        return client.create_workload(name=name,
                                      containers=con,
                                      namespaceId=ns.id,
                                      statefulSetConfig={})
    elif config == "cronJob":
        return client.create_workload(name=name,
                                      containers=con,
                                      namespaceId=ns.id,
                                      cronJobConfig={
                                            "concurrencyPolicy": "Allow",
                                            "failedJobsHistoryLimit": 10,
                                            "schedule": "*/1 * * * *",
                                            "successfulJobsHistoryLimit": 10})
    elif config == "job":
        return client.create_workload(name=name,
                                      containers=con,
                                      namespaceId=ns.id,
                                      jobConfig={})