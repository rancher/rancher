from .common import *  # NOQA
import pytest
import requests
import semver

namespace = {'client': None,
             'cluster': None,
             'rancher_catalog': None,
             'project': None}
m_chart_name = 'rancher-monitoring'
m_version = os.environ.get('RANCHER_MONITORING_V2_VERSION', None)
m_app = 'cattle-monitoring-system/rancher-monitoring'
m_crd = 'cattle-monitoring-system/rancher-monitoring-crd'
m_namespace = "cattle-monitoring-system"


def test_install_monitoring_v2():
    install_monitoring()


def install_monitoring():
    client = namespace['client']
    rancher_catalog = namespace['rancher_catalog']
    # install the monitoring v2 app into the new project
    project_id = namespace["project"]["id"]
    cluster_id = namespace["cluster"]["id"]
    cluster_name = namespace["cluster"]["spec"]["displayName"]
    values = read_json_from_resource_dir("monitoring_v2_values.json")
    values["projectId"] = project_id
    for chart in values["charts"]:
        chart["version"] = m_version
        chart["projectId"] = project_id
        chart["values"]["global"]["cattle"]["clusterId"] = cluster_id
        chart["values"]["global"]["cattle"]["clusterName"] = cluster_name
    client.action(rancher_catalog, "install", values)
    # wait 2 minutes for the app to be fully deployed
    time.sleep(120)
    # check the app rancher-monitoring-crd first then rancher-monitoring
    wait_for(
        lambda: client.by_id_catalog_cattle_io_app(m_crd).status.summary.state == "deployed",
        timeout_message="time out waiting for app to be ready")
    wait_for(
        lambda: client.by_id_catalog_cattle_io_app(m_app).status.summary.state == "deployed",
        timeout_message="time out waiting for app to be ready")


def uninstall_monitoring():
    client = namespace['client']
    # firstly, uninstall the monitoring app
    app = client.by_id_catalog_cattle_io_app(m_app)
    if app is not None:
        client.action(app, "uninstall")
        wait_for(
            lambda: client.by_id_catalog_cattle_io_app(m_app) is None,
            timeout_message="Timeout waiting for uninstalling monitoring")
    # then, clean up the secrets left in the namespace
    res = client.list_secret()
    if "data" in res.keys():
        for item in res.get("data"):
            if m_namespace in item['id']:
                client.delete(item)
    # then, the crd app
    app = client.by_id_catalog_cattle_io_app(m_crd)
    if app is not None:
        client.action(app, "uninstall")
        wait_for(
            lambda: client.by_id_catalog_cattle_io_app(m_crd) is None,
            timeout_message="Timeout waiting for uninstalling monitoring crd")
    # finally, the namespace
    ns = client.by_id_namespace(m_namespace)
    if ns is not None:
        client.delete(ns)
        wait_for(
            lambda: client.by_id_namespace(m_namespace) is None,
            timeout_message="Timeout waiting for deleting the namespace")


def check_monitoring_exist():
    client = namespace['client']
    ns = client.by_id_namespace(m_namespace)
    app = client.by_id_catalog_cattle_io_app(m_app)
    crd = client.by_id_catalog_cattle_io_app(m_crd)

    ns_exist = False if ns is None else True
    app_deployed = False if app is None else True
    crd_app_deployed = False if crd is None else True
    return ns_exist or app_deployed or crd_app_deployed


def get_chart_latest_version(catalog, chart_name):
    headers = {"Accept": "application/json",
               "Authorization": "Bearer " + USER_TOKEN}
    url = catalog["links"]["index"]
    response = requests.get(url=url, verify=False, headers=headers)
    assert response.status_code == 200, \
        "failed to get the response from {}".format(url)
    assert response.content is not None, \
        "no chart is returned from {}".format(url)
    res = json.loads(response.content)
    assert chart_name in res["entries"].keys(), \
        "failed to find the chart {} from the chart repo".format(chart_name)
    charts = res['entries'][chart_name]
    versions = []
    for chart in charts:
        versions.append(chart["version"])
    latest = versions[0]
    for version in versions:
        if semver.compare(latest, version) < 0:
            latest = version
    return latest


@pytest.fixture(scope='module', autouse="True")
def create_client(request):
    client = get_cluster_client_for_token_v1()
    admin_client = get_admin_client_v1()
    cluster = get_cluster_by_name(admin_client, CLUSTER_NAME)
    namespace["client"] = client
    namespace["cluster"] = cluster
    rancher_catalog = \
        client.by_id_catalog_cattle_io_clusterrepo(id="rancher-charts")
    if rancher_catalog is None:
        assert False, "rancher-charts is not available"
    namespace["rancher_catalog"] = rancher_catalog
    # set the monitoring chart version the latest if it is not provided
    global m_version
    if m_version is None:
        m_version = \
            get_chart_latest_version(rancher_catalog, m_chart_name)
        print("chart version is not provided, "
              "get chart version from repo: {}".format(m_version))

    project = create_project(cluster, random_name())
    namespace["project"] = project
    if check_monitoring_exist() is True:
        uninstall_monitoring()

    def fin():
        uninstall_monitoring()
        client.delete(namespace["project"])

    request.addfinalizer(fin)
