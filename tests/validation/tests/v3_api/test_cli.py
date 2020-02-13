import os
import pytest

from .cli_common import RancherCli, DEFAULT_CONTEXT
from .common import CATTLE_TEST_URL, USER_TOKEN, get_user_client
from .cli_objects import CliProject

CLUSTER_NAMES = [os.environ.get("RANCHER_CLUSTER_NAME_1", ""),
                 os.environ.get("RANCHER_CLUSTER_NAME_2", "")]


def test_list_clusters(rancher_cli: RancherCli):
    rancher_cli.log.info("Testing Listing Clusters")
    clusters = rancher_cli.get_clusters()
    assert len(clusters) == 2
    for cluster in clusters:
        assert cluster["name"] in CLUSTER_NAMES


def test_context_switching(rancher_cli: RancherCli):
    rancher_cli.log.info("Testing Context Switching")
    clusters = rancher_cli.get_clusters()
    client = get_user_client()
    projects = client.list_project()
    for project in projects:
        rancher_cli.switch_context(project['id'])
        cluster_name, project_name = rancher_cli.get_context()
        assert any(cluster["id"] == project['clusterId']
                   and cluster["name"] == cluster_name for cluster in clusters)
        assert project_name == project['name']


def test_project_manipulation(remove_cli_resource, cli_projects: CliProject):
    cli_projects.log.info("Testing Creating and Deleting Projects")
    project = cli_projects.create_project(use_context=False)
    remove_cli_resource("project", project["id"])
    assert project is not None
    assert len(cli_projects.initial_projects) == \
        len(cli_projects.get_current_projects()) - 1

    cli_projects.delete_project(project["name"])
    assert len(cli_projects.initial_projects) == \
        len(cli_projects.get_current_projects())


def test_namespace_manipulation(remove_cli_resource, cli_projects: CliProject):
    cli_projects.log.info("Testing Creating, Deleting, and Moving Namespaces")
    p1 = cli_projects.create_project()
    remove_cli_resource("project", p1["id"])
    namespace = cli_projects.create_namespace()
    remove_cli_resource("namespace", namespace)
    assert len(cli_projects.get_namespaces()) == 1
    assert "{}|active".format(namespace) in cli_projects.get_namespaces()

    p2 = cli_projects.create_project(use_context=False)
    remove_cli_resource("project", p2["id"])
    cli_projects.move_namespace(namespace, p2["id"])
    assert len(cli_projects.get_namespaces()) == 0
    cli_projects.switch_context(p2["id"])
    assert len(cli_projects.get_namespaces()) == 1
    assert "{}|active".format(namespace) in cli_projects.get_namespaces()

    deleted = cli_projects.delete_namespace(namespace)
    assert deleted


@pytest.fixture()
def cli_projects(rancher_cli):
    return CliProject()


@pytest.fixture(scope='module', autouse="True")
def rancher_cli(request):
    cli = RancherCli()
    cli.login(CATTLE_TEST_URL, USER_TOKEN)
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
            rancher_cli.switch_context(DEFAULT_CONTEXT)
            rancher_cli.log.info("Cleaning up {}: {}".format(resource, r_id))
            rancher_cli.run_command("{} delete {}".format(resource, r_id),
                                    expect_error=True)
        request.addfinalizer(clean)
    return _cleanup
