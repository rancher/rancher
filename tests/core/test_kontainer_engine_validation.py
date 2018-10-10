from .common import random_str
from .conftest import wait_until


def get_error_message_for_eks_config(admin_mc, remove_resource, config):
    cluster = admin_mc.client.create_cluster(
        name=random_str(), amazonElasticContainerServiceConfig=config)
    remove_resource(cluster)

    def has_provision_status():
        new_cluster = admin_mc.client.reload(cluster)

        return hasattr(new_cluster, "conditions")

    wait_until(has_provision_status, 10)
    cluster = admin_mc.client.reload(cluster)

    for condition in cluster.conditions:
        if condition.type == "Provisioned":
            return condition.message


def test_min_nodes_cannot_be_greater_than_max(admin_mc, remove_resource):
    eks = {
        "accessKey": "not a real access key",
        "secretKey": "not a real secret key",
        "region": "us-west-2",
        "minimumNodes": 3,
        "maximumNodes": 2
    }
    provision_message = \
        get_error_message_for_eks_config(admin_mc, remove_resource, eks)

    assert provision_message == "error parsing state: maximum nodes cannot " \
                                "be less than minimum nodes"


def test_min_nodes_cannot_be_zero(admin_mc, remove_resource):
    eks = {
        "accessKey": "not a real access key",
        "secretKey": "not a real secret key",
        "region": "us-west-2",
        "minimumNodes": 0,
        "maximumNodes": 0
    }
    provision_message = get_error_message_for_eks_config(admin_mc,
                                                         remove_resource, eks)

    assert provision_message == "error parsing state: minimum nodes must be " \
                                "greater than 0"


def test_private_cluster_requires_vpc_subnets(admin_mc, remove_resource):
    eks = {
        "accessKey": "not a real access key",
        "secretKey": "not a real secret key",
        "region": "us-west-2",
        "minimumNodes": 1,
        "maximumNodes": 3,
        "associateWorkerNodePublicIp": False
    }
    provision_message = get_error_message_for_eks_config(admin_mc,
                                                         remove_resource, eks)

    assert \
        provision_message == \
        "error parsing state: if AssociateWorkerNodePublicIP is set to " \
        "false a VPC and subnets must also be provided"
