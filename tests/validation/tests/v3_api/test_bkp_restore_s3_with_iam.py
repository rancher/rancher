import pytest
from .common import *  # NOQA
from .test_rke_cluster_provisioning import rke_config, engine_install_url, \
    validate_rke_dm_host_2

namespace = {"p_client": None, "ns": None, "cluster": None, "project": None,
             "nodes": []}
backup_info = {"backupname": None, "backup_id": None, "workload": None,
               "backupfilename": None, "etcdbackupdata": None}


@if_test_all_snapshot
def test_bkp_restore_s3_with_iam_create():
    validate_backup_create(namespace, backup_info, "s3")


@if_test_all_snapshot
def test_bkp_restore_s3_with_iam_restore():
    ns, binfo = validate_backup_create(namespace, backup_info, "s3")
    validate_backup_restore(ns, binfo)


@if_test_all_snapshot
def test_bkp_restore_s3_with_iam_delete():
    ns, binfo = validate_backup_create(namespace, backup_info, "s3")
    ns, binfo = validate_backup_restore(ns, binfo)
    validate_backup_delete(ns, binfo, "s3")


@pytest.fixture(scope='module', autouse="True")
def create_project_client_and_cluster_s3_with_iam(request):
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
    validate_rke_dm_host_2(node_template_ec2_iam(),
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


def node_template_ec2_iam():
    client = get_user_client()
    ec2_cloud_credential_config = {"accessKey": AWS_ACCESS_KEY_ID,
                                   "secretKey": AWS_SECRET_ACCESS_KEY}
    ec2_cloud_credential = client.create_cloud_credential(
        amazonec2credentialConfig=ec2_cloud_credential_config
    )
    amazonec2Config = {
        "iamInstanceProfile": AWS_IAM_PROFILE,
        "instanceType": "t3a.medium",
        "CreditSpecification": {
            "CPUCredits": "standard"
        },
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
