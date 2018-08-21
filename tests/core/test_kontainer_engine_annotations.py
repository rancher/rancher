from .common import random_str
from .conftest import wait_until


def get_cluster_annotation(admin_mc, remove_resource, config):
    cluster = admin_mc.client.create_cluster(
        name=random_str(), amazonElasticContainerServiceConfig=config)
    remove_resource(cluster)

    def has_cluster_annotation():
        cluster2 = admin_mc.client.reload(cluster)

        return \
            hasattr(cluster2.annotations,
                    "clusterstatus.management.cattle.io/"
                    "temporary-security-credentials")

    wait_until(has_cluster_annotation)

    cluster = admin_mc.client.reload(cluster)

    return cluster.annotations[
        "clusterstatus.management.cattle.io/temporary-security-credentials"]


def test_eks_cluster_gets_temp_security_credentials_annotation(
        admin_mc, remove_resource):
    eks = {
        "accessKey": "not a real access key",
        "secretKey": "not a real secret key",
        "sessionToken": "not a real session token",
        "region": "us-west-2",
    }

    annotation = get_cluster_annotation(admin_mc, remove_resource, eks)
    assert annotation == "true"


def test_eks_cluster_does_not_get_temp_security_credentials_annotation(
        admin_mc, remove_resource):
    eks = {
        "accessKey": "not a real access key",
        "secretKey": "not a real secret key",
        "region": "us-west-2",
    }

    annotation = get_cluster_annotation(admin_mc, remove_resource, eks)
    assert annotation == "false"
