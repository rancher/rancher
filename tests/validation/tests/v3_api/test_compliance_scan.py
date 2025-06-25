import pytest
import os
from .common import USER_TOKEN
from .common import get_cluster_client_for_token_v1
from .common import execute_kubectl_cmd
from .common import get_user_client_and_cluster
from .common import wait_until_app_v2_deployed
from .common import check_v2_app_and_uninstall

COMPLIANCE_CHART_VERSION = os.environ.get('RANCHER_COMPLIANCE_CHART_VERSION', "1.0.100")
SCAN_PROFILE = os.environ.get('RANCHER_SCAN_PROFILE', "profile-sample")

cluster_detail = {"cluster": None}
annotations = \
    {
        "catalog.cattle.io/ui-source-repo": "rancher-charts",
        "catalog.cattle.io/ui-source-repo-type": "cluster"
    }
compliance_charts = {
    "values":
        { "global": {"cattle":{"clusterId": None, "clusterName": None}}},
    "version": COMPLIANCE_CHART_VERSION,
    "projectId": None
}
CHART_NAME = "rancher-compliance"

def test_install_compliance():
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

    # check if Compliance is already installed and uninstall the app
    check_v2_app_and_uninstall(client, CHART_NAME)
    check_v2_app_and_uninstall(client, CHART_NAME + "-crd")

    # create namespace
    ns = "rancher-compliance-system"
    command = "create namespace " + ns
    execute_kubectl_cmd(command, False)

    # install Compliance
    compliance_charts["annotations"] = annotations
    compliance_charts["values"]["global"]["cattle"]["clusterId"] = cluster_id
    compliance_charts["values"]["global"]["cattle"]["clusterName"] = cluster_name
    compliance_charts["chartName"] = CHART_NAME + "-crd"
    compliance_charts["releaseName"] = CHART_NAME + "-crd"

    install_v2_app(client, rancher_repo, compliance_charts, CHART_NAME + "-crd", ns)


    # install app
    compliance_charts["chartName"] = CHART_NAME
    compliance_charts["releaseName"] = CHART_NAME
    install_v2_app(client, rancher_repo, compliance_charts, CHART_NAME, ns)


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
