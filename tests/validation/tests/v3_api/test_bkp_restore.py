from urllib.parse import urlparse

import pytest
from .common import *  # NOQA
from .test_rke_cluster_provisioning import rke_config, engine_install_url, \
    node_template_linode, validate_rke_dm_host_2

namespace = {"p_client": None, "ns": None, "cluster": None, "project": None}


def test_bkp_restore_local(create_project_client):
    validate_backup_create_restore_delete("local")


@if_test_s3
def test_bkp_restore_s3_with_creds(
        create_project_client_and_cluster_s3_with_creds):
    validate_backup_create_restore_delete("s3")


@if_test_s3
def test_bkp_restore_s3_with_iam(
        create_project_client_and_cluster_s3_with_iam):
    validate_backup_create_restore_delete("s3")


def validate_backup_create_restore_delete(backup_mode):
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
    validate_ingress(p_client, cluster, [workload], host, path)

    # Perform Backup
    backup = cluster.backupEtcd()
    backupname = backup['metadata']['name']
    wait_for_backup_to_active(cluster, backupname)

    # Get all the backup info
    etcdbackups = cluster.etcdBackups(name=backupname)
    etcdbackupdata = etcdbackups['data']
    backupId = etcdbackupdata[0]['id']

    backupfilename = ""
    if backup_mode == "s3":
        backupfileurl = etcdbackupdata[0]['filename']
        # Check the backup filename exists in S3
        parseurl = urlparse(backupfileurl)
        backupfilename = os.path.basename(parseurl.path)
        assert AmazonWebServices().s3_backup_check(backupfilename)

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
    validate_ingress(p_client, cluster, [workload], host, path)

    # Verify the workload created after getting a snapshot does not exist
    # after restore
    workload_list = p_client.list_workload(uuid=testworkload.uuid).data
    print(len(workload_list))
    assert len(workload_list) == 0

    if backup_mode == "s3":
        # Check the backup reference is deleted in Rancher and S3
        client.delete(cluster.etcdBackups(id=backupId)['data'][0])
        wait_for_backup_to_delete(cluster, backupname)
        assert AmazonWebServices().s3_backup_check(backupfilename) is False


@pytest.fixture(scope='module')
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
        cluster_cleanup(client, cluster)
    request.addfinalizer(fin)


@pytest.fixture(scope='session')
def create_project_client_and_cluster_s3_with_creds(node_template_linode, request):
    rke_config["services"]["etcd"]["backupConfig"] = {
        "enabled": "true",
        "intervalHours": 12,
        "retention": 6,
        "type": "backupConfig",
        "s3BackupConfig": {
            "type": "s3BackupConfig",
            "accessKey": AWS_ACCESS_KEY_ID,
            "secretKey": AWS_SECRET_ACCESS_KEY,
            "bucketName": AWS_S3_BUCKET_NAME,
            "folder": AWS_S3_BUCKET_FOLDER_NAME,
            "region": AWS_REGION,
            "endpoint": "s3.amazonaws.com"
        }
    }
    cluster_name = random_name()
    validate_rke_dm_host_2(node_template_linode,
                           rke_config, False, cluster_name)
    client = get_user_client()
    cluster = get_cluster_by_name(client, cluster_name)

    p, ns = create_project_and_ns(USER_TOKEN, cluster, "testnoiam")
    p_client = get_project_client_for_token(p, USER_TOKEN)
    c_client = get_cluster_client_for_token(cluster, USER_TOKEN)

    namespace["p_client"] = p_client
    namespace["ns"] = ns
    namespace["cluster"] = cluster
    namespace["project"] = p
    namespace["c_client"] = c_client

    def fin():
        client = get_user_client()
        cluster_cleanup(client, cluster)
    request.addfinalizer(fin)


@pytest.fixture(scope='session')
def create_project_client_and_cluster_s3_with_iam(node_template_ec2_iam, request):
    rke_config["services"]["etcd"]["backupConfig"] = {
        "enabled": "true",
        "intervalHours": 12,
        "retention": 6,
        "type": "backupConfig",
        "s3BackupConfig": {
            "type": "s3BackupConfig",
            "accessKey": "",
            "secretKey": "",
            "bucketName": AWS_S3_BUCKET_NAME,
            "folder": AWS_S3_BUCKET_FOLDER_NAME,
            "region": AWS_REGION,
            "endpoint": "s3.amazonaws.com"
        }
    }
    cluster_name = random_name()
    validate_rke_dm_host_2(node_template_ec2_iam,
                           rke_config, False, cluster_name)
    client = get_user_client()
    cluster = get_cluster_by_name(client, cluster_name)

    p, ns = create_project_and_ns(USER_TOKEN, cluster, "testiam")
    p_client = get_project_client_for_token(p, USER_TOKEN)
    c_client = get_cluster_client_for_token(cluster, USER_TOKEN)

    namespace["p_client"] = p_client
    namespace["ns"] = ns
    namespace["cluster"] = cluster
    namespace["project"] = p
    namespace["c_client"] = c_client

    def fin():
        client = get_user_client()
        cluster_cleanup(client, cluster)
    request.addfinalizer(fin)


@pytest.fixture(scope='session')
def node_template_ec2_iam():
    client = get_user_client()
    ec2_cloud_credential_config = {"accessKey": AWS_ACCESS_KEY_ID,
                                   "secretKey": AWS_SECRET_ACCESS_KEY}
    ec2_cloud_credential = client.create_cloud_credential(
        amazonec2credentialConfig=ec2_cloud_credential_config
    )
    amazonec2Config = {
        "iamInstanceProfile": AWS_IAM_PROFILE,
        "instanceType": "t2.medium",
        "region": AWS_REGION,
        "rootSize": "16",
        "securityGroup": [AWS_SG],
        "sshUser": "ubuntu",
        "subnetId": AWS_SUBNET,
        "usePrivateAddress": False,
        "volumeType": "gp2",
        "vpcId": AWS_VPC,
        "zone": AWS_ZONE
    }

    node_template = client.create_node_template(
        amazonec2Config=amazonec2Config,
        name=random_name(),
        useInternalIpAddress=True,
        driver="amazonec2",
        engineInstallURL=engine_install_url,
        cloudCredentialId=ec2_cloud_credential.id

    )
    node_template = client.wait_success(node_template)
    return node_template


def wait_for_backup_to_active(cluster, backupname,
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


def wait_for_backup_to_delete(cluster, backupname,
                              timeout=DEFAULT_TIMEOUT):
    start = time.time()
    etcdbackups = cluster.etcdBackups(name=backupname)
    while len(etcdbackups) == 1:
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for backup to be deleted")
        time.sleep(.5)
        etcdbackups = cluster.etcdBackups(name=backupname)
