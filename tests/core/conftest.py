import cattle
import pytest
import requests

from common import random_str


class ManagementContext:
    def __init__(self, client):
        self.client = client


class ClusterContext:
    def __init__(self, management, cluster, client):
        self.management = management
        self.cluster = cluster
        self.client = client


class ProjectContext:
    def __init__(self, cluster_context, project, client):
        self.cluster = cluster_context
        self.project = project
        self.client = client


@pytest.fixture
def url():
    return 'https://localhost:8443/v3'


@pytest.fixture
def auth_url():
    return 'https://localhost:8443/v3-public/localproviders/local?action=login'


@pytest.fixture
def chngpwd(url):
    return url + '/users/admin?action=changepassword'


@pytest.fixture
def mc(url, auth_url, chngpwd):
    requests.post(chngpwd, json={
        'newPassword': 'admin',
    }, verify=False)
    r = requests.post(auth_url, json={
        'username': 'admin',
        'password': 'admin',
        'responseType': 'json',
    }, verify=False)
    client = cattle.Client(url=url, token=r.json()['token'], verify=False)
    return ManagementContext(client)


@pytest.fixture
def cc(mc):
    cluster = mc.client.by_id_cluster('local')
    url = cluster.links['self'] + '/schemas'
    client = cattle.Client(url=url,
                           verify=False,
                           token=mc.client._token)
    return ClusterContext(mc, cluster, client)


@pytest.fixture
def pc(request, cc):
    p = cc.management.client.create_project(name='test-' + random_str(),
                                            clusterId=cc.cluster.id)
    p = cc.management.client.wait_success(p)
    assert p.state == 'active'
    request.addfinalizer(lambda: cc.management.client.delete(p))
    url = p.links['self'] + '/schemas'
    return ProjectContext(cc, p, cattle.Client(url=url,
                                               verify=False,
                                               token=cc.client._token))


@pytest.fixture
def cclient(cc):
    return cc.client


@pytest.fixture
def client(pc):
    return pc.client
