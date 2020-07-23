import ast
import os
import pytest

from .test_rke_cluster_provisioning import (create_and_validate_custom_host,
                                            cluster_cleanup)
from .cli_objects import RancherCli
from .common import (ADMIN_TOKEN, USER_TOKEN, CATTLE_TEST_URL, CLUSTER_NAME,
                     DATA_SUBDIR, get_admin_client, get_user_client,
                     get_user_client_and_cluster, random_str,
                     get_project_client_for_token)

KNOWN_HOST = ast.literal_eval(os.environ.get('RANCHER_KNOWN_HOST', "False"))
if_test_multicluster = pytest.mark.skipif(ast.literal_eval(
    os.environ.get('RANCHER_SKIP_MULTICLUSTER', "False")),
    reason='Multi-Cluster tests are skipped in the interest of time/cost.')

SYSTEM_CHART_URL = "https://git.rancher.io/system-charts"
SYSTEM_CHART_BRANCH = os.environ.get("RANCHER_SYSTEM_CHART_BRANCH", "dev")
OPENEBS_CHART = 'openebs'
OPENEBS_CHART_VERSION = '1.5.0'
OPENEBS_CHART_VERSION_UPGRADE = '1.6.0'
CHARTMUSEUM_CHART = 'chartmuseum'
CHARTMUSEUM_CHART_VERSION = '2.3.1'
APP_TIMEOUT = 120
CATALOG_URL = "https://github.com/rancher/integration-test-charts.git"
BRANCH = "validation-tests"
CHARTMUSEUM_CHART_VERSION_CATALOG = 'latest'

# Supplying default answers due to issue with multi-cluster app install:
# https://github.com/rancher/rancher/issues/25514
MULTICLUSTER_APP_ANSWERS = {
    "analytics.enabled": "true",
    "defaultImage": "true",
    "defaultPorts": "true",
    "ndm.filters.excludePaths": "loop,fd0,sr0,/dev/ram,/dev/dm-,/dev/md",
    "ndm.filters.excludeVendors": "CLOUDBYT,OpenEBS",
    "ndm.sparse.count": "0",
    "ndm.sparse.enabled": "true",
    "ndm.sparse.path": "/var/openebs/sparse",
    "ndm.sparse.size": "10737418240", "policies.monitoring.enabled": "true"
}


def test_cli_context_switch(rancher_cli: RancherCli):
    rancher_cli.log.info("Testing Context Switching")
    clusters = rancher_cli.get_clusters()
    client = get_user_client()
    projects = client.list_project()
    assert len(projects) > 0
    for project in projects:
        rancher_cli.switch_context(project['id'])
        cluster_name, project_name = rancher_cli.get_context()
        assert any(cluster["id"] == project['clusterId']
                   and cluster["name"] == cluster_name for cluster in clusters)
        assert project_name == project['name']


def test_cli_project_create(remove_cli_resource, rancher_cli: RancherCli):
    rancher_cli.log.info("Testing Creating Projects")
    initial_projects = rancher_cli.projects.get_current_projects()
    project = rancher_cli.projects.create_project(use_context=False)
    remove_cli_resource("project", project["id"])
    assert project is not None
    assert len(initial_projects) == len(
        rancher_cli.projects.get_current_projects()) - 1


def test_cli_project_delete(remove_cli_resource, rancher_cli: RancherCli):
    rancher_cli.log.info("Testing Deleting Projects")
    initial_projects = rancher_cli.projects.get_current_projects()
    project = rancher_cli.projects.create_project(use_context=False)
    remove_cli_resource("project", project["id"])
    assert project is not None
    rancher_cli.projects.delete_project(project["name"])
    assert len(initial_projects) == len(
        rancher_cli.projects.get_current_projects())


def test_cli_namespace_create(remove_cli_resource, rancher_cli: RancherCli):
    rancher_cli.log.info("Testing Creating Namespaces")
    p1 = rancher_cli.projects.create_project()
    remove_cli_resource("project", p1["id"])
    namespace = rancher_cli.projects.create_namespace()
    remove_cli_resource("namespace", namespace)
    assert len(rancher_cli.projects.get_namespaces()) == 1
    assert "{}|active".format(
        namespace) in rancher_cli.projects.get_namespaces()


def test_cli_namespace_move(remove_cli_resource, rancher_cli: RancherCli):
    rancher_cli.log.info("Testing Moving Namespaces")
    p1 = rancher_cli.projects.create_project()
    remove_cli_resource("project", p1["id"])
    namespace = rancher_cli.projects.create_namespace()
    remove_cli_resource("namespace", namespace)
    assert len(rancher_cli.projects.get_namespaces()) == 1

    p2 = rancher_cli.projects.create_project(use_context=False)
    remove_cli_resource("project", p2["id"])
    rancher_cli.projects.move_namespace(namespace, p2["id"])
    assert len(rancher_cli.projects.get_namespaces()) == 0
    rancher_cli.projects.switch_context(p2["id"])
    assert len(rancher_cli.projects.get_namespaces()) == 1
    assert "{}|active".format(
        namespace) in rancher_cli.projects.get_namespaces()


def test_cli_namespace_delete(remove_cli_resource, rancher_cli: RancherCli):
    rancher_cli.log.info("Testing Deleting Namespaces")
    p1 = rancher_cli.projects.create_project()
    remove_cli_resource("project", p1["id"])
    namespace = rancher_cli.projects.create_namespace()
    remove_cli_resource("namespace", namespace)
    assert len(rancher_cli.projects.get_namespaces()) == 1
    assert "{}|active".format(
        namespace) in rancher_cli.projects.get_namespaces()
    deleted = rancher_cli.projects.delete_namespace(namespace)
    assert deleted


def test_cli_app_install(remove_cli_resource, rancher_cli: RancherCli):
    rancher_cli.log.info("Testing Upgrading Apps")
    initial_app = rancher_cli.apps.install(
        OPENEBS_CHART, "openebs", version=OPENEBS_CHART_VERSION,
        timeout=APP_TIMEOUT)
    remove_cli_resource("apps", initial_app["id"])
    assert initial_app["state"] == "active"
    assert initial_app["version"] == OPENEBS_CHART_VERSION


def test_cli_app_values_install(remove_cli_resource, rancher_cli: RancherCli):
    rancher_cli.log.info("Testing Upgrading Apps")
    initial_app = rancher_cli.apps.install(
        CHARTMUSEUM_CHART, random_str(), version=CHARTMUSEUM_CHART_VERSION, 
        timeout=APP_TIMEOUT, values=DATA_SUBDIR + "/appvalues.yaml")
    remove_cli_resource("apps", initial_app["id"])
    assert initial_app["state"] == "active"
    assert initial_app["version"] == CHARTMUSEUM_CHART_VERSION


def test_cli_app_upgrade(remove_cli_resource, rancher_cli: RancherCli):
    rancher_cli.log.info("Testing Rolling Back Apps")
    initial_app = rancher_cli.apps.install(
        OPENEBS_CHART, "openebs", version=OPENEBS_CHART_VERSION,
        timeout=APP_TIMEOUT)
    remove_cli_resource("apps", initial_app["id"])
    assert initial_app["version"] == OPENEBS_CHART_VERSION
    upgraded_app = rancher_cli.apps.upgrade(
        initial_app, version=OPENEBS_CHART_VERSION_UPGRADE)
    assert upgraded_app["state"] == "active"
    assert upgraded_app["version"] == OPENEBS_CHART_VERSION_UPGRADE


def test_cli_app_rollback(remove_cli_resource, rancher_cli: RancherCli):
    rancher_cli.log.info("Testing Deleting Apps")
    initial_app = rancher_cli.apps.install(
        OPENEBS_CHART, "openebs", version=OPENEBS_CHART_VERSION,
        timeout=APP_TIMEOUT)
    remove_cli_resource("apps", initial_app["id"])
    assert initial_app["version"] == OPENEBS_CHART_VERSION
    upgraded_app = rancher_cli.apps.upgrade(
        initial_app, version=OPENEBS_CHART_VERSION_UPGRADE)
    assert upgraded_app["version"] == OPENEBS_CHART_VERSION_UPGRADE
    rolled_back_app = rancher_cli.apps.rollback(upgraded_app,
                                                OPENEBS_CHART_VERSION)
    assert rolled_back_app["state"] == "active"
    assert rolled_back_app["version"] == OPENEBS_CHART_VERSION


def test_cli_app_delete(rancher_cli: RancherCli):
    rancher_cli.log.info("Testing Deleting Apps")
    initial_app = rancher_cli.apps.install(
        OPENEBS_CHART, "openebs", version=OPENEBS_CHART_VERSION,
        timeout=APP_TIMEOUT)
    deleted = rancher_cli.apps.delete(initial_app)
    assert deleted


def test_app_install_local_dir(remove_cli_resource, rancher_cli: RancherCli):
    rancher_cli.log.info("Testing Installing of an App from Local directory")
    initial_app = rancher_cli.apps.install_local_dir(
        CATALOG_URL, BRANCH, CHARTMUSEUM_CHART,
        version=CHARTMUSEUM_CHART_VERSION_CATALOG, timeout=APP_TIMEOUT)
    remove_cli_resource("apps", initial_app["id"])
    assert initial_app["state"] == "active"


@if_test_multicluster
def test_cli_multiclusterapp_install(custom_cluster, remove_cli_resource,
                                     rancher_cli: RancherCli):
    rancher_cli.log.info("Testing Installing Multi-Cluster Apps")
    # Get list of projects to use and ensure that it is 2 or greater
    client = get_admin_client()
    projects = client.list_project()
    targets = []
    for project in projects:
        if project["name"] == "Default":
            rancher_cli.switch_context(project['id'])
            cluster_name, project_name = rancher_cli.get_context()
            if cluster_name in [custom_cluster.name, CLUSTER_NAME]:
                rancher_cli.log.debug("Using cluster: %s", cluster_name)
                targets.append(project["id"])
    assert len(targets) > 1

    initial_app = rancher_cli.mcapps.install(
        OPENEBS_CHART, targets=targets, role="cluster-owner",
        values=MULTICLUSTER_APP_ANSWERS, version=OPENEBS_CHART_VERSION, 
        timeout=APP_TIMEOUT)
    remove_cli_resource("mcapps", initial_app["name"])
    assert initial_app["state"] == "active"
    assert initial_app["version"] == OPENEBS_CHART_VERSION
    assert len(initial_app["targets"]) == len(targets)
    for target in initial_app["targets"]:
        assert target["state"] == "active"
        assert target["version"] == OPENEBS_CHART_VERSION


@if_test_multicluster
def test_cli_multiclusterapp_upgrade(custom_cluster, remove_cli_resource,
                                     rancher_cli: RancherCli):
    rancher_cli.log.info("Testing Upgrading Multi-Cluster Apps")
    # Get list of projects to use and ensure that it is 2 or greater
    client = get_admin_client()
    projects = client.list_project()
    targets = []
    for project in projects:
        if project["name"] == "Default":
            rancher_cli.switch_context(project['id'])
            cluster_name, project_name = rancher_cli.get_context()
            if cluster_name in [custom_cluster.name, CLUSTER_NAME]:
                rancher_cli.log.debug("Using cluster: %s", cluster_name)
                targets.append(project["id"])
    assert len(targets) > 1

    initial_app = rancher_cli.mcapps.install(
        OPENEBS_CHART, targets=targets, role="cluster-owner",
        values=MULTICLUSTER_APP_ANSWERS, version=OPENEBS_CHART_VERSION, 
        timeout=APP_TIMEOUT)
    remove_cli_resource("mcapps", initial_app["name"])
    assert initial_app["version"] == OPENEBS_CHART_VERSION

    upgraded_app = rancher_cli.mcapps.upgrade(
        initial_app, version=OPENEBS_CHART_VERSION_UPGRADE,
        timeout=APP_TIMEOUT)
    assert upgraded_app["state"] == "active"
    assert upgraded_app["version"] == OPENEBS_CHART_VERSION_UPGRADE
    assert upgraded_app["id"] == initial_app["id"]
    assert len(upgraded_app["targets"]) == len(targets)
    for target in upgraded_app["targets"]:
        assert target["state"] == "active"
        assert target["version"] == OPENEBS_CHART_VERSION_UPGRADE


@if_test_multicluster
def test_cli_multiclusterapp_rollback(custom_cluster, remove_cli_resource,
                                      rancher_cli: RancherCli):
    rancher_cli.log.info("Testing Rolling Back Multi-Cluster Apps")
    # Get list of projects to use and ensure that it is 2 or greater
    client = get_admin_client()
    projects = client.list_project()
    targets = []
    for project in projects:
        if project["name"] == "Default":
            rancher_cli.switch_context(project['id'])
            cluster_name, project_name = rancher_cli.get_context()
            if cluster_name in [custom_cluster.name, CLUSTER_NAME]:
                rancher_cli.log.debug("Using cluster: %s", cluster_name)
                targets.append(project["id"])
    assert len(targets) > 1

    initial_app = rancher_cli.mcapps.install(
        OPENEBS_CHART, targets=targets, role="cluster-owner",
        values=MULTICLUSTER_APP_ANSWERS, version=OPENEBS_CHART_VERSION, 
        timeout=APP_TIMEOUT)
    remove_cli_resource("mcapps", initial_app["name"])
    assert initial_app["version"] == OPENEBS_CHART_VERSION
    upgraded_app = rancher_cli.mcapps.upgrade(
        initial_app, version=OPENEBS_CHART_VERSION_UPGRADE,
        timeout=APP_TIMEOUT)
    assert upgraded_app["version"] == OPENEBS_CHART_VERSION_UPGRADE

    rolled_back_app = rancher_cli.mcapps.rollback(
        upgraded_app["name"], initial_app["revision"], timeout=APP_TIMEOUT)
    assert rolled_back_app["state"] == "active"
    assert rolled_back_app["version"] == OPENEBS_CHART_VERSION
    assert rolled_back_app["id"] == upgraded_app["id"]
    assert len(rolled_back_app["targets"]) == len(targets)
    for target in rolled_back_app["targets"]:
        assert target["state"] == "active"
        assert target["version"] == OPENEBS_CHART_VERSION


@if_test_multicluster
def test_cli_multiclusterapp_delete(custom_cluster, remove_cli_resource,
                                    rancher_cli: RancherCli):
    rancher_cli.log.info("Testing Deleting Multi-Cluster Apps")
    # Get list of projects to use and ensure that it is 2 or greater
    client = get_admin_client()
    projects = client.list_project()
    targets = []
    for project in projects:
        if project["name"] == "Default":
            rancher_cli.switch_context(project['id'])
            cluster_name, project_name = rancher_cli.get_context()
            if cluster_name in [custom_cluster.name, CLUSTER_NAME]:
                rancher_cli.log.debug("Using cluster: %s", cluster_name)
                targets.append(project["id"])
    assert len(targets) > 1

    initial_app = rancher_cli.mcapps.install(
        OPENEBS_CHART, targets=targets, role="cluster-owner",
        values=MULTICLUSTER_APP_ANSWERS, version=OPENEBS_CHART_VERSION, 
        timeout=APP_TIMEOUT)
    assert initial_app["version"] == OPENEBS_CHART_VERSION
    deleted, apps_deleted = rancher_cli.mcapps.delete(initial_app)
    assert deleted
    assert apps_deleted


def test_cli_catalog(admin_cli: RancherCli):
    admin_cli.log.info("Testing Creating and Deleting Catalogs")
    admin_cli.login(CATTLE_TEST_URL, ADMIN_TOKEN)
    catalog = admin_cli.catalogs.add(SYSTEM_CHART_URL,
                                     branch=SYSTEM_CHART_BRANCH)
    assert catalog is not None
    deleted = admin_cli.catalogs.delete(catalog["name"])
    assert deleted


@if_test_multicluster
def test_cluster_removal(custom_cluster, admin_cli: RancherCli):
    admin_cli.log.info("Testing Cluster Removal")
    deleted = admin_cli.clusters.delete(custom_cluster.name)
    assert deleted


def test_inspection(rancher_cli: RancherCli):
    # Test inspect on the default project used for cli tests
    # Validate it has the expected clusterid, id, type, and active state
    rancher_cli.log.info("Testing Inspect Resource")
    resource = rancher_cli.inspect(
        "project", rancher_cli.default_project["id"],
        format="{{.clusterId}}|{{.id}}|{{.type}}|{{.state}}")
    assert resource is not None
    resource_arr = resource.split("|")
    assert resource_arr[0] == rancher_cli.default_project["clusterId"]
    assert resource_arr[1] == rancher_cli.default_project["id"]
    assert resource_arr[2] == "project"
    assert resource_arr[3] == "active"


def test_ps(custom_workload, rancher_cli: RancherCli):
    rancher_cli.log.info("Testing rancher ps")
    # Deploy a workload and validate that the ps command shows it in the
    # correct namespace with the correct name
    rancher_cli.switch_context(rancher_cli.DEFAULT_CONTEXT)
    ps = rancher_cli.ps()
    expected_value = "{}|{}|nginx|2".format(
        rancher_cli.default_namespace, custom_workload.name)
    assert expected_value in ps.splitlines()


def test_kubectl(custom_workload, rancher_cli: RancherCli):
    rancher_cli.log.info("Testing kubectl commands from the CLI")
    rancher_cli.switch_context(rancher_cli.DEFAULT_CONTEXT)
    jsonpath = "-o jsonpath='{.spec.template.spec.containers[0].image}'"
    result = rancher_cli.kubectl("get deploy -n {} {} {}".format(
        rancher_cli.default_namespace, custom_workload.name, jsonpath))
    assert result == "nginx"


# Note this expects nodes not to be Windows due to usage of ifconfig.me
@pytest.mark.skip(reason="Fails in Jenkins")
def test_ssh(rancher_cli: RancherCli):
    rancher_cli.log.info("Testing ssh into nodes.")
    failures = []
    rancher_cli.switch_context(rancher_cli.DEFAULT_CONTEXT)
    nodes = rancher_cli.nodes.get()
    rancher_cli.log.debug("Nodes is: {}".format(nodes))

    is_jenkins = False
    if os.environ.get("RANCHER_IS_JENKINS", None):
        is_jenkins = True
    for node in nodes:
        ip = rancher_cli.nodes.ssh(node, "curl -s ifconfig.me",
                                   known=KNOWN_HOST, is_jenkins=is_jenkins)
        if node["ip"] != ip:
            failures.append(node["ip"])
    assert failures == []


@pytest.fixture(scope='module')
def custom_workload(rancher_cli):
    client, cluster = get_user_client_and_cluster()
    project = client.list_project(name=rancher_cli.default_project["name"],
                                  clusterId=cluster.id).data[0]
    p_client = get_project_client_for_token(project, USER_TOKEN)
    workload = p_client.create_workload(
        name=random_str(),
        namespaceId=rancher_cli.default_namespace,
        scale=2,
        containers=[{
            'name': 'one',
            'image': 'nginx',
        }])
    return workload


@pytest.fixture(scope='module')
def custom_cluster(request, rancher_cli):
    rancher_cli.log.info("Creating cluster in AWS to test CLI actions that "
                         "require more than one cluster. Please be patient, "
                         "as this takes some time...")
    node_roles = [["controlplane"], ["etcd"],
                  ["worker"], ["worker"], ["worker"]]
    cluster, aws_nodes = create_and_validate_custom_host(
        node_roles, random_cluster_name=True)

    def fin():
        cluster_cleanup(get_admin_client(), cluster, aws_nodes)
    request.addfinalizer(fin)
    return cluster


@pytest.fixture
def admin_cli(request, rancher_cli) -> RancherCli:
    """
       Login occurs at a global scope, so need to ensure we log back in as the
       user in a finalizer so that future tests have no issues.
    """
    rancher_cli.login(CATTLE_TEST_URL, ADMIN_TOKEN)

    def fin():
        rancher_cli.login(CATTLE_TEST_URL, USER_TOKEN)
    request.addfinalizer(fin)
    return rancher_cli


@pytest.fixture(scope='module', autouse="True")
def rancher_cli(request) -> RancherCli:
    client, cluster = get_user_client_and_cluster()
    project_id = client.list_project(name='Default',
                                     clusterId=cluster.id).data[0]["id"]
    cli = RancherCli(CATTLE_TEST_URL, USER_TOKEN, project_id)

    def fin():
        cli.cleanup()
    request.addfinalizer(fin)
    return cli


@pytest.fixture
def remove_cli_resource(request, rancher_cli):
    """Remove a resource after a test finishes even if the test fails.

    How to use:
      pass this function as an argument of your testing function,
      then call this function with the resource type and its id
      as arguments after creating any new resource
    """
    def _cleanup(resource, r_id):
        def clean():
            rancher_cli.switch_context(rancher_cli.DEFAULT_CONTEXT)
            rancher_cli.log.info("Cleaning up {}: {}".format(resource, r_id))
            rancher_cli.run_command("{} delete {}".format(resource, r_id),
                                    expect_error=True)
        request.addfinalizer(clean)
    return _cleanup
