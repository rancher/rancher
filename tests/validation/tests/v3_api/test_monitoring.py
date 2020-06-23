import pytest
import copy
from .common import *  # NOQA


namespace = {
    "cluster": None,
    "project": None,
    "system_project": None,
    "system_project_client": None
}

cluster_query_template = {
    "obj": None,
    "action_name": "query",
    "filters": {},
    "metricParams": {},
    "interval": "5s",
    "isDetails": True,
    "from": "now-5s",
    "to": "now"
}

cluster_graph_list = [
    "cluster-network-packet",
    "cluster-network-io",
    "cluster-disk-io",
    "cluster-cpu-load",
    "cluster-cpu-usage",
    "cluster-fs-usage-percent",
    "cluster-memory-usage",
]

etcd_graph_list = [
    "etcd-grpc-client",
    "etcd-stream",
    "etcd-raft-proposals",
    "etcd-server-leader-sum",
    "etcd-db-bytes-sum",
    "etcd-sync-duration",
    "etcd-server-failed-proposal",
    "etcd-leader-change",
    "etcd-rpc-rate",
    'etcd-peer-traffic'
]

kube_component_graph_list = [
    "scheduler-total-preemption-attempts",
    "ingresscontroller-nginx-connection",
    "apiserver-request-count",
    "controllermanager-queue-depth",
    "scheduler-e-2-e-scheduling-latency-seconds-quantile",
    "scheduler-pod-unscheduler",
    "apiserver-request-latency",
]

node_graph_list = [
    "node-network-packet",
    "node-network-io",
    "node-fs-usage-percent",
    "node-cpu-load",
    "node-disk-io",
    "node-memory-usage",
    "node-cpu-usage",
]

rancher_component_graph_list = [
    "fluentd-buffer-queue-length",
    "fluentd-input-record-number",
    "fluentd-output-errors",
    "fluentd-output-record-number",
]

workload_graph_list = [
    "workload-network-packet",
    "workload-memory-usage-bytes-sum",
    "workload-cpu-usage",
    "workload-network-io",
    "workload-disk-io",
]

name_mapping = {
    "cluster": cluster_graph_list,
    "etcd": etcd_graph_list,
    "kube-component": kube_component_graph_list,
    "rancher-component": rancher_component_graph_list,
    "workload": workload_graph_list,
    "node": node_graph_list,
}
STORAGE_CLASS = "longhorn"
ENABLE_STORAGE = os.environ.get('RANCHER_ENABLE_STORAGE_FOR_MONITORING',
                                "false")
ENABLE_STORAGE = ENABLE_STORAGE.lower()
if ENABLE_STORAGE == "false":
    STORAGE_CLASS = "default"

# Longhorn is provided as the persistence storage class
C_MONITORING_ANSWERS = {"operator-init.enabled": "true",
                        "exporter-node.enabled": "true",
                        "exporter-node.ports.metrics.port": "9796",
                        "exporter-kubelets.https": "true",
                        "exporter-node.resources.limits.cpu": "200m",
                        "exporter-node.resources.limits.memory": "200Mi",
                        "operator.resources.limits.memory": "500Mi",
                        "prometheus.retention": "12h",
                        "grafana.persistence.enabled": ENABLE_STORAGE,
                        "prometheus.persistence.enabled": ENABLE_STORAGE,
                        "prometheus.persistence.storageClass": STORAGE_CLASS,
                        "grafana.persistence.storageClass": STORAGE_CLASS,
                        "grafana.persistence.size": "10Gi",
                        "prometheus.persistence.size": "10Gi",
                        "prometheus.resources.core.requests.cpu": "750m",
                        "prometheus.resources.core.limits.cpu": "1000m",
                        "prometheus.resources.core.requests.memory": "750Mi",
                        "prometheus.resources.core.limits.memory": "1000Mi",
                        "prometheus.persistent.useReleaseName": "true"}

P_MONITORING_ANSWER = {"prometheus.retention": "12h",
                       "grafana.persistence.enabled": "false",
                       "prometheus.persistence.enabled": "false",
                       "prometheus.persistence.storageClass": "default",
                       "grafana.persistence.storageClass": "default",
                       "grafana.persistence.size": "10Gi",
                       "prometheus.persistence.size": "10Gi",
                       "prometheus.resources.core.requests.cpu": "750m",
                       "prometheus.resources.core.limits.cpu": "1000m",
                       "prometheus.resources.core.requests.memory": "750Mi",
                       "prometheus.resources.core.limits.memory": "1000Mi",
                       "prometheus.persistent.useReleaseName": "true"}

MONITORING_VERSION = os.environ.get('RANCHER_MONITORING_VERSION', "")
MONITORING_TEMPLATE_ID = "cattle-global-data:system-library-rancher-monitoring"
CLUSTER_MONITORING_APP = "cluster-monitoring"
MONITORING_OPERATOR_APP = "monitoring-operator"
PROJECT_MONITORING_APP = "project-monitoring"
GRAFANA_PROJECT_MONITORING = "grafana-project-monitoring"
PROMETHEUS_PROJECT_MONITORING = "prometheus-project-monitoring"
LONGHORN_APP_VERSION = os.environ.get('RANCHER_LONGHORN_VERSION', "1.0.0")


def test_monitoring_cluster_graph():
    rancher_client, cluster = get_user_client_and_cluster()
    cluster_monitoring_obj = rancher_client.list_clusterMonitorGraph()
    # generate the request payload
    query1 = copy.deepcopy(cluster_query_template)
    query1["obj"] = cluster_monitoring_obj
    query1["filters"]["clusterId"] = cluster.id
    query1["filters"]["resourceType"] = "cluster"
    validate_cluster_graph(query1, "cluster")


def test_monitoring_etcd_graph():
    rancher_client, cluster = get_user_client_and_cluster()
    cluster_monitoring_obj = rancher_client.list_clusterMonitorGraph()
    # generate the request payload
    query1 = copy.deepcopy(cluster_query_template)
    query1["obj"] = cluster_monitoring_obj
    query1["filters"]["clusterId"] = cluster.id
    query1["filters"]["resourceType"] = "etcd"
    validate_cluster_graph(query1, "etcd")


def test_monitoring_kube_component_graph():
    rancher_client, cluster = get_user_client_and_cluster()
    cluster_monitoring_obj = rancher_client.list_clusterMonitorGraph()
    # generate the request payload
    query1 = copy.deepcopy(cluster_query_template)
    query1["obj"] = cluster_monitoring_obj
    query1["filters"]["clusterId"] = cluster.id
    query1["filters"]["displayResourceType"] = "kube-component"
    validate_cluster_graph(query1, "kube-component")


# rancher component graphs are from the fluent app for cluster logging
def test_monitoring_rancher_component_graph():
    rancher_client, cluster = get_user_client_and_cluster()
    # check if the cluster logging is enabled, assuming fluent is used
    if cluster.enableClusterAlerting is False:
        print("cluster logging is not enabled, skip the test")
        return
    else:
        cluster_monitoring_obj = rancher_client.list_clusterMonitorGraph()
        # generate the request payload
        query1 = copy.deepcopy(cluster_query_template)
        query1["obj"] = cluster_monitoring_obj
        query1["filters"]["clusterId"] = cluster.id
        query1["filters"]["displayResourceType"] = "rancher-component"
        validate_cluster_graph(query1, "rancher-component")


def test_monitoring_node_graph():
    rancher_client, cluster = get_user_client_and_cluster()
    node_list_raw = rancher_client.list_node(clusterId=cluster.id).data
    for node in node_list_raw:
        cluster_monitoring_obj = rancher_client.list_clusterMonitorGraph()
        # generate the request payload
        query1 = copy.deepcopy(cluster_query_template)
        query1["obj"] = cluster_monitoring_obj
        query1["filters"]["clusterId"] = cluster.id
        query1["filters"]["resourceType"] = "node"
        query1["metricParams"]["instance"] = node.id
        validate_cluster_graph(query1, "node")


def test_monitoring_workload_graph():
    rancher_client, cluster = get_user_client_and_cluster()
    system_project = rancher_client.list_project(clusterId=cluster.id,
                                                 name="System").data[0]
    project_monitoring_obj = rancher_client.list_projectMonitorGraph()
    # generate the request payload
    query1 = copy.deepcopy(cluster_query_template)
    query1["obj"] = project_monitoring_obj
    query1["filters"]["projectId"] = system_project.id
    query1["filters"]["resourceType"] = "workload"
    query1["metricParams"]["workloadName"] = \
        "deployment:cattle-prometheus:grafana-cluster-monitoring"
    validate_cluster_graph(query1, "workload")


def test_monitoring_project_monitoring():
    validate_project_monitoring(namespace["project"], USER_TOKEN)


# ------------------ RBAC for Project Monitoring ------------------
@if_test_rbac
def test_rbac_cluster_owner_control_project_monitoring():
    # cluster owner can enable and disable monitoring in any project
    user_token = rbac_get_user_token_by_role(CLUSTER_OWNER)
    user_client = get_client_for_token(user_token)
    project = user_client.reload(rbac_get_project())

    if project["enableProjectMonitoring"] is True:
        assert "disableMonitoring" in project.actions.keys()
        disable_project_monitoring(project, user_token)
    validate_project_monitoring(project, user_token)


@if_test_rbac
def test_rbac_cluster_member_control_project_monitoring(remove_resource):
    # cluster member can enable and disable monitoring in his project
    user_token = rbac_get_user_token_by_role(CLUSTER_MEMBER)
    user_client = get_client_for_token(user_token)
    # create a new project
    project = create_project(user_client, namespace["cluster"])
    validate_project_monitoring(project, user_token)

    remove_resource(project)


@if_test_rbac
def test_rbac_project_owner_control_project_monitoring():
    # project owner can enable and disable monitoring in his project
    user_token = rbac_get_user_token_by_role(PROJECT_OWNER)
    user_client = get_client_for_token(user_token)
    project = user_client.reload(rbac_get_project())

    if project["enableProjectMonitoring"] is True:
        assert "disableMonitoring" in project.actions.keys()
        disable_project_monitoring(project, user_token)
    validate_project_monitoring(project, user_token)


@if_test_rbac
def test_rbac_project_member_control_project_monitoring():
    # project member can NOT enable and disable monitoring in his project
    token = rbac_get_user_token_by_role(PROJECT_MEMBER)
    validate_no_permission_project_monitoring(token)


@if_test_rbac
def test_rbac_project_read_only_control_project_monitoring():
    # project read-only can NOT enable and disable monitoring in his project
    token = rbac_get_user_token_by_role(PROJECT_READ_ONLY)
    validate_no_permission_project_monitoring(token)


@if_test_rbac
def test_rbac_project_owner_project_graph_1():
    # project owner can see graphs in his project
    project = rbac_get_project()
    wl = rbac_get_workload()
    token = rbac_get_user_token_by_role(PROJECT_OWNER)
    check_permission_project_graph(project, wl, token, True)


@if_test_rbac
def test_rbac_project_owner_project_graph_2():
    # project owner can NOT see graphs in others' project
    project = rbac_get_unshared_project()
    wl = rbac_get_unshared_workload()
    token = rbac_get_user_token_by_role(PROJECT_OWNER)
    check_permission_project_graph(project, wl, token, False)


@if_test_rbac
def test_rbac_project_member_project_graph_1():
    # project member can see graphs in his project
    project = rbac_get_project()
    wl = rbac_get_workload()
    token = rbac_get_user_token_by_role(PROJECT_MEMBER)
    check_permission_project_graph(project, wl, token, True)


@if_test_rbac
def test_rbac_project_member_project_graph_2():
    # project member can NOT see graphs in others' project
    project = rbac_get_unshared_project()
    wl = rbac_get_unshared_workload()
    token = rbac_get_user_token_by_role(PROJECT_MEMBER)
    check_permission_project_graph(project, wl, token, False)


@if_test_rbac
def test_rbac_project_read_only_project_graph_1():
    # project read-only can see graphs in his project
    project = rbac_get_project()
    wl = rbac_get_workload()
    token = rbac_get_user_token_by_role(PROJECT_READ_ONLY)
    check_permission_project_graph(project, wl, token, True)


@if_test_rbac
def test_rbac_project_read_only_project_graph_2():
    # project read-only can NOT see graphs in other's project
    project = rbac_get_unshared_project()
    wl = rbac_get_unshared_workload()
    token = rbac_get_user_token_by_role(PROJECT_READ_ONLY)
    check_permission_project_graph(project, wl, token, False)


@if_test_rbac
def test_rbac_cluster_owner_project_graph():
    # cluster owner can see graphs in all projects
    token = rbac_get_user_token_by_role(CLUSTER_OWNER)
    project1 = rbac_get_project()
    wl1 = rbac_get_workload()
    check_permission_project_graph(project1, wl1, token, True)
    project2 = rbac_get_unshared_project()
    wl2 = rbac_get_unshared_workload()
    check_permission_project_graph(project2, wl2, token, True)


@if_test_rbac
def test_rbac_cluster_member_project_graph_1(remove_resource):
    # cluster member can see graphs in his project only
    token = rbac_get_user_token_by_role(CLUSTER_MEMBER)
    project, ns = create_project_and_ns(token,
                                        namespace["cluster"],
                                        random_test_name("cluster-member"))
    p_client = get_project_client_for_token(project, token)
    con = [{"name": "test1", "image": TEST_IMAGE}]
    name = random_test_name("default")
    workload = p_client.create_workload(name=name,
                                        containers=con,
                                        namespaceId=ns.id)
    wait_for_wl_to_active(p_client, workload)
    remove_resource(project)
    check_permission_project_graph(project, workload, token, True)


@if_test_rbac
def test_rbac_cluster_member_project_graph_2():
    # cluster member can NOT see graphs in other's project
    token = rbac_get_user_token_by_role(CLUSTER_MEMBER)
    project = rbac_get_project()
    wl = rbac_get_workload()
    check_permission_project_graph(project, wl, token, False)


# ------------------ RBAC for Cluster Monitoring ------------------
@if_test_rbac
def test_rbac_project_owner_cluster_graphs():
    # project owner can NOT see cluster graphs
    token = rbac_get_user_token_by_role(PROJECT_OWNER)
    cluster = namespace["cluster"]
    check_permission_cluster_graph(cluster, token, False)


@if_test_rbac
def test_rbac_project_member_cluster_graphs():
    # project member can NOT see cluster graphs
    token = rbac_get_user_token_by_role(PROJECT_MEMBER)
    cluster = namespace["cluster"]
    check_permission_cluster_graph(cluster, token, False)


@if_test_rbac
def test_rbac_project_read_only_cluster_graphs():
    # project read-only can NOT see cluster graphs
    token = rbac_get_user_token_by_role(PROJECT_READ_ONLY)
    cluster = namespace["cluster"]
    check_permission_cluster_graph(cluster, token, False)


@if_test_rbac
def test_rbac_cluster_owner_cluster_graphs():
    # cluster owner can see cluster graph
    token = rbac_get_user_token_by_role(CLUSTER_OWNER)
    cluster = namespace["cluster"]
    check_permission_cluster_graph(cluster, token, True)


@if_test_rbac
def test_rbac_cluster_member_cluster_graphs():
    # cluster member can see cluster graphs
    token = rbac_get_user_token_by_role(CLUSTER_MEMBER)
    cluster = namespace["cluster"]
    check_permission_cluster_graph(cluster, token, True)


@if_test_rbac
def test_rbac_cluster_member_control_cluster_monitoring():
    # cluster member can NOT enable or disable the cluster monitoring
    token = rbac_get_user_token_by_role(CLUSTER_MEMBER)
    validate_no_permission_cluster_monitoring(token)


@if_test_rbac
def test_rbac_project_owner_control_cluster_monitoring():
    # project owner can NOT enable or disable the cluster monitoring
    token = rbac_get_user_token_by_role(PROJECT_OWNER)
    validate_no_permission_cluster_monitoring(token)


@if_test_rbac
def test_rbac_project_member_control_cluster_monitoring():
    # project member can NOT enable or disable the cluster monitoring
    token = rbac_get_user_token_by_role(PROJECT_MEMBER)
    validate_no_permission_cluster_monitoring(token)


@if_test_rbac
def test_rbac_project_read_only_control_cluster_monitoring():
    # project read-only can NOT enable or disable the cluster monitoring
    token = rbac_get_user_token_by_role(PROJECT_READ_ONLY)
    validate_no_permission_cluster_monitoring(token)


@pytest.mark.last
@if_test_rbac
def test_rbac_cluster_owner_control_cluster_monitoring():
    # cluster owner can enable and disable the cluster monitoring
    user_token = rbac_get_user_token_by_role(CLUSTER_OWNER)
    client = get_client_for_token(user_token)
    user_client, cluster = get_user_client_and_cluster(client)

    if cluster["enableClusterMonitoring"] is True:
        assert "disableMonitoring" in cluster.actions.keys()
        user_client.action(cluster, "disableMonitoring")
        # sleep 10 seconds to wait for all apps removed
        time.sleep(10)

    cluster = user_client.reload(cluster)
    assert "enableMonitoring" in cluster.actions.keys()
    user_client.action(cluster, "enableMonitoring",
                       answers=C_MONITORING_ANSWERS,
                       version=MONITORING_VERSION)
    validate_cluster_monitoring_apps()


@pytest.fixture(scope="module", autouse="True")
def setup_monitoring(request):
    '''
    Initialize projects, clients, install longhorn app and enable monitoring
    with persistence storageClass set to longhorn
    '''
    global MONITORING_VERSION
    rancher_client, cluster = get_user_client_and_cluster()
    create_kubeconfig(cluster)
    project = create_project(rancher_client, cluster,
                             random_test_name("p-monitoring"))
    system_project = rancher_client.list_project(clusterId=cluster.id,
                                                 name="System").data[0]
    sys_proj_client = get_project_client_for_token(system_project, USER_TOKEN)
    cluster_client = get_cluster_client_for_token(cluster, USER_TOKEN)
    namespace["cluster"] = cluster
    namespace["project"] = project
    namespace["system_project"] = system_project
    namespace["system_project_client"] = sys_proj_client
    namespace["cluster_client"] = cluster_client

    if ENABLE_STORAGE == "true":
        # Deploy Longhorn app from the library catalog
        app_name = "longhorn"
        ns = create_ns(cluster_client, cluster, project, "longhorn-system")
        app_ext_id = create_catalog_external_id('library', app_name,
                                                LONGHORN_APP_VERSION)
        answer = get_defaut_question_answers(rancher_client, app_ext_id)
        project_client = get_project_client_for_token(project, USER_TOKEN)
        try:
            app = project_client.create_app(
                externalId=app_ext_id,
                targetNamespace=ns.name,
                projectId=ns.projectId,
                answers=answer)
            print(app)
            validate_catalog_app(project_client, app, app_ext_id, answer)
        except (AssertionError, RuntimeError):
            assert False, "App {} deployment/Validation failed."\
                .format(app_name)

    monitoring_template = rancher_client.list_template(
        id=MONITORING_TEMPLATE_ID).data[0]
    if MONITORING_VERSION == "":
        MONITORING_VERSION = monitoring_template.defaultVersion
    print("MONITORING_VERSION=" + MONITORING_VERSION)
    # Enable cluster monitoring
    if cluster["enableClusterMonitoring"] is False:
        rancher_client.action(cluster, "enableMonitoring",
                              answers=C_MONITORING_ANSWERS,
                              version=MONITORING_VERSION)
    validate_cluster_monitoring_apps()

    # Wait 3 minutes for all graphs to be available
    time.sleep(60 * 3)

    def fin():
        rancher_client.delete(project)
        # Disable monitoring
        cluster = rancher_client.reload(namespace["cluster"])
        if cluster["enableClusterMonitoring"] is True:
            rancher_client.action(cluster, "disableMonitoring")

    request.addfinalizer(fin)


def check_data(source, target_list):
    """ check if any graph is missing or any new graph is introduced"""
    if not hasattr(source, "data"):
        return False
    data = source.get("data")
    if len(data) == 0:
        print("no graph is received")
        return False
    target_copy = copy.deepcopy(target_list)
    res = []
    extra = []
    for item in data:
        if not hasattr(item, "series"):
            return False
        if len(item.series) == 0:
            print("no data point")
            return False
        name = item.get("graphID").split(":")[1]
        res.append(name)
        if name in target_list:
            target_copy.remove(name)
        else:
            extra.append(name)

    target_list.sort()
    res.sort()
    target_copy.sort()
    extra.sort()
    if len(target_copy) != 0 or len(extra) != 0:
        print("target graphs : {}".format(target_list))
        print("actual graphs : {}".format(res))
        print("missing graphs: {}".format(target_copy))
        print("extra graphs  : {}".format(extra))
        return False
    return True


def validate_cluster_graph(action_query, resource_type, timeout=10):
    target_graph_list = copy.deepcopy(name_mapping.get(resource_type))
    rancher_client, cluster = get_user_client_and_cluster()
    # handle the special case that if the graph etcd-peer-traffic is
    # is not available if there is only one etcd node in the cluster
    if resource_type == "etcd":
        nodes = get_etcd_nodes(cluster, rancher_client)
        if len(nodes) == 1:
            target_graph_list.remove("etcd-peer-traffic")
    start = time.time()

    if resource_type == "kube-component":
        cluster = namespace["cluster"]
        k8s_version = cluster.appliedSpec["rancherKubernetesEngineConfig"][
            "kubernetesVersion"]
        # the following two graphs are available only in k8s 1.15 and 1.16
        if not k8s_version[0:5] in ["v1.15", "v1.16"]:
            target_graph_list.remove("apiserver-request-latency")
            target_graph_list.remove(
                "scheduler-e-2-e-scheduling-latency-seconds-quantile")

    while True:
        res = rancher_client.action(**action_query)
        if check_data(res, target_graph_list):
            return
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for all graphs to be available")
        time.sleep(2)


def wait_for_target_up(token, cluster, project, job):
    """wait for a job's state to be up in Prometheus"""

    project_client = get_project_client_for_token(project, token)
    app = project_client.list_app(name=PROJECT_MONITORING_APP).data[0]
    url = CATTLE_TEST_URL + '/k8s/clusters/' + cluster.id \
        + '/api/v1/namespaces/' + app.targetNamespace \
        + '/services/http:access-prometheus:80/proxy/api/v1/targets'
    headers1 = {'Authorization': 'Bearer ' + token}
    start = time.time()
    while True:
        t = requests.get(headers=headers1, url=url, verify=False).json()
        for item in t["data"]["activeTargets"]:
            if "job" in item["labels"].keys():
                if item["labels"]["job"] == job and item["health"] == "up":
                    return
        if time.time() - start > DEFAULT_MONITORING_TIMEOUT:
            raise AssertionError(
                "Timed out waiting for target to be up")
        time.sleep(5)


def validate_cluster_monitoring_apps():
    sys_project_client = namespace["system_project_client"]
    wait_for_app_to_active(sys_project_client, CLUSTER_MONITORING_APP)
    wait_for_app_to_active(sys_project_client, MONITORING_OPERATOR_APP)


def validate_no_permission_cluster_monitoring(user_token):
    client = get_client_for_token(user_token)
    _, cluster = get_user_client_and_cluster(client)
    actions = cluster.actions.keys()
    assert "enableMonitoring" not in actions
    assert "disableMonitoring" not in actions
    assert "editMonitoring" not in actions


def validate_no_permission_project_monitoring(user_token):
    user_client = get_client_for_token(user_token)
    project = user_client.reload(rbac_get_project())
    actions = project.actions.keys()
    assert "enableMonitoring" not in actions
    assert "disableMonitoring" not in actions
    assert "editMonitoring" not in actions


def enable_project_monitoring(project, token):
    client = get_client_for_token(token)
    user_client, cluster = get_user_client_and_cluster(client)
    system_project_client = namespace["system_project_client"]
    project = user_client.reload(project)
    project_client = get_project_client_for_token(project, token)

    # enable the project monitoring
    if project["enableProjectMonitoring"] is False:
        assert "enableMonitoring" in project.actions.keys()
        user_client.action(project, "enableMonitoring",
                           answers=P_MONITORING_ANSWER,
                           version=MONITORING_VERSION)
    wait_for_app_to_active(project_client, PROJECT_MONITORING_APP)
    wait_for_app_to_active(system_project_client, MONITORING_OPERATOR_APP)
    # wait for targets to be up
    wait_for_target_up(token, cluster, project, "expose-prometheus-metrics")
    wait_for_target_up(token, cluster, project, "expose-grafana-metrics")


def disable_project_monitoring(project, token):
    user_client = get_client_for_token(token)
    project = user_client.reload(project)
    p_client = get_project_client_for_token(project, token)
    # disable the project monitoring
    assert "disableMonitoring" in project.actions.keys()
    user_client.action(project, "disableMonitoring")
    start = time.time()
    while True:
        if time.time() - start > 30:
            raise AssertionError(
                "Timed out waiting for disabling project monitoring")
        apps = p_client.list_app(name=PROJECT_MONITORING_APP)
        wl1 = p_client.list_workload(name=PROMETHEUS_PROJECT_MONITORING)
        wl2 = p_client.list_workload(name=GRAFANA_PROJECT_MONITORING)
        if len(apps.data) == 0 and len(wl1.data) == 0 and len(wl2.data) == 0:
            break


def validate_project_prometheus(project, token):
    """
    This function deploys a workload which exposes a metrics
    in the target project, and validate if the metrics is scraped
    by the project prometheus.
    """

    cluster = namespace["cluster"]
    project_client = get_project_client_for_token(project, token)

    # deploy a workload to test project monitoring
    cluster_client = get_cluster_client_for_token(cluster, token)
    ns = create_ns(cluster_client, cluster, project, random_name())
    port = {"containerPort": 8080,
            "type": "containerPort",
            "kind": "NodePort",
            "protocol": "TCP"}
    metrics = [{"path": "/metrics",
                "port": 8080,
                "schema": "HTTP"}]
    con = [{"name": "test-web",
            "image": "loganhz/web",
            "ports": [port]}]
    wl_name = random_name()
    workload = project_client.create_workload(name=wl_name,
                                              containers=con,
                                              namespaceId=ns.id,
                                              workloadMetrics=metrics)
    wait_for_wl_to_active(project_client, workload)
    app = project_client.list_app(name=PROJECT_MONITORING_APP).data[0]
    url = CATTLE_TEST_URL + '/k8s/clusters/' + cluster.id \
        + '/api/v1/namespaces/' + app.targetNamespace \
        + '/services/http:access-prometheus:80/proxy/api/v1/' \
        + 'query?query=web_app_online_user_count'
    headers1 = {'Authorization': 'Bearer ' + USER_TOKEN}
    start = time.time()
    while True:
        result = requests.get(headers=headers1, url=url, verify=False).json()
        if len(result["data"]["result"]) > 0:
            project_client.delete(workload)
            return
        if time.time() - start > DEFAULT_MONITORING_TIMEOUT:
            project_client.delete(workload)
            raise AssertionError(
                "Timed out waiting for the graph data available in Prometheus")
        time.sleep(5)


def check_permission_project_graph(project, workload, token, permission=True):
    """
    check if the user has the permission to see graphs in the project

    :param project: the target project where graphs are from
    :param workload:  the target workload in the project
    :param token: the user's token
    :param permission: the user can see graphs if permission is True
    :return: None
    """

    p_id = project["id"]
    client = get_client_for_token(token)
    project_monitoring_obj = client.list_project_monitor_graph(projectId=p_id)
    graphs_list = project_monitoring_obj.get("data")
    if permission:
        assert len(graphs_list) > 0
    else:
        assert len(graphs_list) == 0

    query1 = copy.deepcopy(cluster_query_template)
    query1["obj"] = project_monitoring_obj
    query1["filters"]["projectId"] = p_id
    query1["filters"]["resourceType"] = "workload"
    query1["metricParams"]["workloadName"] = workload.get("id")
    res = client.action(**query1)
    if permission:
        start_time = time.time()
        while time.time() - start_time < DEFAULT_TIMEOUT \
                and "data" not in res.keys():
            time.sleep(10)
            res = client.action(**query1)
        assert "data" in res.keys()
        assert len(res.get("data")) > 0
    else:
        assert "data" not in res.keys()


def check_permission_cluster_graph(cluster, token, permission=True):
    """
    check if the user has the permission to see graphs in the cluster

    :param cluster: the target cluster where graphs are from
    :param token: the user's token
    :param permission: the user can see graphs if permission is True
    :return: None
    """

    c_id = cluster["id"]
    client = get_client_for_token(token)
    cluster_monitoring_obj = client.list_cluster_monitor_graph(clusterId=c_id)
    graphs_list = cluster_monitoring_obj.get("data")
    if permission:
        assert len(graphs_list) > 0
    else:
        assert len(graphs_list) == 0

    query1 = copy.deepcopy(cluster_query_template)
    query1["obj"] = cluster_monitoring_obj
    query1["filters"]["clusterId"] = cluster.id
    query1["filters"]["resourceType"] = "cluster"
    res = client.action(**query1)
    if permission:
        start_time = time.time()
        while time.time() - start_time < DEFAULT_TIMEOUT \
                and "data" not in res.keys():
            time.sleep(10)
            res = client.action(**query1)
        assert "data" in res.keys()
        assert len(res.get("data")) > 0
    else:
        assert "data" not in res.keys()


def validate_project_monitoring(project, token):
    enable_project_monitoring(project, token)
    validate_project_prometheus(project, token)
    disable_project_monitoring(project, token)
