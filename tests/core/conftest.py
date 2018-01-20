import cattle
import pytest
import requests

from common import random_str


class ClusterContext:
    def __init__(self, cluster, client):
        self.cluster = cluster
        self.client = client


class ProjectContext:
    def __init__(self, cluster_context, project, client):
        self.cluster = cluster_context
        self.project = project
        self.client = client


@pytest.fixture
def url():
    return 'http://localhost:8080/v3'


@pytest.fixture
def auth_url(url):
    return url + '/tokens?action=login'


@pytest.fixture
def chngpwd(url):
    return url + '/users/admin?action=changepassword'


@pytest.fixture
def cc(url, auth_url, chngpwd):
    requests.post(chngpwd, json={
        'newPassword': 'admin',
    })
    r = requests.post(auth_url, json={
        'localCredential': {
            'username': 'admin',
            'password': 'admin',
        },
        'responseType': 'json',
    })
    client = cattle.Client(url=url, token=r.json()['token'])
    cluster = client.by_id_cluster('local')
    return ClusterContext(cluster, client)


@pytest.fixture
def pc(request, cc):
    p = cc.client.create_project(name='test-' + random_str(),
                                 clusterId=cc.cluster.id)
    p = cc.client.wait_success(p)
    assert p.state == 'active'
    request.addfinalizer(lambda: cc.client.delete(p))
    url = p.links['self'] + '/schemas'
    return ProjectContext(cc, p, cattle.Client(url=url,
                                               token=cc.client._token))


@pytest.fixture
def cclient(cc):
    return cc.client


@pytest.fixture
def client(pc):
    return pc.client
