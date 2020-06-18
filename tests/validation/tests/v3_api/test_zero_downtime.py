from .common import *
import pytest
import time

from .test_rke_cluster_provisioning import create_and_validate_custom_host

CLUSTER_NAME = os.environ.get("RANCHER_CLUSTER_NAME", "")
cluster_name = CLUSTER_NAME
namespace = {"p_client": None, "ns": None, "cluster": None, "project": None,
             "testclient_pods": []}
catalogUrl = "https://github.com/rancher/integration-test-charts.git"
catalogBranch = "validation-tests"
env_details = "env.RANCHER_CLUSTER_NAMES='"

#@pytest.fixture(scope='module', autouse="True")
def test_deploy_rke():
    print("Deploying RKE Clusters")
    global env_details
    rancher_version = get_setting_value_by_name('server-version')
    print("rancher version:", rancher_version)
    if str(rancher_version).startswith('v2.2'):
        k8s_v = get_setting_value_by_name('k8s-version-to-images')
        default_k8s_versions = json.loads(k8s_v).keys()
    else:
        k8s_v = get_setting_value_by_name('k8s-versions-current')
        default_k8s_versions = k8s_v.split(",")
    print(default_k8s_versions)
    # Create clusters
    for k8s_version in default_k8s_versions:
        if env_details != "env.RANCHER_CLUSTER_NAMES='":
            env_details += ","
        print("Deploying RKE Cluster using kubernetes version {}".format(
            k8s_version))
        node_roles = [["controlplane"], ["etcd"],
                      ["worker"], ["worker"], ["worker"]]
        cluster, aws_nodes = create_and_validate_custom_host(
            node_roles, random_cluster_name=True, version=k8s_version)
        env_details += cluster.name
        print("Successfully deployed {} with kubernetes version {}".format(
            cluster.name, k8s_version))

