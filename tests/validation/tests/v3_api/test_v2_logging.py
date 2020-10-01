import pytest
import os
from .common import USER_TOKEN
from .common import check_v2_app_and_uninstall
from .common import create_kubeconfig
from .common import execute_kubectl_cmd
from .common import get_cluster_client_for_token_v1
from .common import get_user_client_and_cluster
from .common import install_v2_app


LOGGING_CHART_VERSION = os.environ.get('RANCHER_LOGGING_CHART_VERSION', "3.6.000")

cluster_detail = {"cluster": None}
logging_annotations = \
    {
        "catalog.cattle.io/ui-source-repo": "rancher-charts",
        "catalog.cattle.io/ui-source-repo-type": "cluster"
    }
logging_charts = {
    "values":
        {
            "global": {"cattle": {"clusterId": None, "clusterName": None}},
            "additionalLoggingSources": None,
         },
    "version": LOGGING_CHART_VERSION,
    "projectId": None
}
CHART_NAME = "rancher-logging"


def test_v2_logging_install():
    """
    List installed apps
    Check if the app is installed
    If installed, delete the app and the CRDs
    Create namespace
    Install CRDs and the App
    :return:
    """
    client = \
        get_cluster_client_for_token_v1(
            cluster_detail["cluster"]["id"], USER_TOKEN
        )
    rancherrepo = \
        client.list_catalog_cattle_io_clusterrepo(id="rancher-charts")
    cluster_id = cluster_detail["cluster"]["id"]
    cluster_name = cluster_detail["cluster"]["name"]
    cluster_provider = cluster_detail["cluster"]["provider"]
    rancher_repo = rancherrepo["data"][0]

    # check if logging v2 is already installed and uninstall the app
    check_v2_app_and_uninstall(client, CHART_NAME)
    check_v2_app_and_uninstall(client, CHART_NAME + "-crd")

    # create namespace
    ns = "cattle-logging-system"
    command = "create namespace " + ns
    execute_kubectl_cmd(command, False)

    # install logging v2 crd
    logging_charts["annotations"] = logging_annotations
    logging_charts["values"]["global"]["cattle"]["clusterId"] = cluster_id
    logging_charts["values"]["global"]["cattle"]["clusterName"] = cluster_name
    logging_charts["chartName"] = CHART_NAME + "-crd"
    logging_charts["releaseName"] = CHART_NAME + "-crd"

    install_v2_app(client, rancher_repo, logging_charts, CHART_NAME + "-crd", ns)

    # install logging v2 app
    logging_charts["chartName"] = CHART_NAME
    logging_charts["releaseName"] = CHART_NAME
    logging_charts["values"]["additionalLoggingSources"] = {cluster_provider: {"enabled": True}}
    install_v2_app(client, rancher_repo, logging_charts, CHART_NAME, ns)


@pytest.fixture(scope='module', autouse="True")
def create_project_client(request):
    client, cluster_detail["cluster"] = get_user_client_and_cluster()
    create_kubeconfig(cluster_detail["cluster"])

    def fin():
        client = \
            get_cluster_client_for_token_v1(
                cluster_detail["cluster"]["id"], USER_TOKEN
            )
        check_v2_app_and_uninstall(client, CHART_NAME)
        check_v2_app_and_uninstall(client, CHART_NAME + "-crd")
        ns = "cattle-logging-system"
        command = "delete namespace " + ns
        execute_kubectl_cmd(command, False)
    request.addfinalizer(fin)
