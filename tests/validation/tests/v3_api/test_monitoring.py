import pytest
import copy
from .common import *  # NOQA

namespace = {
    "cluster": None,
    "project": None,
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

CLUSTER_MONITORING_APP = "cluster-monitoring"
MONITORING_OPERATOR_APP = "monitoring-operator"
PROJECT_MONITORING_APP = "project-monitoring"


def test_monitoring_cluster_graph():
    rancher_client, cluster = get_admin_client_and_cluster()
    cluster_monitor_obj = rancher_client.list_clusterMonitorGraph()
    # generate the request payload
    query1 = copy.deepcopy(cluster_query_template)
    query1["obj"] = cluster_monitor_obj
    query1["filters"]["clusterId"] = cluster.id
    query1["filters"]["resourceType"] = "cluster"
    validate_cluster_graph(query1, "cluster")


def test_monitoring_etcd_graph():
    rancher_client, cluster = get_admin_client_and_cluster()
    cluster_monitor_obj = rancher_client.list_clusterMonitorGraph()
    # generate the request payload
    query1 = copy.deepcopy(cluster_query_template)
    query1["obj"] = cluster_monitor_obj
    query1["filters"]["clusterId"] = cluster.id
    query1["filters"]["resourceType"] = "etcd"
    validate_cluster_graph(query1, "etcd")


def test_monitoring_kube_component_graph():
    rancher_client, cluster = get_admin_client_and_cluster()
    cluster_monitor_obj = rancher_client.list_clusterMonitorGraph()
    # generate the request payload
    query1 = copy.deepcopy(cluster_query_template)
    query1["obj"] = cluster_monitor_obj
    query1["filters"]["clusterId"] = cluster.id
    query1["filters"]["displayResourceType"] = "kube-component"
    validate_cluster_graph(query1, "kube-component")


# rancher component graphs are from the fluent app for cluster logging
def test_monitoring_rancher_component_graph():
    rancher_client, cluster = get_admin_client_and_cluster()
    # check if the cluster logging is enabled, assuming fluent is used
    if cluster.enableClusterAlerting is False:
        print("cluster logging is not enabled, skip the test")
        return
    else:
        cluster_monitor_obj = rancher_client.list_clusterMonitorGraph()
        # generate the request payload
        query1 = copy.deepcopy(cluster_query_template)
        query1["obj"] = cluster_monitor_obj
        query1["filters"]["clusterId"] = cluster.id
        query1["filters"]["displayResourceType"] = "rancher-component"
        validate_cluster_graph(query1, "rancher-component")


def test_monitoring_node_graph():
    rancher_client, cluster = get_admin_client_and_cluster()
    node_list_raw = rancher_client.list_node(clusterId=cluster.id).data
    for node in node_list_raw:
        cluster_monitor_obj = rancher_client.list_clusterMonitorGraph()
        # generate the request payload
        query1 = copy.deepcopy(cluster_query_template)
        query1["obj"] = cluster_monitor_obj
        query1["filters"]["clusterId"] = cluster.id
        query1["filters"]["resourceType"] = "node"
        query1["metricParams"]["instance"] = node.id
        validate_cluster_graph(query1, "node")


def test_monitoring_workload_graph():
    rancher_client, cluster = get_admin_client_and_cluster()
    system_project = rancher_client.list_project(clusterId=cluster.id, name="System").data[0]
    project_monitor_obj = rancher_client.list_projectMonitorGraph()
    # generate the request payload
    query1 = copy.deepcopy(cluster_query_template)
    query1["obj"] = project_monitor_obj
    query1["filters"]["projectId"] = system_project.id
    query1["filters"]["resourceType"] = "workload"
    query1["metricParams"]["workloadName"] = \
        "deployment:cattle-prometheus:grafana-cluster-monitoring"
    validate_cluster_graph(query1, "workload")


def test_monitoring_project_monitoring():
    rancher_client, cluster = get_admin_client_and_cluster()
    system_project = rancher_client.list_project(name="System").data[0]
    system_project_client = get_project_client_for_token(system_project, ADMIN_TOKEN)
    project = namespace["project"]
    # enable the project monitoring
    project.enableMonitoring()
    project_client = get_project_client_for_token(project, ADMIN_TOKEN)
    namespace["project"] = rancher_client.reload(project)
    wait_for_app_to_active(project_client, PROJECT_MONITORING_APP)
    wait_for_app_to_active(system_project_client, MONITORING_OPERATOR_APP)
    namespace["project"] = rancher_client.reload(project)
    # wait for targets to be up
    wait_for_target_up("expose-prometheus-metrics")
    wait_for_target_up("expose-grafana-metrics")
    # deploy a workload to test project monitoring
    cluster_client = get_cluster_client_for_token(cluster, ADMIN_TOKEN)
    ns = create_ns(cluster_client, cluster, project, random_name())
    port = {"containerPort": 8080,
            "type": "containerPort",
            "kind": "NodePort",
            "protocol": "TCP"}
    workloadMetrics = [{"path": "/metrics",
                        "port": 8080,
                        "schema": "HTTP"}]
    con = [{"name": "test-web",
            "image": "loganhz/web",
            "ports": [port]}]
    wl_name = random_name()
    workload = project_client.create_workload(name=wl_name,
                                              containers=con,
                                              namespaceId=ns.id,
                                              workloadMetrics=workloadMetrics)
    wait_for_wl_to_active(project_client, workload)
    app = project_client.list_app(name=PROJECT_MONITORING_APP).data[0]
    url = CATTLE_TEST_URL + '/k8s/clusters/' + cluster.id + '/api/v1/namespaces/' + app.targetNamespace \
          + '/services/http:access-prometheus:80/proxy/api/v1/query?query=web_app_online_user_count'
    headers1 = {'Authorization': 'Bearer ' + ADMIN_TOKEN}
    start = time.time()
    while True:
        result = requests.get(headers=headers1, url=url, verify=False).json()
        if len(result["data"]["result"]) > 0:
            return
        if time.time() - start > DEFAULT_MONITORING_TIMEOUT:
            raise AssertionError(
                "Timed out waiting for the graph data is available in Prometheus")
        time.sleep(5)


@pytest.fixture(scope="module", autouse="True")
def create_project_client(request):
    rancher_client, cluster = get_admin_client_and_cluster()
    create_kubeconfig(cluster)
    project = create_project(rancher_client, cluster,
                             random_test_name("p-monitoring"))
    namespace["cluster"] = cluster
    namespace["project"] = project

    system_project = rancher_client.list_project(clusterId=cluster.id, name="System").data[0]
    system_project_client = get_project_client_for_token(system_project, ADMIN_TOKEN)
    # enable the cluster monitoring
    if cluster["enableClusterMonitoring"] is False:
        cluster.enableMonitoring()
    wait_for_app_to_active(system_project_client, CLUSTER_MONITORING_APP)
    wait_for_app_to_active(system_project_client, MONITORING_OPERATOR_APP)
    # wait 2 minutes for all graphs to be available
    time.sleep(60*2)

    def fin():
        rancher_client.delete(project)
        cluster2 = rancher_client.reload(cluster)
        if "disableMonitoring" in cluster2.actions:
            cluster2.disableMonitoring()

    request.addfinalizer(fin)


def get_graph_data(query1, list_len, timeout=10):
    rancher_client, cluster = get_admin_client_and_cluster()
    start = time.time()
    while True:
        res = rancher_client.action(**query1)
        if hasattr(res, "data") is True:
            print("\nthe length is ", len(res.data), "expected to be: ", list_len)
            print(res.data)
            if len(res.data) == list_len:
                # check if all graphs have valid data points
                for graph in res.data:
                    if hasattr(graph, "series"):
                        if len(graph.series) == 0:
                            continue
                return res.data
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for all graphs to be available")
        time.sleep(2)


def validate_cluster_graph(action_query, resource_type):
    target_graph_list = copy.deepcopy(name_mapping.get(resource_type))
    graphs = get_graph_data(action_query, len(target_graph_list))
    for item in graphs:
        name = item.graphID.split(":")[1]
        assert name is not None, " [cluster monitoring] invalid graph name"
        assert name in target_graph_list, \
            "[cluster monitoring] the graph {} is missing".format(name)
        target_graph_list.remove(name)
    assert len(target_graph_list) == 0, target_graph_list


def wait_for_target_up(job):
    _, cluster = get_admin_client_and_cluster()
    project_client = get_project_client_for_token(namespace["project"], ADMIN_TOKEN)
    app = project_client.list_app(name=PROJECT_MONITORING_APP).data[0]
    url = CATTLE_TEST_URL + '/k8s/clusters/' + cluster.id + '/api/v1/namespaces/' + app.targetNamespace \
          + '/services/http:access-prometheus:80/proxy/api/v1/targets'
    headers1 = {'Authorization': 'Bearer ' + ADMIN_TOKEN}
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

