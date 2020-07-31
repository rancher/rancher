import pytest
import os
from .common import create_kubeconfig
from .common import DATA_SUBDIR
from .common import get_user_client_and_cluster
from .common import test_reader
from .common import validate_cluster_role_rbac


@pytest.fixture(scope='module', autouse="True")
def create_project_client():
    client, cluster = get_user_client_and_cluster()
    create_kubeconfig(cluster)


@pytest.mark.parametrize("cluster_role, command, authorization, name",
                         test_reader(os.path.join(
                             DATA_SUBDIR,
                             'rbac/monitoring/monitoring_rbac.json')))
def test_monitoring_v2_rbac(cluster_role, command, authorization, name):
    validate_cluster_role_rbac(cluster_role, command, authorization, name)
