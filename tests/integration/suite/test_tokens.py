import pytest
import rancher
from .conftest import BASE_URL


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
