from .common import *  # NOQA
import pytest

namespace = {"client": None, "ns": None}


def test_namespace_create():
    template = read_yaml_from_resource_dir("namespace.yaml")
    template["metadata"]["name"] = random_test_name()
    client = namespace["client"]
    res = client.create_namespace(template)
    # validate the namespace is created
    ns = client.by_id_namespace(res.id)
    assert ns.id == res.id
    # delete the namespace at the end
    client.delete(ns)


def test_deployment():
    client = namespace["client"]
    ns = namespace["ns"]
    template = read_json_from_resource_dir("deployment_1.json")
    name = random_name()
    # set name
    template["metadata"]["name"] = name
    # set namespace
    template["metadata"]["namespace"] = ns.id
    # set container image and name
    template["spec"]["template"]["spec"]["containers"][0]["image"] = TEST_IMAGE_V1
    template["spec"]["template"]["spec"]["containers"][0]["name"] = name
    # set label and selector
    label_value = "apps.deployment-{}-{}".format(ns.id, name)
    labels = template["spec"]["template"]["metadata"]["labels"]
    labels["workload.user.cattle.io/workloadselector"] = label_value
    matches = template["spec"]["selector"]["matchLabels"]
    matches["workload.user.cattle.io/workloadselector"] = label_value

    deployment = client.create_apps_deployment(template)
    deployment = validate_deployment(client, deployment)
    # scale up to 5 pods
    deployment.spec.replicas = 5
    deployment = client.update(deployment, deployment)
    deployment = validate_deployment(client, deployment)
    client.delete(deployment)


def validate_deployment(client, deployment):
    # wait for the deployment to be active
    wait_for(lambda: client.reload(deployment).metadata.state.name == "active",
             timeout_message="time out waiting for deployment to be ready")
    res = client.reload(deployment)
    name = res["metadata"]["name"]
    namespace = res["metadata"]["namespace"]
    replicas = res["spec"]["replicas"]
    # Rancher Dashboard gets pods by passing the label selector
    target_label = 'workload.user.cattle.io/workloadselector=apps.deployment-{}-{}'
    pods = client.list_pod(
        labelSelector=target_label.format(namespace, name))
    assert "data" in pods.keys(), "failed to get pods"
    assert len(pods.data) == replicas, "failed to get the right number of pods"
    for pod in pods.data:
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
