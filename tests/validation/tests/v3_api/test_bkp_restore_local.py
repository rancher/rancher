import pytest
from .common import *  # NOQA

namespace = {"p_client": None, "ns": None, "cluster": None, "project": None,
             "nodes": []}
backup_info = {"backupname": None, "backup_id": None, "workload": None,
               "backupfilename": None, "etcdbackupdata": None}


def test_bkp_restore_local_create():
    validate_backup_create(namespace, backup_info)


def test_bkp_restore_local_restore():
    ns , binfo = validate_backup_create(namespace, backup_info)
    validate_backup_restore(ns, binfo)


def test_bkp_restore_local_delete():
    ns , binfo = validate_backup_create(namespace, backup_info)
    ns, binfo = validate_backup_restore(ns, binfo)
    validate_backup_delete(ns, binfo)


@pytest.fixture(scope='module', autouse="True")
def create_project_client(request):
    client, cluster = get_user_client_and_cluster()
    create_kubeconfig(cluster)
    p, ns = create_project_and_ns(USER_TOKEN, cluster, "testsecret")
    p_client = get_project_client_for_token(p, USER_TOKEN)
    c_client = get_cluster_client_for_token(cluster, USER_TOKEN)
    namespace["p_client"] = p_client
    namespace["ns"] = ns
    namespace["cluster"] = cluster
    namespace["project"] = p
    namespace["c_client"] = c_client

    def fin():
        client = get_user_client()
        client.delete(p)
    request.addfinalizer(fin)
