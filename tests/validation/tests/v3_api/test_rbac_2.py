import pytest
import os
from .common import create_kubeconfig
from .common import DATA_SUBDIR
from .common import get_user_client_and_cluster
from .common import rbac_test_file_reader
from .common import validate_cluster_role_rbac
from .common import if_test_rbac_v2


@pytest.fixture(scope='module', autouse="True")
def create_project_client():
    client, cluster = get_user_client_and_cluster()
    create_kubeconfig(cluster)


@if_test_rbac_v2
@pytest.mark.parametrize("cluster_role, command, authorization, name",
                         rbac_test_file_reader(os.path.join(
                             DATA_SUBDIR,
                             'rbac/monitoring/monitoring_rbac.json')))
def test_monitoring_rbac_v2(cluster_role, command, authorization, name):
    validate_cluster_role_rbac(cluster_role, command, authorization, name)


@if_test_rbac_v2
@pytest.mark.parametrize("cluster_role, command, authorization, name",
                         rbac_test_file_reader(os.path.join(
                             DATA_SUBDIR,
                             'rbac/istio/istio_rbac.json')))
def test_istio_rbac_v2(cluster_role, command, authorization, name):
    validate_cluster_role_rbac(cluster_role, command, authorization, name)


@if_test_rbac_v2
@pytest.mark.parametrize("cluster_role, command, authorization, name",
                         rbac_test_file_reader(os.path.join(
                             DATA_SUBDIR,
                             'rbac/logging/logging_rbac.json')))
def test_logging_rbac_v2(cluster_role, command, authorization, name):
    validate_cluster_role_rbac(cluster_role, command, authorization, name)


@if_test_rbac_v2
@pytest.mark.parametrize("cluster_role, command, authorization, name",
                         rbac_test_file_reader(os.path.join(
                             DATA_SUBDIR,
                             'rbac/cis/cis_rbac.json')))
def test_cis_rbac_v2(cluster_role, command, authorization, name):
    validate_cluster_role_rbac(cluster_role, command, authorization, name)
