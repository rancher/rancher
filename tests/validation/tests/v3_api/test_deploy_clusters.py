from .common import *  # NOQA
from .test_rke_cluster_provisioning import create_and_validate_custom_host

if_not_auto_deploy_rke = pytest.mark.skipif(
    ast.literal_eval(
        os.environ.get(
            'RANCHER_TEST_DEPLOY_RKE', "False")) is False,
    reason='auto deploy RKE tests are skipped')
if_not_auto_deploy_eks = pytest.mark.skipif(
    ast.literal_eval(
        os.environ.get(
            'RANCHER_TEST_DEPLOY_EKS', "False")) is False,
    reason='auto deploy EKS tests are skipped')
if_not_auto_deploy_gke = pytest.mark.skipif(
    ast.literal_eval(
        os.environ.get(
            'RANCHER_TEST_DEPLOY_GKE', "False")) is False,
    reason='auto deploy GKE tests are skipped')
if_not_auto_deploy_aks = pytest.mark.skipif(
    ast.literal_eval(
        os.environ.get(
            'RANCHER_TEST_DEPLOY_AKS', "False")) is False,
    reason='auto deploy AKS tests are skipped')


@if_not_auto_deploy_rke
def test_deploy_rke():
    print("Deploying RKE Clusters")

    # TODO: Change kdm if env variable supplied (in module fixture)

    rancher_version = get_setting_value_by_name('server-version')
    if str(rancher_version).startswith('v2.2'):
        k8s_v = get_setting_value_by_name('k8s-version-to-images')
        default_k8s_versions = json.loads(k8s_v).keys()
    else:
        k8s_v = get_setting_value_by_name('k8s-versions-current')
        default_k8s_versions = k8s_v.split(",")

    # Create clusters
    env_details = "env.CLUSTER_NAMES='"
    for k8s_version in default_k8s_versions:
        if env_details != "env.CLUSTER_NAMES='":
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
    env_details += "'"
    create_config_file(env_details)


@if_not_auto_deploy_eks
def test_deploy_eks():
    print("Deploying EKS Clusters")


@if_not_auto_deploy_gke
def test_deploy_gke():
    print("Deploying GKE Clusters")


@if_not_auto_deploy_aks
def test_deploy_aks():
    print("Deploying AKS Clusters")
