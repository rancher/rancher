import pytest
import rancher
import requests
from requests.exceptions import SSLError
from os import path

certs_exist = pytest.mark.skipif(
    not path.exists("/etc/rancher/ssl/failclient.pem"),
    reason='Test certs dont exist for proxy'
)


@certs_exist
def test_user_proxy(admin_mc):
    headers = {
        "X-Remote-User": admin_mc.user.id,
        "X-Remote-Group": "abc"
    }
    certs = ('/etc/rancher/ssl/client.pem')
    client = rancher.Client(url=admin_mc.base_url, verify=False,
                            headers=headers, cert=certs)
    assert client.list_user(username='admin').data[0]['username'] == 'admin'


@certs_exist
def test_user_proxy_invalid_cert(admin_mc):
    headers = {
        "X-Remote-User": admin_mc.user.id,
        "X-Remote-Group": "abc"
    }
    certs = ('/etc/rancher/ssl/failclient.pem')
    with pytest.raises(requests.exceptions.RequestException) as e:
        rancher.Client(url=admin_mc.base_url, verify=False,
                       headers=headers, cert=certs)
    assert isinstance(e.value, SSLError)


@certs_exist
def test_user_proxy_no_cert(admin_mc):
    headers = {
        "X-Remote-User": admin_mc.user.id,
        "X-Remote-Group": "abc"
    }
    with pytest.raises(rancher.ApiError) as e:
        rancher.Client(url=admin_mc.base_url, verify=False,
                       headers=headers)
    assert e.value.error.status == '401'
