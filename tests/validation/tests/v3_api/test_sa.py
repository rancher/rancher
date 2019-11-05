import pytest

from .common import *  # NOQA

do_test_sa = \
    ast.literal_eval(os.environ.get('RANCHER_SA_CHECK', "True"))

if_test_sa = pytest.mark.skipif(
    do_test_sa is not True,
    reason="This test should not be executed on imported clusters")


@if_test_sa
def test_sa_for_user_clusters():
    cmd = "get serviceaccounts -n default"
    out = execute_kubectl_cmd(cmd, False, False)
    assert "netes-default" not in out
    cmd = "get serviceaccounts -n cattle-system"
    out = execute_kubectl_cmd(cmd, False, False)
    assert "kontainer-engine" in out


@pytest.fixture(scope='module', autouse="True")
def create_cluster_client(request):
    client, cluster = get_user_client_and_cluster()
    create_kubeconfig(cluster)
