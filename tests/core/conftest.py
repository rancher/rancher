import rancher
import pytest
import requests
import time
import urllib3
from .common import random_str
from kubernetes.client import ApiClient, Configuration
from kubernetes.config.kube_config import KubeConfigLoader
from rancher import ApiError
import yaml

# This stops ssl warnings for unsecure certs
urllib3.disable_warnings()

BASE_URL = 'https://localhost:8443/v3'
AUTH_URL = BASE_URL + '-public/localproviders/local?action=login'
CHNG_PWD_URL = BASE_URL + '/users/admin?action=changepassword'
DEFAULT_TIMEOUT = 45


class ManagementContext:
    """Contains a client that is scoped to the managment plane APIs. That is,
    APIs that are not specific to a cluster or project."""

    def __init__(self, client, k8s_client=None, user=None):
        self.client = client
        self.k8s_client = k8s_client
        self.user = user


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


@pytest.fixture(scope="session")
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
    k8s_client = kubernetes_api_client(client, 'local')
    return ManagementContext(client, k8s_client)


@pytest.fixture
def admin_cc(admin_mc):
    """Returns a ClusterContext for the local cluster for the default global
    admin user."""
    cluster, client = cluster_and_client('local', admin_mc.client)
    return ClusterContext(admin_mc, cluster, client)


def cluster_and_client(cluster_id, mgmt_client):
    cluster = mgmt_client.by_id_cluster(cluster_id)
    url = cluster.links.self + '/schemas'
    client = rancher.Client(url=url,
                            verify=False,
                            token=mgmt_client.token)
    return cluster, client


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
    return ManagementContext(client, user=user)


@pytest.fixture
def admin_cc_client(admin_cc):
    """Returns the client from the default admin's ClusterContext"""
    return admin_cc.client


@pytest.fixture
def admin_pc_client(admin_pc):
    """Returns the client from the default admin's ProjectContext """
    return admin_pc.client


def wait_for(callback, timeout=DEFAULT_TIMEOUT, fail_handler=None):
    sleep_time = _sleep_time()
    start = time.time()
    ret = callback()
    while ret is None or ret is False:
        time.sleep(next(sleep_time))
        if time.time() - start > timeout:
            exception_msg = 'Timeout waiting for condition.'
            if fail_handler:
                exception_msg = exception_msg + ' Fail handler message: ' + \
                                fail_handler()
            raise Exception(exception_msg)
        ret = callback()
    return ret


def _sleep_time():
    sleep = 0.01
    while True:
        yield sleep
        sleep *= 2
        if sleep > 1:
            sleep = 1


def wait_until_available(client, obj, timeout=DEFAULT_TIMEOUT):
    start = time.time()
    sleep = 0.01
    while True:
        time.sleep(sleep)
        sleep *= 2
        if sleep > 2:
            sleep = 2
        try:
            obj = client.reload(obj)
        except ApiError as e:
            if e.error.status != 403:
                raise e
        else:
            return obj
        delta = time.time() - start
        if delta > timeout:
            msg = 'Timeout waiting for [{}:{}] for condition after {}' \
                  ' seconds'.format(obj.type, obj.id, delta)
            raise Exception(msg)


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


def kubernetes_api_client(rancher_client, cluster_name):
    c = rancher_client.by_id_cluster(cluster_name)
    kc = c.generateKubeconfig()
    loader = KubeConfigLoader(config_dict=yaml.load(kc.config))
    client_configuration = type.__call__(Configuration)
    loader.load_and_set(client_configuration)
    k8s_client = ApiClient(configuration=client_configuration)
    return k8s_client
