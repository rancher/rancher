from .common import random_str
from .conftest import wait_until


def assert_has_error_message(admin_mc, remove_resource, eks, message):
    cluster = admin_mc.client.create_cluster(
        name=random_str(), amazonElasticContainerServiceConfig=eks)
    remove_resource(cluster)

    def get_provisioned_type(cluster):
        for condition in cluster.conditions:
            if condition.type == "Provisioned":
                if hasattr(condition, 'message'):
                    return condition.message
        return None

    def has_provision_status():
        new_cluster = admin_mc.client.reload(cluster)

        return \
            hasattr(new_cluster, "conditions") and \
            get_provisioned_type(new_cluster) is not None

    def has_error_message():
        for condition in cluster.conditions:
            if condition.type == "Provisioned":
                if getattr(condition, 'message') == message:
                    return True

        return False

    wait_until(has_provision_status)
    cluster = admin_mc.client.reload(cluster)

    wait_until(has_error_message, timeout=120, backoff=False)
    cluster = admin_mc.client.reload(cluster)

    assert has_error_message(), "no error message %r was present" % \
                                message


def test_min_nodes_cannot_be_greater_than_max(admin_mc, remove_resource):
    eks = {
        "accessKey": "not a real access key",
        "secretKey": "not a real secret key",
        "region": "us-west-2",
        "kubernetesVersion": "1.14",
        "minimumNodes": 3,
        "maximumNodes": 2
    }

    assert_has_error_message(admin_mc, remove_resource, eks,
                             "error parsing state: maximum nodes cannot "
                             "be less than minimum nodes")


def test_min_nodes_cannot_be_zero(admin_mc, remove_resource):
    eks = {
        "accessKey": "not a real access key",
        "secretKey": "not a real secret key",
        "region": "us-west-2",
        "kubernetesVersion": "1.14",
        "minimumNodes": 0,
        "maximumNodes": 0
    }
    assert_has_error_message(admin_mc, remove_resource, eks,
                             "error parsing state: minimum nodes must be "
                             "greater than 0")


def test_node_volume_size_cannot_be_zero(admin_mc, remove_resource):
    eks = {
        "accessKey": "not a real access key",
        "secretKey": "not a real secret key",
        "region": "us-west-2",
        "kubernetesVersion": "1.14",
        "minimumNodes": 1,
        "maximumNodes": 3,
        "nodeVolumeSize": 0
    }
    assert_has_error_message(admin_mc, remove_resource, eks,
                             "error parsing state: node volume size must "
                             "be greater than 0")


def test_private_cluster_requires_vpc_subnets(admin_mc, remove_resource):
    eks = {
        "accessKey": "not a real access key",
        "secretKey": "not a real secret key",
        "region": "us-west-2",
        "kubernetesVersion": "1.14",
        "minimumNodes": 1,
        "maximumNodes": 3,
        "associateWorkerNodePublicIp": False
    }
    assert_has_error_message(admin_mc, remove_resource, eks,
                             "error parsing state: if "
                             "AssociateWorkerNodePublicIP is set to "
                             "false a VPC and subnets must also be provided")
