import pytest
import requests
import time
from .common import random_test_name
from .common import get_admin_client_and_cluster
from .common import create_project_and_ns
from .common import get_project_client_for_token
from .common import create_kubeconfig
from .common import get_admin_client
from .common import wait_for_condition
from .common import assign_members_to_project
from .common import wait_for_wl_to_active
from .common import CATTLE_TEST_URL
from .common import ADMIN_TOKEN
from .test_app import check_condition
from .test_rbac import create_user

namespace = {"p_client": None, "admin_client": None, "cluster": None,
             "project": None, "ns": None,
             "user1": None, "user1_token": None, "b_project": None, "workload": None,
             "user2": None, "user2_token": None}

CLUSTER_UEL = \
    CATTLE_TEST_URL + "/v3/clusters"
ClUSTER_MONITOR_GRAPH_QUERY_URL = \
    CATTLE_TEST_URL + "/v3/clustermonitorgraphs?action=query"
PROJECT_MONITOR_GRAPH_QUERY_URL = \
    CATTLE_TEST_URL + "/v3/projectmonitorgraphs?action=query"

enable_monitoring_action = "enableMonitoring"
disable_monitoring_action = "disableMonitoring"

answer = {
    "answers": {
        "exporter-node.enabled": "true",
        "exporter-node.ports.metrics.port": "9796",
        "exporter-kubelets.https": "true",
        "exporter-node.resources.limits.cpu": "200m",
        "exporter-node.resources.limits.memory": "200Mi",
        "operator.resources.limits.memory": "500Mi",
        "prometheus.retention": "12h",
        "grafana.persistence.enabled": "false",
        "prometheus.persistence.enabled": "false",
        "prometheus.persistence.storageClass": "default",
        "grafana.persistence.storageClass": "default",
        "grafana.persistence.size": "10Gi",
        "prometheus.persistence.size": "50Gi",
        "prometheus.resources.core.requests.cpu": "500m",
        "prometheus.resources.core.limits.cpu": "1000m",
        "prometheus.resources.core.requests.memory": "500Mi",
        "prometheus.resources.core.limits.memory": "1000Mi",
        "prometheus.persistent.useReleaseName": "true"
    }
}

container_name = "one"

def test_query_graph(setup_monitoring):
    cluster = namespace["cluster"]
    project = namespace["project"]
    admin_client = namespace["admin_client"]
    p_client = namespace["p_client"]
    user1_token = namespace["user1_token"]
    user2_token = namespace["user2_token"]

    # cluster
    # valid params
    # case 1: valid cluster resource params
    cluster_metric_params = {}
    cluster_filter = {"clusterId": cluster.id, "resourceType": "cluster"}
    query_graph(ADMIN_TOKEN, ClUSTER_MONITOR_GRAPH_QUERY_URL, cluster_filter, cluster_metric_params, expected_status=200)

    # case 2: valid etcd resource params
    etcd_filter = {"clusterId": cluster.id, "resourceType": "etcd"}
    query_graph(ADMIN_TOKEN, ClUSTER_MONITOR_GRAPH_QUERY_URL, etcd_filter, cluster_metric_params, expected_status=200)

    # case 3: valid kube component resource params
    kube_component_filter = {"clusterId": cluster.id, "displayResourceType": "kube-component"}
    query_graph(ADMIN_TOKEN, ClUSTER_MONITOR_GRAPH_QUERY_URL, kube_component_filter, cluster_metric_params, expected_status=200)

    # case 4: valid node params
    node_filter = {"clusterId": cluster.id, "resourceType": "node"}
    nodes = admin_client.list_node(clusterId=cluster.id).data
    assert len(nodes) > 0
    node_metric_params = {"instance": nodes[0].id}
    query_graph(ADMIN_TOKEN, ClUSTER_MONITOR_GRAPH_QUERY_URL, node_filter, node_metric_params, expected_status=200)

    # invalid params
    # case 5: lack of param clusterId
    invalid_cluster_filter = {"resourceType": "cluster"}
    query_graph(ADMIN_TOKEN, ClUSTER_MONITOR_GRAPH_QUERY_URL, invalid_cluster_filter, cluster_metric_params, expected_status=500)

    # case 6: invalid resourceType
    invalid_cluster_filter = {"clusterId": cluster.id, "resourceType": "fake_type"}
    query_graph(ADMIN_TOKEN, ClUSTER_MONITOR_GRAPH_QUERY_URL, invalid_cluster_filter, cluster_metric_params, expected_status=500)

    # case 7: not exist node
    node_metric_params = {"instance": "fake_node"}
    query_graph(ADMIN_TOKEN, ClUSTER_MONITOR_GRAPH_QUERY_URL, node_filter, node_metric_params, expected_status=500)

    # case 8: unauth query
    query_graph(user2_token, ClUSTER_MONITOR_GRAPH_QUERY_URL, cluster_filter, cluster_filter, expected_status=403)

    # project
    # case 1: valid workload resource params
    workload = namespace["workload"]
    workload_filter = {"projectId": project.id, "resourceType": "workload"}
    workload_metric_params = {"workloadName": workload.id}
    query_graph(ADMIN_TOKEN, PROJECT_MONITOR_GRAPH_QUERY_URL, workload_filter, workload_metric_params, expected_status=200)

    # case 2: valid pod resource params
    pod_list = p_client.list_pod(workloadId=workload.id).data
    assert len(pod_list) > 0
    pod = pod_list[0]
    pod_filter = {"projectId": project.id, "resourceType": "pod"}
    pod_metric_params = {"podName": pod.id}
    query_graph(ADMIN_TOKEN, PROJECT_MONITOR_GRAPH_QUERY_URL, pod_filter, pod_metric_params, expected_status=200)

    # case 3: valid container resource params
    container_filter = {"projectId": project.id, "resourceType": "container"}
    container_metric_params = {"containerName": container_name, "podName": pod.id}
    query_graph(ADMIN_TOKEN, PROJECT_MONITOR_GRAPH_QUERY_URL, container_filter, container_metric_params, expected_status=200)

    # invalid params
    # case 4: lack of projectId
    invalid_project_filter = {"resourceType": "workload"}
    query_graph(ADMIN_TOKEN, PROJECT_MONITOR_GRAPH_QUERY_URL, invalid_project_filter, workload_metric_params, expected_status=500)

    # case 5: invalid resourceType
    invalid_project_filter = {"projectId": project.id, "resourceType": "fake_type"}
    query_graph(ADMIN_TOKEN, PROJECT_MONITOR_GRAPH_QUERY_URL, invalid_project_filter, invalid_project_filter, expected_status=500)

    # case 6: use b_project token to query a_project metric
    b_project = namespace["b_project"]
    b_ns = namespace["b_ns"]
    ns = namespace["ns"]
    query_graph(user1_token, PROJECT_MONITOR_GRAPH_QUERY_URL, workload_filter, workload_metric_params, expected_status=403)

    # case 7: use token, project_id in b_project and workload_name in a_project to query metric
    workload_filter = {"projectId": b_project.id, "resourceType": "workload"}
    workload_metric_params = {"workloadName": workload.id}
    query_graph(user1_token, PROJECT_MONITOR_GRAPH_QUERY_URL, workload_filter, workload_metric_params, expected_status=403)


def query_graph(token, url, filters, metric_params, expected_status=204):
    from_time = "now-1h"
    to_time = "now"
    interval = "5s"
    is_details = True
    body = {"from": from_time,
            "to": to_time,
            "interval": interval,
            "is_details": is_details,
            "filters": filters,
            "metricParams": metric_params}
    r = send_request(token, url, body)
    if r.status_code != expected_status:
        raise AssertionError("status code " + str(r.status_code) + " not equal to expected status " + str(expected_status) + ", response body: " + str(r.content))
    if r.status_code == 200:
        if "data" not in r.json():
            raise AssertionError("response body doesn't include data field, response body: " + str(r.json()))
        data = r.json()['data']
        assert len(data) > 0


def operate_cluster_monitoring(action_name, expected_status=204):
    cluster = namespace["cluster"]
    url = CLUSTER_UEL + "/" + cluster.id + "?action=" + action_name
    r = send_request(ADMIN_TOKEN, url, answer)
    if r.status_code != expected_status:
        raise AssertionError("status code " + str(r.status_code) + " not equal to expected status " + str(expected_status) + ", response body: " + str(r.content))


def send_request(token, url, body):
    headers = {'Authorization': 'Bearer ' + token}
    r = requests.post(url,
                      json=body,
                      verify=False, headers=headers)
    return r


def enable_cluster_monitoring():
    cluster = namespace["cluster"]
    admin_client = namespace["admin_client"]
    operate_cluster_monitoring(enable_monitoring_action)
    wait_for_condition(admin_client, cluster, check_condition('MonitoringEnabled', 'True'))


def create_project_and_user():
    admin_client = namespace["admin_client"]
    cluster = namespace["cluster"]

    b_p, b_ns = create_project_and_ns(ADMIN_TOKEN, cluster, random_test_name("testlogging"))
    user1, user1_token = create_user(admin_client)

    prtb = assign_members_to_project(
        admin_client, user1, b_p, "project-owner")

    user2, user2_token = create_user(admin_client)

    namespace["b_project"] = b_p
    namespace["b_ns"] = b_ns
    namespace["user1"] = user1
    namespace["user1_token"] = user1_token
    namespace["user2"] = user2
    namespace["user2_token"] = user2_token

def create_project_workload():
    p_client = namespace["p_client"]
    ns = namespace["ns"]
    workload_name = random_test_name("myapp")
    workload = p_client.create_workload(name=workload_name,
                                        containers=[{
                                            "name": container_name,
                                            "image": "nginx",
                                        }],
                                        namespaceId=ns.id)
    wait_for_wl_to_active(p_client, workload, timeout=90)
    return workload


@pytest.fixture(scope='function')
def setup_monitoring(request):
    def disable_cluster_monitoring():
        admin_client = get_admin_client()
        operate_cluster_monitoring(disable_monitoring_action)

    def clean_b_project_and_user():
        client = get_admin_client()
        client.delete(namespace["b_project"])
        client.delete(namespace["user1"])
        client.delete(namespace["user2"])

    request.addfinalizer(disable_cluster_monitoring)
    request.addfinalizer(clean_b_project_and_user)

    enable_cluster_monitoring()
    create_project_and_user()
    workload = create_project_workload()
    namespace["workload"] = workload
    # sleep 240 seconds to wait for scrape data
    time.sleep(240)


@pytest.fixture(scope='module', autouse="True")
def create_project_client(request):
    client, cluster = get_admin_client_and_cluster()
    create_kubeconfig(cluster)
    p, ns = create_project_and_ns(ADMIN_TOKEN, cluster,
                                  random_test_name("testlogging"))
    p_client = get_project_client_for_token(p, ADMIN_TOKEN)
    namespace["p_client"] = p_client
    namespace["ns"] = ns
    namespace["cluster"] = cluster
    namespace["project"] = p
    namespace["admin_client"] = client

    def fin():
        client = get_admin_client()
        client.delete(namespace["project"])
    request.addfinalizer(fin)
