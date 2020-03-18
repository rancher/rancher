import os
import pytest
import requests
import time
import urllib3
import yaml
import socket
import subprocess
import json
import rancher
from sys import platform
from .common import random_str, wait_for_template_to_be_created
from kubernetes.client import ApiClient, Configuration, CustomObjectsApi, \
    ApiextensionsV1beta1Api
from kubernetes.client.rest import ApiException
from kubernetes.config.kube_config import KubeConfigLoader
from rancher import ApiError
from .cluster_common import \
    generate_cluster_config, \
    create_cluster, \
    import_cluster


# This stops ssl warnings for unsecure certs
urllib3.disable_warnings()


def get_ip():
    s = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    try:
        # doesn't even have to be reachable
        s.connect(('10.255.255.255', 1))
        IP = s.getsockname()[0]
    except Exception:
        IP = '127.0.0.1'
    finally:
        s.close()
    return IP


IP = get_ip()
SERVER_URL = 'https://' + IP + ':8443'
BASE_URL = SERVER_URL + '/v3'
AUTH_URL = BASE_URL + '-public/localproviders/local?action=login'
DEFAULT_TIMEOUT = 45
DEFAULT_CATALOG = "https://github.com/rancher/integration-test-charts"


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


class DINDContext:
    """Returns a DINDContext for a new RKE cluster for the default global
    admin user."""

    def __init__(
        self, name, admin_mc, cluster, client, cluster_file, kube_file
    ):
        self.name = name
        self.admin_mc = admin_mc
        self.cluster = cluster
        self.client = client
        self.cluster_file = cluster_file
        self.kube_file = kube_file


@pytest.fixture(scope="session")
def admin_mc():
    """Returns a ManagementContext for the default global admin user."""
    r = requests.post(AUTH_URL, json={
        'username': 'admin',
        'password': 'admin',
        'responseType': 'json',
    }, verify=False)
    protect_response(r)
    client = rancher.Client(url=BASE_URL, token=r.json()['token'],
                            verify=False)
    k8s_client = kubernetes_api_client(client, 'local')
    admin = client.list_user(username='admin').data[0]
    return ManagementContext(client, k8s_client, user=admin)


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


def user_project_client(user, project):
    """Returns a project level client for the user"""
    return rancher.Client(url=project.links.self+'/schemas', verify=False,
                          token=user.client.token)


def user_cluster_client(user, cluster):
    """Returns a cluster level client for the user"""
    return rancher.Client(url=cluster.links.self+'/schemas', verify=False,
                          token=user.client.token)


@pytest.fixture
def admin_pc_factory(admin_cc, remove_resource):
    """Returns a ProjectContext for a newly created project in the local
    cluster for the default global admin user. The project will be deleted
    when this fixture is cleaned up."""
    def _admin_pc():
        admin = admin_cc.management.client
        p = admin.create_project(name='test-' + random_str(),
                                 clusterId=admin_cc.cluster.id)
        p = admin.wait_success(p)
        wait_for_condition("BackingNamespaceCreated", "True",
                           admin_cc.management.client, p)
        assert p.state == 'active'
        remove_resource(p)
        p = admin.reload(p)
        url = p.links.self + '/schemas'
        return ProjectContext(admin_cc, p, rancher.Client(url=url,
                                                          verify=False,
                                                          token=admin.token))
    return _admin_pc


@pytest.fixture
def admin_pc(admin_pc_factory):
    return admin_pc_factory()


@pytest.fixture
def admin_system_pc(admin_mc):
    """Returns a ProjectContext for the system project in the local cluster
    for the default global admin user."""
    admin = admin_mc.client
    plist = admin.list_project(name='System', clusterId='local')
    assert len(plist) == 1
    p = plist.data[0]
    url = p.links.self + '/schemas'
    return ProjectContext(admin_cc, p, rancher.Client(url=url,
                                                      verify=False,
                                                      token=admin.token))


@pytest.fixture
def user_mc(user_factory):
    """Returns a ManagementContext for a newly created standard user"""
    return user_factory()


@pytest.fixture
def user_factory(admin_mc, remove_resource):
    """Returns a factory for creating new users which a ManagementContext for
    a newly created standard user is returned.

    This user and globalRoleBinding will be cleaned up automatically by the
    fixture remove_resource.
    """
    def _create_user(globalRoleId='user'):
        admin = admin_mc.client
        username = random_str()
        password = random_str()
        user = admin.create_user(username=username, password=password)
        remove_resource(user)
        grb = admin.create_global_role_binding(
            userId=user.id, globalRoleId=globalRoleId)
        remove_resource(grb)
        response = requests.post(AUTH_URL, json={
            'username': username,
            'password': password,
            'responseType': 'json',
        }, verify=False)
        protect_response(response)
        client = rancher.Client(url=BASE_URL, token=response.json()['token'],
                                verify=False)
        return ManagementContext(client, user=user)

    return _create_user


@pytest.fixture
def admin_cc_client(admin_cc):
    """Returns the client from the default admin's ClusterContext"""
    return admin_cc.client


@pytest.fixture
def admin_pc_client(admin_pc):
    """Returns the client from the default admin's ProjectContext """
    return admin_pc.client


@pytest.fixture(scope="session")
def custom_catalog(admin_mc, remove_resource_session):
    """Create a catalog from the URL and cleanup after tests finish"""
    def _create_catalog(name=random_str(), catalogURL=DEFAULT_CATALOG):
        client = admin_mc.client
        catalog = client.create_catalog(name=name,
                                        branch="master",
                                        url=catalogURL,
                                        )
        remove_resource_session(catalog)
        wait_for_template_to_be_created(client, name)
    return _create_catalog


@pytest.fixture()
def restore_rancher_version(request, admin_mc):
    client = admin_mc.client
    server_version = client.by_id_setting('server-version')

    def _restore():
        client.update_by_id_setting(
            id=server_version.id, value=server_version.value)
    request.addfinalizer(_restore)


def set_server_version(client, version):
    client.update_by_id_setting(id='server-version', value=version)

    def _wait_for_version():
        server_version = client.by_id_setting('server-version')
        return server_version.value == version

    wait_for(_wait_for_version)


@pytest.fixture(scope="session")
def dind_cc(request, admin_mc):
    # verify platform is linux
    if platform != 'linux':
        raise Exception('rke dind only supported on linux')

    def set_server_url(url):
        admin_mc.client.update_by_id_setting(id='server-url', value=url)

    original_url = admin_mc.client.by_id_setting('server-url').value

    # make sure server-url is set to IP address for dind accessibility
    set_server_url(SERVER_URL)

    # revert server url to original when done
    request.addfinalizer(lambda: set_server_url(original_url))

    # create the cluster & import
    name, config, cluster_file, kube_file = generate_cluster_config(request, 1)
    create_cluster(cluster_file)
    cluster = import_cluster(admin_mc, kube_file, cluster_name=name)

    # delete cluster when done
    request.addfinalizer(lambda: admin_mc.client.delete(cluster))

    # wait for cluster to completely provision
    wait_for_condition("Ready", "True", admin_mc.client, cluster, 120)
    cluster, client = cluster_and_client(cluster.id, admin_mc.client)

    # get ip address of cluster node
    node_name = config['nodes'][0]['address']
    node_inspect = subprocess.check_output('docker inspect rke-dind-' +
                                           node_name, shell=True).decode()
    node_json = json.loads(node_inspect)
    node_ip = node_json[0]['NetworkSettings']['IPAddress']

    # update cluster fqdn with node ip
    admin_mc.client.update_by_id_cluster(
        id=cluster.id,
        name=cluster.name,
        localClusterAuthEndpoint={
            'enabled': True,
            'fqdn': node_ip + ':6443',
            'caCerts': cluster.caCert,
        },
    )
    return DINDContext(
        name, admin_mc, cluster, client, cluster_file, kube_file
    )


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


@pytest.fixture
def remove_resource(admin_mc, request):
    """Remove a resource after a test finishes even if the test fails."""
    client = admin_mc.client

    def _cleanup(resource):
        def clean():
            try:
                client.delete(resource)
            except ApiError as e:
                code = e.error.status
                if code == 409 and "namespace will automatically be purged " \
                        in e.error.message:
                    pass
                elif code != 404:
                    raise e
        request.addfinalizer(clean)
    return _cleanup


@pytest.fixture
def raw_remove_custom_resource(admin_mc, request):
    """Remove a custom resource, using the k8s client, after a test finishes
    even if the test fails. This should only be used if remove_resource, which
    exclusively uses the rancher api, cannot be used"""
    def _cleanup(resource):
        k8s_v1beta1_client = ApiextensionsV1beta1Api(admin_mc.k8s_client)
        k8s_client = CustomObjectsApi(admin_mc.k8s_client)

        def clean():
            kind = resource["kind"]
            metadata = resource["metadata"]
            api_version = resource["apiVersion"]
            api_version_parts = api_version.split("/")
            if len(api_version_parts) != 2:
                raise ValueError("Error parsing ApiVersion [" + api_version
                                 + "]." + "Expected form \"group/version\""
                                 )

            group = api_version_parts[0]
            version = api_version_parts[1]

            crd_list = k8s_v1beta1_client.\
                list_custom_resource_definition().items
            crd = list(filter(lambda x: x.spec.names.kind == kind and
                              x.spec.group == group and
                              x.spec.version == version,
                              crd_list))[0]
            try:
                k8s_client.delete_namespaced_custom_object(
                    group,
                    version,
                    metadata["namespace"],
                    crd.spec.names.plural,
                    metadata["name"],
                    {})
            except ApiException as e:
                body = json.loads(e.body)
                if body["code"] != 404:
                    raise e
        request.addfinalizer(clean)
    return _cleanup


@pytest.fixture(scope="session")
def remove_resource_session(admin_mc, request):
    """Remove a resource after the test session finishes. Can only be used
    with fixtures that are 'session' scoped.
    """
    client = admin_mc.client

    def _cleanup(resource):
        def clean():
            try:
                client.delete(resource)
            except ApiError as e:
                if e.error.status != 404:
                    raise e
        request.addfinalizer(clean)
    return _cleanup


@pytest.fixture()
def wait_remove_resource(admin_mc, request, timeout=DEFAULT_TIMEOUT):
    """Remove a resource after a test finishes even if the test fails and
    wait until deletion is confirmed."""
    client = admin_mc.client

    def _cleanup(resource):
        def clean():
            try:
                client.delete(resource)
            except ApiError as e:
                code = e.error.status
                if code == 409 and "namespace will automatically be purged " \
                        in e.error.message:
                    pass
                elif code != 404:
                    raise e
            wait_until(lambda: client.reload(resource) is None)
        request.addfinalizer(clean)
    return _cleanup


@pytest.fixture()
def list_remove_resource(admin_mc, request):
    """Takes list of resources to remove & supports reordering of the list """
    client = admin_mc.client

    def _cleanup(resource):
        def clean():
            for item in resource:
                try:
                    client.delete(item)
                except ApiError as e:
                    if e.error.status != 404:
                        raise e
                wait_until(lambda: client.reload(item) is None)
        request.addfinalizer(clean)
    return _cleanup


def wait_for_condition(condition_type, status, client, obj, timeout=45):
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
            msg = 'Expected condition {} to have status {}\n'\
                'Timeout waiting for [{}:{}] for condition after {} ' \
                'seconds\n {}'.format(condition_type, status, obj.type, obj.id,
                                      delta, str(obj))
            raise Exception(msg)
    return obj


def wait_until(cb, timeout=DEFAULT_TIMEOUT, backoff=True):
    start_time = time.time()
    interval = 1
    while time.time() < start_time + timeout and cb() is False:
        if backoff:
            interval *= 2
        time.sleep(interval)


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
    loader = KubeConfigLoader(config_dict=yaml.full_load(kc.config))
    client_configuration = type.__call__(Configuration)
    loader.load_and_set(client_configuration)
    k8s_client = ApiClient(configuration=client_configuration)
    return k8s_client


def protect_response(r):
    if r.status_code >= 300:
        message = 'Server responded with {r.status_code}\nbody:\n{r.text}'
        raise ValueError(message)


def create_kubeconfig(request, dind_cc, client):
    # request cluster scoped kubeconfig, permissions may not be synced yet
    def generateKubeconfig(max_attempts=5):
        for attempt in range(1, max_attempts+1):
            try:
                # get cluster for client
                cluster = client.by_id_cluster(dind_cc.cluster.id)
                return cluster.generateKubeconfig()['config']
            except ApiError as err:
                if attempt == max_attempts:
                    raise err
            time.sleep(1)

    cluster_kubeconfig = generateKubeconfig()

    # write cluster scoped kubeconfig
    cluster_kubeconfig_file = "kubeconfig-" + random_str() + ".yml"
    f = open(cluster_kubeconfig_file, "w")
    f.write(cluster_kubeconfig)
    f.close()

    # cleanup file when done
    request.addfinalizer(lambda: os.remove(cluster_kubeconfig_file))

    # extract token name
    config = yaml.safe_load(cluster_kubeconfig)
    token_name = config['users'][0]['user']['token'].split(':')[0]

    # wait for token to sync
    crd_client = CustomObjectsApi(
        kubernetes_api_client(
            dind_cc.admin_mc.client,
            dind_cc.cluster.id
        )
    )

    def cluster_token_available():
        try:
            return crd_client.get_namespaced_custom_object(
                'cluster.cattle.io',
                'v3',
                'cattle-system',
                'clusterauthtokens',
                token_name
            )
        except ApiException:
            return None

    wait_for(cluster_token_available)

    return cluster_kubeconfig_file


def set_cluster_psp(admin_mc, value):
    """Enable or Disable the pod security policy at the local cluster"""
    k8s_dynamic_client = CustomObjectsApi(admin_mc.k8s_client)
    # these create a mock pspts... not valid for real psp's

    def update_cluster():
        try:
            local_cluster = k8s_dynamic_client.get_cluster_custom_object(
                "management.cattle.io", "v3", "clusters", "local")
            local_cluster["metadata"]["annotations"][
                "capabilities/pspEnabled"] = value
            k8s_dynamic_client.replace_cluster_custom_object(
                "management.cattle.io", "v3", "clusters", "local",
                local_cluster)
        except ApiException as e:
            assert e.status == 409
            return False
        return True

    wait_for(update_cluster)

    def check_psp():
        cluster_obj = admin_mc.client.by_id_cluster(id="local")
        return str(cluster_obj.capabilities.pspEnabled).lower() == value

    wait_for(check_psp)


@pytest.fixture()
def restore_cluster_psp(admin_mc, request):
    cluster_obj = admin_mc.client.by_id_cluster(id="local")
    value = str(cluster_obj.capabilities.pspEnabled).lower()

    def _restore():
        set_cluster_psp(admin_mc, value)

    request.addfinalizer(_restore)
