import pytest
import rancher
import requests
import time
from .conftest import BASE_URL, AUTH_URL, protect_response


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
        id="kubeconfig-token-ttl-minutes",
        value=kubeconfig_ttl_mins)

    # call login action for kubeconfig token
    kubeconfig_token = login()
    ttl1, token1 = get_token_and_ttl(kubeconfig_token)
    assert ttl1 == kubeconfig_ttl_mins

    # wait for token to expire
    time.sleep(kubeconfig_ttl_mins*60)

    # confirm new kubeconfig token gets generated
    kubeconfig_token2 = login()
    ttl2, token2 = get_token_and_ttl(kubeconfig_token2)
    assert ttl2 == kubeconfig_ttl_mins
    assert token1 != token2

    # reset kubeconfig ttl setting
    client.update_by_id_setting(id="kubeconfig-token-ttl-minutes",
                                value="960")

    # enable kubeconfig generation setting
    client.update_by_id_setting(id="kubeconfig-generate-token", value="true")


def login():
    r = requests.post(AUTH_URL, json={
        'username': 'admin',
        'password': 'admin',
        'responseType': 'kubeconfig',
    }, verify=False)
    protect_response(r)
    return r.json()


def get_token_and_ttl(token):
    token1_ttl_mins = mins(int(token["ttl"]))
    token1_token = token["token"]
    return token1_ttl_mins, token1_token


def mins(time_millisec):
    return time_millisec / 60000
