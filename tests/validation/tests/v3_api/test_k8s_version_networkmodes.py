from lib.aws import AmazonWebServices

from .common import *  # NOQA

k8s_version = "v1.10.1-rancher1"
rke_config = {"authentication": {"type": "authnConfig", "strategy": "x509"},
              "ignoreDockerVersion": False,
              "type": "rancherKubernetesEngineConfig"}
RANCHER_CLEANUP_CLUSTER = os.environ.get('RANCHER_CLEANUP_CLUSTER', "True")
NETWORK_PLUGIN = os.environ.get('NETWORK_PLUGIN', "canal")


def test_rke_custom_k8s_1_8_10():
    validate_k8s_version("v1.8.10-rancher1-1", plugin=NETWORK_PLUGIN)


def test_rke_custom_k8s_1_8_11():
    validate_k8s_version("v1.8.11-rancher1", plugin=NETWORK_PLUGIN)


def test_rke_custom_k8s_1_9_5():
    validate_k8s_version("v1.9.5-rancher1-1", plugin=NETWORK_PLUGIN)


def test_rke_custom_k8s_1_9_7():
    validate_k8s_version("v1.9.7-rancher1", plugin=NETWORK_PLUGIN)


def test_rke_custom_k8s_1_10_0():
    validate_k8s_version("v1.10.0-rancher1-1", plugin=NETWORK_PLUGIN)


def test_rke_custom_k8s_1_10_1():
    validate_k8s_version("v1.10.1-rancher1", plugin=NETWORK_PLUGIN)


def validate_k8s_version(k8s_version, plugin="canal"):
    rke_config["kubernetesVersion"] = k8s_version
    rke_config["network"] = {"type": "networkConfig", "plugin": plugin}
    aws_nodes = \
        AmazonWebServices().create_multiple_nodes(
            8, random_test_name("testcustom"))
    node_roles = [["controlplane"], ["controlplane"],
                  ["etcd"], ["etcd"], ["etcd"],
                  ["worker"], ["worker"], ["worker"]]
    client = get_user_client()
    cluster = client.create_cluster(name=random_name(),
                                    driver="rancherKubernetesEngine",
                                    rancherKubernetesEngineConfig=rke_config)
    assert cluster.state == "active"
    i = 0
    for aws_node in aws_nodes:
        docker_run_cmd = \
            get_custom_host_registration_cmd(client, cluster,
                                             node_roles[i], aws_node)
        aws_node.execute_command(docker_run_cmd)
        i += 1
    cluster = validate_cluster(client, cluster)
    if RANCHER_CLEANUP_CLUSTER == "True":
        delete_cluster(client, cluster)
        delete_node(aws_nodes)


def delete_node(aws_nodes):
    for node in aws_nodes:
        AmazonWebServices().delete_node(node)
