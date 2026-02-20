import pytest
import requests

from .conftest import SERVER_URL, protect_response


def _auth_headers(token):
    return {
        "Authorization": f"Bearer {token}",
        "Content-Type": "application/json",
    }


def _downstream_cluster_id(admin_mc):
    clusters = admin_mc.client.list_cluster().data

    for cluster in clusters:
        if cluster.id != "local" and cluster.state == "active":
            return cluster.id

    pytest.skip("No active downstream cluster is available for this test")


def test_k8s_proxy_fetches_namespaces_from_downstream_cluster(admin_mc):
    cluster_id = _downstream_cluster_id(admin_mc)
    url = f"{SERVER_URL}/k8s/proxy/{cluster_id}/api/v1/namespaces"

    response = requests.get(
        url,
        headers=_auth_headers(admin_mc.client.token),
        verify=False,
    )
    protect_response(response)

    payload = response.json()
    assert payload["kind"] == "NamespaceList"
    assert "items" in payload


def test_proxy_k8s_v1_path_returns_not_found(admin_mc):
    cluster_id = _downstream_cluster_id(admin_mc)
    url = f"{SERVER_URL}/proxy/k8s/{cluster_id}/v1"

    response = requests.get(
        url,
        headers=_auth_headers(admin_mc.client.token),
        verify=False,
    )

    assert response.status_code == 404
