import os
import pytest
import time
from lib.aws import AmazonWebServices
from rancher import ApiError

from .common import (
    AWS_ACCESS_KEY_ID,
    AWS_SECRET_ACCESS_KEY,
    AWS_REGION,
    AWS_SG,
    AWS_SUBNET,
    AWS_VPC,
    AWS_ZONE,
    DEFAULT_TIMEOUT,
    cluster_cleanup,
    get_user_client,
    random_name,
    wait_for_cluster_delete,
    wait_for_nodes_to_become_active
)
from .test_rke_cluster_provisioning import (
    validate_rke_dm_host_1,
    engine_install_url
)


def test_provision_encrypted_instance(client,
                                      encrypted_cluster_nodepool):
    """
    Provisions an EC2 nodepool with encrypted EBS volume backed instances by providing
    a flag on the node template and ensures that the provisioned instances are encrypted
    """
    cluster, nodepools = encrypted_cluster_nodepool
    aws_nodes = get_aws_nodes_from_nodepools(client, cluster, nodepools)
    check_if_volumes_are_encrypted(aws_nodes)


def get_aws_nodes_from_nodepools(client, cluster, nodepools):
    """
    Retrieves the AWS Nodes related to the nodes in the nodepool so that
    methods invoking the AWS CLI defined in aws.py can be called on the nodes
    """
    wait_for_nodes_to_become_active(client, cluster)
    aws_nodes = []
    for nodepool in nodepools:
        nodes = nodepool.nodes().data
        for node in nodes:
            node_ip_address = node['ipAddress']
            ip_address_filter = [{
                    'Name': 'private-ip-address', 'Values': [node_ip_address]}]
            nodes = AmazonWebServices().get_nodes(ip_address_filter)
            assert len(nodes) == 1, \
                "Multiple aws_nodes seem to have private-ip-address %s" \
                % node_ip_address
            aws_nodes.append(nodes[0])
    return aws_nodes


def check_if_volumes_are_encrypted(aws_nodes):
    """
    Given a set of AWS Nodes, return whether the nodes have encrypted EBS volumes
    """
    for aws_node in aws_nodes:
        provider_node_id = aws_node.provider_node_id
        volumes = AmazonWebServices().get_ebs_volumes(provider_node_id)
        for volume in volumes:
            assert volume['Encrypted']


@pytest.fixture('module')
def client():
    """
    A user client to be used in tests
    """
    return get_user_client()


@pytest.fixture(scope='module')
def node_template_ec2_with_encryption(client):
    """
    A node template that defines a set of encrypted EC2 volume backed instances
    """
    def _attempt_delete_node_template(client, node_template,
                                      timeout=DEFAULT_TIMEOUT,
                                      sleep_time=.5):
        start = time.time()
        while node_template:
            if time.time() - start > timeout:
                raise AssertionError(
                    "Timed out waiting for node template %s to get deleted"
                    % node_template["name"])
            time.sleep(sleep_time)
            client.reload(node_template)
            try:
                client.delete(node_template)
                break
            except ApiError:
                pass
            except Exception as e:
                raise e

    ec2_cloud_credential_config = {"accessKey": AWS_ACCESS_KEY_ID,
                                   "secretKey": AWS_SECRET_ACCESS_KEY}
    ec2_cloud_credential = client.create_cloud_credential(
        amazonec2credentialConfig=ec2_cloud_credential_config
    )
    amazonec2Config = {
        "instanceType": "t2.medium",
        "region": AWS_REGION,
        "rootSize": "16",
        "securityGroup": [AWS_SG],
        "sshUser": "ubuntu",
        "subnetId": AWS_SUBNET,
        "usePrivateAddress": False,
        "volumeType": "gp2",
        "vpcId": AWS_VPC,
        "zone": AWS_ZONE,
        "encryptEbsVolume": True
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
    yield node_template
    _attempt_delete_node_template(client, node_template)


@pytest.fixture('module')
def encrypted_cluster_nodepool(client, node_template_ec2_with_encryption):
    """
    Returns a cluster with a single nodepool of encrypted EBS volume backed EC2 instances
    """
    cluster, nodepools = validate_rke_dm_host_1(
        node_template_ec2_with_encryption,
        attemptDelete=False)
    yield (cluster, nodepools)
    cluster_cleanup(client, cluster)
    wait_for_cluster_delete(client, cluster["name"])
