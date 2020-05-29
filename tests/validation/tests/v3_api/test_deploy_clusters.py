from .common import *  # NOQA
from .test_aks_cluster import get_aks_version, create_and_validate_aks_cluster
from .test_eks_cluster import EKS_K8S_VERSIONS, create_and_validate_eks_cluster
from .test_gke_cluster import get_gke_config, \
    create_and_validate_gke_cluster, get_gke_version_credentials
from .test_rke_cluster_provisioning import create_and_validate_custom_host

KDM_BRANCH = os.environ.get('RANCHER_KDM_BRANCH', "")
KDM_URL = os.environ.get(
    "RANCHER_KDM_URL",
    "https://github.com/rancher/kontainer-driver-metadata.git")

env_details = "env.RANCHER_CLUSTER_NAMES='"

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
    global env_details

    rancher_version = get_setting_value_by_name('server-version')
    if str(rancher_version).startswith('v2.2'):
        k8s_v = get_setting_value_by_name('k8s-version-to-images')
        default_k8s_versions = json.loads(k8s_v).keys()
    else:
        k8s_v = get_setting_value_by_name('k8s-versions-current')
        default_k8s_versions = k8s_v.split(",")

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


@if_not_auto_deploy_eks
def test_deploy_eks():
    print("Deploying EKS Clusters")
    global env_details
    errors = []
    if len(EKS_K8S_VERSIONS) > 1:
        k8s_versions = [EKS_K8S_VERSIONS[0], EKS_K8S_VERSIONS[-1]]
    else:
        k8s_versions = [EKS_K8S_VERSIONS[0]]

    for version in k8s_versions:
        if env_details != "env.RANCHER_CLUSTER_NAMES='":
            env_details += ","
        try:
            print("Deploying EKS Cluster using kubernetes version {}".format(
                version))
            client, cluster = create_and_validate_eks_cluster(version)
            env_details += cluster.name
        except Exception as e:
            errors.append(e)

    assert not errors


@if_not_auto_deploy_gke
def test_deploy_gke():
    print("Deploying GKE Clusters")
    global env_details
    errors = []

    gke_versions, creds = get_gke_version_credentials(multiple_versions=True)

    for i, version in enumerate(gke_versions, start=1):
        c_name = "test-auto-gke-{}".format(i)
        if env_details != "env.RANCHER_CLUSTER_NAMES='":
            env_details += ","
        try:
            print("Deploying GKE Cluster using kubernetes version {}".format(
                version))
            client, cluster = create_and_validate_gke_cluster(c_name,
                                                              version, creds)
            env_details += cluster.name
        except Exception as e:
            errors.append(e)

    assert not errors


@if_not_auto_deploy_aks
def test_deploy_aks():
    print("Deploying AKS Clusters")
    global env_details
    errors = []

    aks_versions = get_aks_version(multiple_versions=True)

    for version in aks_versions:
        if env_details != "env.RANCHER_CLUSTER_NAMES='":
            env_details += ","
        try:
            print("Deploying AKS Cluster using kubernetes version {}".format(
                version))
            client, cluster = create_and_validate_aks_cluster(version)
            env_details += cluster.name
        except Exception as e:
            errors.append(e)

    assert not errors


@pytest.fixture(scope='module', autouse="True")
def set_data(request):
    print("In set_data function")
    if KDM_BRANCH != "":
        print("Updating KDM to use {}/tree/{}".format(KDM_URL, KDM_BRANCH))
        header = {'Authorization': 'Bearer ' + ADMIN_TOKEN}
        kdm_url = CATTLE_API_URL + "/settings/rke-metadata-config"
        kdm_json = {
            "name": "rke-metadata-config",
            "value": json.dumps({
                "branch": KDM_BRANCH,
                "refresh-interval-minutes": "1440",
                "url": KDM_URL
            })
        }
        r = requests.put(kdm_url, verify=False, headers=header, json=kdm_json)
        r_content = json.loads(r.content)
        assert r.ok
        assert r_content['name'] == kdm_json['name']
        assert r_content['value'] == kdm_json['value']

        # Refresh Kubernetes Metadata
        kdm_refresh_url = CATTLE_API_URL + "/kontainerdrivers?action=refresh"
        response = requests.post(kdm_refresh_url, verify=False, headers=header)
        assert response.ok

    def fin():
        global env_details
        env_details += "'"
        print("\n{}".format(env_details))
        create_config_file(env_details)

    request.addfinalizer(fin)
