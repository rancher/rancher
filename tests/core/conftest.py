import rancher
import pytest
import requests
import time
import urllib3
from .common import random_str

# This stops ssl warnings for unsecure certs
urllib3.disable_warnings()

BASE_URL = 'https://localhost:8443/v3'
AUTH_URL = BASE_URL + '-public/localproviders/local?action=login'
CHNG_PWD_URL = BASE_URL + '/users/admin?action=changepassword'


class ManagementContext:
    """Contains a client that is scoped to the managment plane APIs. That is,
    APIs that are not specific to a cluster or project."""

    def __init__(self, client):
        self.client = client


class ClusterContext:
    """Contains a client that is scoped to a specific cluster. Also contains
    a reference to the ManagementContext used to create cluster client and
    the cluster object itself.
    """

    def __init__(self, management, cluster, client):
        self.management = management
        self.cluster = cluster
        self.client = client


class ProjectContext:
    """Contains a client that is scoped to a newly created project. Also
    contains a reference to the clusterContext used to crete the project and
    the project object itself.
    """

    def __init__(self, cluster_context, project, client):
        self.cluster = cluster_context
        self.project = project
        self.client = client


@pytest.fixture
def admin_mc():
    """Returns a ManagementContext for the default global admin user."""
    requests.post(CHNG_PWD_URL, json={
        'newPassword': 'admin',
    }, verify=False)
    r = requests.post(AUTH_URL, json={
        'username': 'admin',
        'password': 'admin',
        'responseType': 'json',
    }, verify=False)
    client = rancher.Client(url=BASE_URL, token=r.json()['token'],
                            verify=False)
    return ManagementContext(client)


@pytest.fixture
def admin_cc(admin_mc):
    """Returns a ClusterContext for the local cluster for the default global
    admin user."""
    cluster = admin_mc.client.by_id_cluster('local')
    url = cluster.links.self + '/schemas'
    client = rancher.Client(url=url,
                            verify=False,
                            token=admin_mc.client.token)
    return ClusterContext(admin_mc, cluster, client)


@pytest.fixture
def admin_pc(request, admin_cc):
    """Returns a ProjectContect for a newly created project in the local
    cluster for the default global admin user. The project will be deleted
    when this fixture is cleaned up."""
    admin = admin_cc.management.client
    p = admin.create_project(name='test-' + random_str(),
                             clusterId=admin_cc.cluster.id)
    p = admin.wait_success(p)
    wait_for_condition("BackingNamespaceCreated", "True",
                       admin_cc.management.client, p)
    assert p.state == 'active'
    request.addfinalizer(lambda: admin_cc.management.client.delete(p))
    url = p.links.self + '/schemas'
    return ProjectContext(admin_cc, p, rancher.Client(url=url,
                                                      verify=False,
                                                      token=admin.token))


@pytest.fixture
def user_mc(admin_mc):
    """Returns a ManagementContext for a newly created standard user"""
    admin = admin_mc.client
    username = random_str()
    password = random_str()
    user = admin.create_user(username=username, password=password)
    admin.create_global_role_binding(userId=user.id, globalRoleId='user')
    response = requests.post(AUTH_URL, json={
        'username': username,
        'password': password,
        'responseType': 'json',
    }, verify=False)
    client = rancher.Client(url=BASE_URL, token=response.json()['token'],
                            verify=False)
    return ManagementContext(client)


def wait_for_condition(condition_type, status, client, obj):
    timeout = 45
    start = time.time()
    obj = client.reload(obj)
    sleep = 0.01
    while not find_condition(condition_type, status, obj):
        time.sleep(sleep)
        sleep *= 2
        if sleep > 2:
            sleep = 2
        obj = client.reload(obj)
        delta = time.time() - start
        if delta > timeout:
            msg = 'Timeout waiting for [{}:{}] for condition after {}' \
                ' seconds'.format(obj.type, obj.id, delta)
            raise Exception(msg)


def find_condition(condition_type, status, obj):
    if not hasattr(obj, "conditions"):
        return False

    if obj.conditions is None:
        return False

    for condition in obj.conditions:
        if condition.type == condition_type and condition.status == status:
            return True
    return False


@pytest.fixture
def admin_cc_client(admin_cc):
    """Returns the client from the default admin's ClusterContext"""
    return admin_cc.client


@pytest.fixture
def admin_pc_client(admin_pc):
    """Returns the client from the default admin's ProjectContext """
    return admin_pc.client
