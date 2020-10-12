import pytest
from .common import *  # NOQA
from .test_rke_cluster_provisioning import HOST_NAME
from .test_rke_cluster_provisioning import create_and_validate_custom_host
from .test_rke_cluster_provisioning import rke_config

namespace = {"p_client": None, "ns": None, "cluster": None, "project": None,
             "nodes": []}
backup_info = {"backupname": None, "backup_id": None, "workload": None,
               "backupfilename": None, "etcdbackupdata": None}


@if_test_all_snapshot
def test_bkp_restore_s3_recover_validate():
    """
    - This test create 1 cluster with s3 backups enabled
        - 1 ControlPlane/worker node
        - 2 worker nodes
        - 3 etcd nodes
    - Creates an Ingress pointing to a workload
    - Snapshots the cluster and checks the backup is in S3
    - Stops the etcd nodes in ec2
    - Waits for the cluster to go into unavailable state
    - Removes all 3 etcd nodes
    - Creates 3 new etcd nodes and waits until the cluster
      asks to restore from backup.
    - Restores from S3 backup
    - Cluster is validated after it gets in Active state.
    - Checks the Ingress created before the backup is functional after restore.
    - Removes cluster if RANCHER_CLEANUP_CLUSTER=True
    """

    cluster = namespace["cluster"]
    client = namespace["client"]
    ns, b_info = validate_backup_create(namespace, backup_info, "s3")
    ips_to_remove = []
    etcd_nodes = get_etcd_nodes(client, cluster)
    assert len(etcd_nodes) > 0, "Make sure we have etcd nodes in the cluster"

    # stop the etcd ec2 instances
    [stop_node_from_ec2(etcd_node.externalIpAddress)
     for etcd_node in etcd_nodes]
    # wait for cluster to get into unavailable state
    cluster = wait_for_cluster_unavailable_or_error(client, cluster)
    for etcd_node in etcd_nodes:
        ips_to_remove.append(etcd_node.customConfig['internalAddress'])
        client.delete(etcd_node)
        wait_for_node_to_be_deleted(client, etcd_node)
    # Also remove the ec2 instances
    for ip_to_remove in ips_to_remove:
        delete_node_from_ec2(ip_to_remove)
        namespace["nodes"] = [node for node
                              in namespace["nodes"]
                              if node.private_ip_address != ip_to_remove]
    ips_to_remove.clear()
    cluster = client.reload(cluster)
    wait_for_cluster_node_count(client, cluster, 3)
    # Add completely new etcd nodes to the cluster
    cluster = add_new_etcd_nodes(client, cluster)
    cluster = client.reload(cluster)
    wait_for_cluster_node_count(client, cluster, 6)
    # This message is expected to appear after we add new etcd nodes
    # The cluster will require the user to perform a backup to recover
    # this is appears in the cluster object in cluster.transitioningMessage
    message = "Please restore your cluster from backup"
    cluster = wait_for_cluster_transitioning_message(client, cluster, message)
    etcd_nodes = get_etcd_nodes(client, cluster)
    assert len(etcd_nodes) == 3, "Make sure the cluster now has 3 etcd nodes"
    cluster = restore_backup_to_recover(client, cluster, b_info)
    # validate the ingress that was created in the first backup
    # after restoring and recovering the cluster
    validate_ingress(namespace["p_client"], cluster,
                     [b_info["workload"]], ns["host"],
                     "/name.html")


@pytest.fixture(scope='module', autouse=True)
def create_project_client_and_cluster_s3_three_etcd(request):
    node_roles = [
        ["controlplane", "worker"],
        ["etcd"], ["etcd"], ["etcd"],
        ["worker"], ["worker"]
    ]
    rke_config["services"]["etcd"]["backupConfig"] = {
        "enabled": "true",
        "intervalHours": 6,
        "retention": 3,
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
    cluster, aws_nodes = create_and_validate_custom_host(
        node_roles,
        random_cluster_name=True
    )
    client = get_user_client()
    namespace["nodes"].extend(aws_nodes)

    p, ns = create_project_and_ns(USER_TOKEN, cluster, "testrecover")
    p_client = get_project_client_for_token(p, USER_TOKEN)
    c_client = get_cluster_client_for_token(cluster, USER_TOKEN)

    namespace["p_client"] = p_client
    namespace["ns"] = ns
    namespace["cluster"] = cluster
    namespace["project"] = p
    namespace["c_client"] = c_client
    namespace["client"] = client

    def fin():
        cluster_cleanup(client, cluster, namespace["nodes"])
    request.addfinalizer(fin)


def wait_for_cluster_unavailable_or_error(client, cluster):
    return wait_for_condition(
        client,
        cluster,
        lambda x: x.state == "unavailable" or x.state == "error",
        lambda x: "State is: " + x.state,
        timeout=DEFAULT_CLUSTER_STATE_TIMEOUT,
    )


def wait_for_cluster_transitioning_message(client, cluster, message):
    start = time.time()
    while message not in cluster.transitioningMessage:
        print(cluster.transitioningMessage)
        time.sleep(5)
        cluster = client.reload(cluster)
        # We are waiting 4 minutes for the transitioning message to appear
        # this could be impacted by environmental factors
        if time.time() - start > DEFAULT_CLUSTER_STATE_TIMEOUT:
            raise Exception('Timeout waiting for condition')
    return cluster


def add_new_etcd_nodes(client, cluster, no_of_nodes=3):
    aws_nodes = AmazonWebServices().create_multiple_nodes(
        no_of_nodes, random_test_name(HOST_NAME))
    for aws_node in aws_nodes:
        docker_run_cmd = \
            get_custom_host_registration_cmd(client, cluster, ["etcd"],
                                             aws_node)
        print("Docker run command: " + docker_run_cmd)
        aws_node.roles.append("etcd")

        result = aws_node.execute_command(docker_run_cmd)
        namespace["nodes"].append(aws_node)
        print(result)
    return cluster


def delete_node_from_ec2(internal_ip):

    filters = [
        {'Name': 'private-ip-address',
         'Values': [internal_ip]}
    ]
    aws_node = AmazonWebServices().get_nodes(filters)
    if len(aws_node) > 0:
        AmazonWebServices().delete_node(aws_node[0])


def stop_node_from_ec2(address):
    filters = [
        {'Name': 'ip-address',
         'Values': [address]}
    ]
    aws_node = AmazonWebServices().get_nodes(filters)
    if len(aws_node) > 0:
        AmazonWebServices().stop_node(aws_node[0])


def restore_backup_to_recover(client, cluster, b_info):
    cluster.restoreFromEtcdBackup(etcdBackupId=b_info["backup_id"])
    return validate_cluster(client, cluster, intermediate_state="updating",
                            check_intermediate_state=True,
                            skipIngresscheck=False)


def get_etcd_nodes(client, cluster):
    nodes = client.list_node(clusterId=cluster.id).data
    return [node for node in nodes if node.etcd is True]


def get_worker_nodes(client, cluster):
    nodes = client.list_node(clusterId=cluster.id).data
    return [node for node in nodes if node.worker is True]
