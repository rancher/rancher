import pytest
from .common import *  # NOQA
from .test_rke_cluster_provisioning import create_and_validate_custom_host

namespace = {"p_client": None, "ns": None, "cluster": None, "project": None,
             "nodes": []}
backup_info = {"backupname": None, "backup_id": None, "workload": None,
               "backupfilename": None, "etcdbackupdata": None}


@if_test_all_snapshot
def test_bkp_restore_local_with_snapshot_check_create():
    validate_backup_create(namespace, backup_info, "filesystem")


@if_test_all_snapshot
def test_bkp_restore_local_with_snapshot_check_restore():
    ns, binfo = validate_backup_create(namespace, backup_info, "filesystem")
    validate_backup_restore(ns, binfo)


@if_test_all_snapshot
def test_bkp_restore_local_with_snapshot_check_delete():
    ns, binfo = validate_backup_create(namespace, backup_info, "filesystem")
    ns, binfo = validate_backup_restore(ns, binfo)
    validate_backup_delete(ns, binfo, "filesystem")


@pytest.fixture(scope='module', autouse="True")
def create_project_client_ec2(request):
    node_roles = [["controlplane"], ["etcd"],
                  ["worker"], ["worker"], ["worker"]]
    cluster, aws_nodes = create_and_validate_custom_host(node_roles, True)
    client = get_user_client()
    p, ns = create_project_and_ns(USER_TOKEN, cluster, "testsecret")
    p_client = get_project_client_for_token(p, USER_TOKEN)
    c_client = get_cluster_client_for_token(cluster, USER_TOKEN)
    namespace["p_client"] = p_client
    namespace["ns"] = ns
    namespace["cluster"] = cluster
    namespace["project"] = p
    namespace["c_client"] = c_client
    namespace["nodes"] = aws_nodes.copy()

    def fin():
        client.delete(p)
        cluster_cleanup(client, cluster, aws_nodes)
    request.addfinalizer(fin)
