from .common import *  # NOQA
import pytest

namespace = {"client": None, "ns": None}


def test_daemonset():
    client = namespace["client"]
    ns = namespace["ns"]
    template = read_json_from_resource_dir("daemonset_1.json")
    name = random_name()
    # set name
    template["metadata"]["name"] = name
    # set namespace
    template["metadata"]["namespace"] = ns.id
    # set container image and name
    template["spec"]["template"]["spec"]["containers"][0]["image"] = TEST_IMAGE_V1
    template["spec"]["template"]["spec"]["containers"][0]["name"] = name
    # set label and selector
    label_value = "apps.daemonset-{}-{}".format(ns.id, name)
    labels = template["spec"]["template"]["metadata"]["labels"]
    labels["workload.user.cattle.io/workloadselector"] = label_value
    matches = template["spec"]["selector"]["matchLabels"]
    matches["workload.user.cattle.io/workloadselector"] = label_value

    res = client.create_apps_daemonset(template)
    res = validate_daemonset(client, res)
    client.delete(res)


def get_worker_node(client):
    nodes = client.list_node(
        labelSelector="node-role.kubernetes.io/worker=true")
    return nodes.data


def validate_daemonset(client, daemonset):
    # wait for the deployment to be active
    wait_for(lambda: client.reload(daemonset).metadata.state.name == "active",
             timeout_message="time out waiting for deployment to be ready")
    res = client.reload(daemonset)
    name = res["metadata"]["name"]
    namespace = res["metadata"]["namespace"]
    node_count = len(get_worker_node(client))
    # Rancher Dashboard gets pods by passing the label selector
    label_key = 'workload.user.cattle.io/workloadselector'
    label_value = 'apps.daemonset-{}-{}'.format(namespace, name)
    pods = client.list_pod(
        labelSelector='{}={}'.format(label_key, label_value))
    assert "data" in pods.keys(), "failed to get pods"
    assert len(pods.data) == node_count, "wrong number of pods"
    for pod in pods.data:
        assert label_value == pod.metadata.labels[label_key]
        assert pod.metadata.state.name == "running"
    return res


@pytest.fixture(scope='module', autouse="True")
def create_client(request):
    client = get_cluster_client_for_token_v1()
    template = read_yaml_from_resource_dir("namespace.yaml")
    template["metadata"]["name"] = random_test_name()
    ns = client.create_namespace(template)

    namespace["client"] = client
    namespace["ns"] = ns

    def fin():
        client.delete(namespace["ns"])

    request.addfinalizer(fin)
