import pytest
import rancher
import requests
import time
from .conftest import SERVER_PASSWORD, BASE_URL, AUTH_URL, \
                    AUTH_URL_V1, protect_response


def test_certificates(admin_mc):
    client = admin_mc.client

    tokens = client.list_token()

    currentCount = 0
    for t in tokens:
        if t.current:
            assert t.userId == admin_mc.user.id
            currentCount += 1

    assert currentCount == 1


def test_websocket(admin_mc):
    client = rancher.Client(url=BASE_URL, token=admin_mc.client.token,
                            verify=False)
    # make a request that looks like a websocket
    client._session.headers["Connection"] = "upgrade"
    client._session.headers["Upgrade"] = "websocket"
    client._session.headers["Origin"] = "badStuff"
    client._session.headers["User-Agent"] = "Mozilla"
    # do something with client now that we have a "websocket"

    with pytest.raises(rancher.ApiError) as e:
        client.list_cluster()

    assert e.value.error.Code.Status == 403


def test_api_token_ttl(admin_mc, remove_resource):
    client = admin_mc.client

    max_ttl = client.by_id_setting("auth-token-max-ttl-minutes")
    max_ttl_mins = int(max_ttl["value"])

    # api tokens must be created with min(input_ttl, max_ttl)
    token = client.create_token(ttl=0)
    remove_resource(token)

    token_ttl_mins = mins(token["ttl"])

    assert token_ttl_mins == max_ttl_mins


@pytest.mark.nonparallel
def test_kubeconfig_token_ttl(admin_mc, user_mc):
    client = admin_mc.client

    # delete existing kubeconfig token
    kubeconfig_token_name = "kubeconfig-" + admin_mc.user.id
    token = client.by_id_token(kubeconfig_token_name)
    if token is not None:
        client.delete(token)

    # disable kubeconfig generation setting
    client.update_by_id_setting(id="kubeconfig-generate-token", value="false")

    # update kubeconfig ttl setting for test
    kubeconfig_ttl_mins = 0.01
    client.update_by_id_setting(
        id="kubeconfig-default-token-ttl-minutes",
        value=kubeconfig_ttl_mins)

    # /v3-public endpoint (deprecated)
    # call login action for kubeconfig token
    kubeconfig_token = login()
    assert kubeconfig_token["token"] != ""
    assert kubeconfig_token["expiresAt"] != ""
    assert kubeconfig_token["id"] != ""
    assert kubeconfig_token["token"].startswith(kubeconfig_token["id"])
    assert kubeconfig_token["type"] == "token"
    assert kubeconfig_token["baseType"] == "token"

    # wait for token to expire
    time.sleep(kubeconfig_ttl_mins*60)

    # confirm new kubeconfig token gets generated
    kubeconfig_token2 = login()
    assert kubeconfig_token2["token"] != ""
    assert kubeconfig_token2["expiresAt"] != ""

    # make sure new token is different
    assert kubeconfig_token["token"] != kubeconfig_token2["token"]

    time.sleep(kubeconfig_ttl_mins*60)

    # /v1-public endpoint
    kubeconfig_token = login_v1()
    assert kubeconfig_token["token"] != ""
    assert kubeconfig_token["expiresAt"] != ""

    # wait for token to expire
    time.sleep(kubeconfig_ttl_mins*60)

    # confirm new kubeconfig token gets generated
    kubeconfig_token2 = login_v1()
    assert kubeconfig_token2["token"] != ""
    assert kubeconfig_token2["expiresAt"] != ""

    # reset kubeconfig ttl setting
    client.update_by_id_setting(id="kubeconfig-default-token-ttl-minutes",
                                value="43200")

    # enable kubeconfig generation setting
    client.update_by_id_setting(id="kubeconfig-generate-token", value="true")


def login():
    r = requests.post(AUTH_URL, json={
        'username': 'admin',
        'password': SERVER_PASSWORD,
        'responseType': 'kubeconfig',
    }, verify=False)
    protect_response(r)
    return r.json()


def login_v1():
    r = requests.post(AUTH_URL_V1, json={
        'type': 'localProvider',
        'username': 'admin',
        'password': SERVER_PASSWORD,
        'responseType': 'kubeconfig',
    }, verify=False)
    protect_response(r)
    return r.json()


def mins(time_millisec):
    return time_millisec / 60000
