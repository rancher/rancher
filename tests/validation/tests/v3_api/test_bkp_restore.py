import pytest
from .common import *  # NOQA

namespace = {"p_client": None, "ns": None, "cluster": None, "project": None}


def test_bkp_restore():
    p_client = namespace["p_client"]
    ns = namespace["ns"]
    client = get_user_client()
    cluster = namespace["cluster"]
    name = random_test_name("default")

    if not hasattr(cluster, 'rancherKubernetesEngineConfig'):
        assert False, "Cluster is not of type RKE"

    con = [{"name": "test1",
           "image": TEST_IMAGE}]
    workload = p_client.create_workload(name=name,
                                        containers=con,
                                        namespaceId=ns.id,
                                        daemonSetConfig={})
    validate_workload(p_client, workload, "daemonSet", ns.name,
                      len(get_schedulable_nodes(cluster)))
    host = "test1.com"
    path = "/name.html"
    rule = {"host": host,
            "paths": [{"workloadIds": [workload.id], "targetPort": "80"}]}
    p_client.create_ingress(name=name,
                            namespaceId=ns.id,
                            rules=[rule])
    validate_ingress(namespace["p_client"], namespace["cluster"],
                     [workload], host, path)

    # Perform Backup
    backup = cluster.backupEtcd()
    backupname = backup['metadata']['name']
    etcdbackups = cluster.etcdBackups(name=backupname)
    etcdbackupdata = etcdbackups['data']
    backupId = etcdbackupdata[0]['id']
    wait_for_backup_to_active(client, cluster, backupname)

    # Create workload after backup
    testworkload = p_client.create_workload(name=name,
                                            containers=con,
                                            namespaceId=ns.id)

    validate_workload(p_client, testworkload, "deployment", ns.name)

    # Perform Restore
    cluster.restoreFromEtcdBackup(etcdBackupId=backupId)
    # After restore, validate cluster
    validate_cluster(client, cluster, intermediate_state="updating",
                     check_intermediate_state=True,
                     skipIngresscheck=False)

    # Verify the ingress created before taking the snapshot
    validate_ingress(namespace["p_client"], namespace["cluster"],
                     [workload], host, path)

    # Verify the workload created after getting a snapshot does not exist
    # after restore
    workload_list = p_client.list_workload(uuid=testworkload.uuid).data
    print(len(workload_list))
    assert len(workload_list) == 0


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
        client.delete(namespace["project"])
    request.addfinalizer(fin)


def wait_for_backup_to_active(client, cluster, backupname,
                              timeout=DEFAULT_TIMEOUT):
    start = time.time()
    etcdbackups = cluster.etcdBackups(name=backupname)
    assert len(etcdbackups) == 1
    etcdbackupdata = etcdbackups['data']
    etcdbackupstate = etcdbackupdata[0]['state']
    while etcdbackupstate != "active":
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for state to get to active")
        time.sleep(.5)
        etcdbackups = cluster.etcdBackups(name=backupname)
        assert len(etcdbackups) == 1
        etcdbackupdata = etcdbackups['data']
        etcdbackupstate = etcdbackupdata[0]['state']
    print("BACKUP STATE")
    print(etcdbackupstate)
    return etcdbackupstate
