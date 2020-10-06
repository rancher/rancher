import pytest
import os
from .common import USER_TOKEN
from .common import get_cluster_client_for_token_v1
from .common import execute_kubectl_cmd
from .common import get_user_client_and_cluster
from .common import wait_until_app_v2_deployed
from .common import check_v2_app_and_uninstall

CIS_CHART_VERSION = os.environ.get('RANCHER_CIS_CHART_VERSION', "1.0.100")
SCAN_PROFILE = os.environ.get('RANCHER_SCAN_PROFILE', "rke-profile-permissive")

cluster_detail = {"cluster": None}
cis_annotations = \
    {
        "catalog.cattle.io/ui-source-repo": "rancher-charts",
        "catalog.cattle.io/ui-source-repo-type": "cluster"
    }
cis_charts = {
    "values":
        { "global": {"cattle":{"clusterId": None, "clusterName": None}}},
    "version": CIS_CHART_VERSION,
    "projectId": None
}
CHART_NAME = "rancher-cis-benchmark"

def test_install_v2_cis_benchmark():
    """
    List installed apps
    Check if the app is installed
    If installed, delete the app and the CRDs
    Create namespace
    Install App and the CRDs
    :return:
    """
    client = \
        get_cluster_client_for_token_v1(
            cluster_detail["cluster"]["id"],USER_TOKEN
        )
    rancherrepo = \
        client.list_catalog_cattle_io_clusterrepo(id="rancher-charts")
    cluster_id = cluster_detail["cluster"]["id"]
    cluster_name = cluster_detail["cluster"]["name"]
    rancher_repo = rancherrepo["data"][0]

    # check if CIS is already installed and uninstall the app
    check_v2_app_and_uninstall(client, CHART_NAME)
    check_v2_app_and_uninstall(client, CHART_NAME + "-crd")

    # create namespace
    ns = "cis-operator-system"
    command = "create namespace " + ns
    execute_kubectl_cmd(command, False)

    # install CIS v2
    cis_charts["annotations"] = cis_annotations
    cis_charts["values"]["global"]["cattle"]["clusterId"] = cluster_id
    cis_charts["values"]["global"]["cattle"]["clusterName"] = cluster_name
    cis_charts["chartName"] = CHART_NAME + "-crd"
    cis_charts["releaseName"] = CHART_NAME + "-crd"

    install_v2_app(client, rancher_repo, cis_charts, CHART_NAME + "-crd", ns)


    # install app
    cis_charts["chartName"] = CHART_NAME
    cis_charts["releaseName"] = CHART_NAME
    install_v2_app(client, rancher_repo, cis_charts, CHART_NAME, ns)


@pytest.fixture(scope='module', autouse="True")
def create_project_client(request):
    client, cluster_detail["cluster"] = get_user_client_and_cluster()


def install_v2_app(client, rancher_repo, chart_values, chart_name, ns):
    # install CRD
    response = client.action(obj=rancher_repo, action_name="install",
                             charts=[chart_values],
                             namespace=ns,
                             disableOpenAPIValidation=False,
                             noHooks=False,
                             projectId=None,
                             skipCRDs=False,
                             timeout="600s",
                             wait=True)
    print("response", response)
    app_list = wait_until_app_v2_deployed(client, chart_name)
    assert chart_name in app_list
