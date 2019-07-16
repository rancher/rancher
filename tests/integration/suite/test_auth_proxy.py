import pytest
import rancher
import requests
from os import path

certs_exist = pytest.mark.skipif(
    not path.exists("/etc/rancher/ssl/failclient.pem"),
    reason='Test certs dont exist for proxy'
)


@certs_exist
def test_user_proxy(admin_proxy):
    headers = {
        "X-Remote-User": admin_proxy.user.id,
        "X-Remote-Group": "abc"
    }
    certs = ('/etc/rancher/ssl/client.pem')
    client = rancher.Client(url=admin_proxy.BASE_URL, verify=False,
                            headers=headers, cert=certs)
    client.list_user(username='admin').data[0]
    assert True


@certs_exist
def test_user_proxy_invalid_cert(admin_proxy):
    headers = {
        "X-Remote-User": admin_proxy.user.id,
        "X-Remote-Group": "abc"
    }
    certs = ('/etc/rancher/ssl/failclient.pem')
    with pytest.raises(requests.exceptions.RequestException) as e:
        rancher.Client(url=admin_proxy.BASE_URL, verify=False,
                       headers=headers, cert=certs)
    assert e is not None


@certs_exist
def test_user_proxy_no_cert(admin_proxy):
    headers = {
        "X-Remote-User": admin_proxy.user.id,
        "X-Remote-Group": "abc"
    }
    with pytest.raises(rancher.ApiError) as e:
        rancher.Client(url=admin_proxy.BASE_URL, verify=False,
                       headers=headers)
    assert e.value.error.status == '401'
