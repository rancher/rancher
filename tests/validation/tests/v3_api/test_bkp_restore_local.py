import pytest
from rancher import ApiError
from .common import *  # NOQA

namespace = {"p_client": None, "ns": None, "cluster": None, "project": None,
             "nodes": []}
backup_info = {"backupname": None, "backup_id": None, "workload": None,
               "backupfilename": None, "etcdbackupdata": None}

rbac_roles = [CLUSTER_MEMBER, PROJECT_OWNER, PROJECT_MEMBER, PROJECT_READ_ONLY]

def test_bkp_restore_local_create():
    validate_backup_create(namespace, backup_info)


def test_bkp_restore_local_restore():
    ns , binfo = validate_backup_create(namespace, backup_info)
    validate_backup_restore(ns, binfo)


def test_bkp_restore_local_delete():
    ns , binfo = validate_backup_create(namespace, backup_info)
    ns, binfo = validate_backup_restore(ns, binfo)
    validate_backup_delete(ns, binfo)


@if_test_rbac
def test_rbac_bkp_restore_create_cluster_owner():
    """ Only cluster-owner should be allowed to create backups """
    user_token = rbac_get_user_token_by_role(CLUSTER_OWNER)
    user_client = get_client_for_token(user_token)
    user_cluster = user_client.list_cluster(name=CLUSTER_NAME).data[0]
    backup = user_cluster.backupEtcd()
    backupname = backup['metadata']['name']
    wait_for_backup_to_active(user_cluster, backupname)
    assert len(user_cluster.etcdBackups(name=backupname)) == 1


@if_test_rbac
@pytest.mark.parametrize("role", rbac_roles)
def test_rbac_bkp_restore_create(role):
    """
    Only cluster-owner should be allowed to create backups
    unprivileged user should get 403 PermissionDenied
    """
    user_token = rbac_get_user_token_by_role(role)
    user_client = get_client_for_token(user_token)
    user_cluster = user_client.list_cluster(name=CLUSTER_NAME).data[0]
    with pytest.raises(ApiError) as e:
        user_cluster.backupEtcd()
    assert e.value.error.status == 403
    assert e.value.error.code == 'PermissionDenied'

@if_test_rbac
def test_rbac_bkp_restore_list_cluster_owner():
    """ Only cluster-owner should be allowed to list backups """
    user_token = rbac_get_user_token_by_role(CLUSTER_OWNER)
    user_client = get_client_for_token(user_token)
    user_cluster = user_client.list_cluster(name=CLUSTER_NAME).data[0]
    backup = user_cluster.backupEtcd()
    backupname = backup['metadata']['name']
    assert len(user_cluster.etcdBackups(name=backupname)) == 1


@if_test_rbac
@pytest.mark.parametrize("role", rbac_roles)
def test_rbac_bkp_restore_list(role):
    """
    unprivileged user should not be allowed to list backups
    cluster etcdBackups() should always return length zero
    """
    user_token = rbac_get_user_token_by_role(CLUSTER_OWNER)
    user_client = get_client_for_token(user_token)
    user_cluster = user_client.list_cluster(name=CLUSTER_NAME).data[0]
    backup = user_cluster.backupEtcd()
    backupname = backup['metadata']['name']
    assert len(user_cluster.etcdBackups(name=backupname)) == 1
    wait_for_backup_to_active(user_cluster, backupname)
    user_token2 = rbac_get_user_token_by_role(role)
    user_client = get_client_for_token(user_token2)
    user_cluster2 = user_client.list_cluster(name=CLUSTER_NAME).data[0]
    assert len(user_cluster2.etcdBackups()) == 0


@if_test_rbac
@pytest.mark.parametrize("role", rbac_roles)
def test_rbac_bkp_restore_restore(role):
    """
    unprivileged user should not be allowed to restore backups
    """
    user_token = rbac_get_user_token_by_role(CLUSTER_OWNER)
    user_client = get_client_for_token(user_token)
    user_cluster = user_client.list_cluster(name=CLUSTER_NAME).data[0]
    backup = user_cluster.backupEtcd()
    backupname = backup['metadata']['name']
    etcdbackup = user_cluster.etcdBackups(name=backupname)
    backup_id = etcdbackup['data'][0]['id']
    wait_for_backup_to_active(user_cluster, backupname)

    user_token2 = rbac_get_user_token_by_role(role)
    user_client2 = get_client_for_token(user_token2)
    user_cluster2 = user_client2.list_cluster(name=CLUSTER_NAME).data[0]
    with pytest.raises(ApiError) as e:
        user_cluster2.restoreFromEtcdBackup(etcdBackupId=backup_id)
    assert e.value.error.status == 403
    assert e.value.error.code == 'PermissionDenied'


@if_test_rbac
def test_rbac_bkp_restore_delete_cluster_owner():
    """ Only cluster-owner should be allowed to delete backups """
    user_token = rbac_get_user_token_by_role(CLUSTER_OWNER)
    user_client = get_client_for_token(user_token)
    user_cluster = user_client.list_cluster(name=CLUSTER_NAME).data[0]
    backup = user_cluster.backupEtcd()
    backupname = backup['metadata']['name']
    wait_for_backup_to_active(user_cluster, backupname)
    assert len(user_cluster.etcdBackups(name=backupname)) == 1
    user_client.delete(
        user_cluster.etcdBackups(name=backupname)['data'][0]
    )
    wait_for_backup_to_delete(user_cluster, backupname)
    assert len(user_cluster.etcdBackups(name=backupname)) == 0


@if_test_rbac
@pytest.mark.parametrize("role", rbac_roles)
def test_rbac_bkp_restore_delete(role):
    """
    Only cluster-owner should be allowed to delete backups
    unprivileged user shouldn't be allowed to delete
    """
    user_token = rbac_get_user_token_by_role(CLUSTER_OWNER)
    user_client = get_client_for_token(user_token)
    user_cluster = user_client.list_cluster(name=CLUSTER_NAME).data[0]
    backup = user_cluster.backupEtcd()
    backupname = backup['metadata']['name']
    wait_for_backup_to_active(user_cluster, backupname)

    user_token2 = rbac_get_user_token_by_role(role)
    user_client2 = get_client_for_token(user_token2)
    user_cluster2 = user_client2.list_cluster(name=CLUSTER_NAME).data[0]

    assert len(user_cluster2.etcdBackups(name=backupname)) == 0
    backup_to_delete = user_cluster.etcdBackups(name=backupname)['data'][0]
    with pytest.raises(ApiError) as e:
        user_client2.delete(backup_to_delete)
    assert e.value.error.status == 403
    assert e.value.error.code == 'Forbidden'


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
