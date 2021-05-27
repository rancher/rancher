import os
import pytest

from python_terraform import * # NOQA
from ast import literal_eval
from .common import CATTLE_TEST_URL
from .common import CLUSTER_NAME
from .common import DATA_SUBDIR
from .common import USER_TOKEN
from .common import create_project
from .common import get_project_client_for_token
from .common import get_user_client_and_cluster
from .common import wait_for_wl_to_active
from ..v1_api.test_monitoring_v2 import get_chart_latest_version
from ..v1_api.common import get_cluster_client_for_token_v1
from ..v1_api.common import get_admin_client_v1
from ..v1_api.common import get_cluster_by_name


monitoring_version = os.environ.get('RANCHER_MONITORING_V2_VERSION', '')
logging_version = os.environ.get('RANCHER_LOGGING_V2_VERSION', '')
kiali_version = os.environ.get('RANCHER_KIALI_V2_VERSION', '')
istio_version = os.environ.get('RANCHER_ISTIO_V2_VERSION', '')
cis_version = os.environ.get('RANCHER_CIS_V2_VERSION', '')
remove_charts = literal_eval(
    os.environ.get('RANCHER_REMOVE_V2_CHARTS', 'False'))
tf = Terraform()


def test_install_charts_v2(precheck_install_versions):
    client, cluster = get_user_client_and_cluster()
    project_name = 'chartsv2'
    cluster_provider = cluster['provider']
    p = create_project(client, cluster, project_name)
    project_id = p['id']
    cluster_id = cluster['id']
    tf_dir = DATA_SUBDIR + "/" + "terraform/charts_v2"
    tf.working_dir = tf_dir
    tf.variables = {'cluster_id': cluster_id,
                    'cluster_provider': cluster_provider,
                    'project_id': project_id,
                    'api_url': CATTLE_TEST_URL,
                    'token_key': USER_TOKEN,
                    'monitoring_version': monitoring_version,
                    'logging_version': logging_version,
                    'kiali_version': kiali_version,
                    'istio_version': istio_version,
                    'cis_version': cis_version,
                    'values_path': tf_dir}

    print("installing charts")
    tf.init()
    print(tf.plan(out="charts_plan.out"))
    print("\n\n")
    print(tf.apply("--auto-approve"))
    print("\n\n")
    check_charts_project_workloads(client, cluster, project_name=project_name)


def check_charts_project_workloads(client, cluster, project_name=''):
    charts_project = client.list_project(name=project_name,
                                         clusterId=cluster.id).data[0]
    charts_p_client = get_project_client_for_token(charts_project, USER_TOKEN)
    for wl in charts_p_client.list_workload().data:
        wait_for_wl_to_active(charts_p_client, wl, timeout=300)
    check_workloads_exist(charts_p_client.list_workload().data)


def check_workloads_exist(workloads_data):
    workloads_names = ('rancher-logging',
                       'rancher-monitoring-operator',
                       'cis-operator',
                       'istiod'
                       )
    active_names = [wl_name['name'] for wl_name in workloads_data]
    print(active_names)
    for name in workloads_names:
        assert name in active_names


@pytest.fixture(scope='module')
def precheck_install_versions(request):
    global monitoring_version
    global logging_version
    global kiali_version
    global istio_version
    global cis_version
    namespace = {}
    client = get_cluster_client_for_token_v1()
    admin_client = get_admin_client_v1()
    cluster = get_cluster_by_name(admin_client, CLUSTER_NAME)
    namespace["client"] = client
    namespace["cluster"] = cluster
    rancher_catalog = \
        client.by_id_catalog_cattle_io_clusterrepo(id="rancher-charts")
    if monitoring_version == '' or monitoring_version is None:
        monitoring_version = get_chart_latest_version(rancher_catalog, "rancher-monitoring")
    if logging_version == '' or logging_version is None:
        logging_version = get_chart_latest_version(rancher_catalog, "rancher-logging")
    if kiali_version == '' or kiali_version is None:
        kiali_version = get_chart_latest_version(rancher_catalog, "rancher-kiali-server-crd")
    if istio_version == '' or istio_version is None:
        istio_version = get_chart_latest_version(rancher_catalog, "rancher-istio")
    if cis_version == '' or cis_version is None:
        cis_version = get_chart_latest_version(rancher_catalog, "rancher-cis-benchmark")

    def fin():
        if remove_charts:
            tf_dir = DATA_SUBDIR + "/" + "terraform/charts_v2"
            tf.working_dir = tf_dir
            tf.destroy(force=True)
    request.addfinalizer(fin)
