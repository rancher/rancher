from .common import random_str
from .conftest import wait_until, wait_for

annotation = "clusterstatus.management.cattle.io/" \
             "temporary-security-credentials"
access_key = "accessKey"
secret_key = "secretKey"
session_token = "sessionToken"
region = "region"

"""
There are effectively 2 ways that an EKS cluster will get a temporary \
security credentials annotation.  The first way is if it is created with \
a session token, then an annotation will be added in the \
cluster_store.go.  The other way is if a cluster is edited to add a \
session token.  In this case a controller will watch for the change and \
apply the annotation.  We test for both of those scenarios here.
"""


def has_cluster_annotation(client, cluster, expected=None):
    def poll():
        cluster2 = client.reload(cluster)

        has_attribute = hasattr(cluster2.annotations, annotation)

        if expected is not None:
            return has_attribute and cluster2.annotations[annotation] == \
                   expected
        else:
            return has_attribute

    return poll


def assert_cluster_annotation(expected, admin_mc, remove_resource, config):
    cluster = admin_mc.client.create_cluster(
        name=random_str(), amazonElasticContainerServiceConfig=config)
    remove_resource(cluster)

    assert cluster.annotations[annotation] == expected

    wait_until(has_cluster_annotation(admin_mc.client, cluster))

    cluster = admin_mc.client.reload(cluster)

    assert cluster.annotations[annotation] == expected

    return cluster


def test_eks_cluster_gets_temp_security_credentials_annotation(
        admin_mc, remove_resource):
    eks = {
        access_key: "not a real access key",
        secret_key: "not a real secret key",
        session_token: "not a real session token",
        region: "us-west-2",
    }

    assert_cluster_annotation("true", admin_mc, remove_resource, eks)


def test_eks_does_not_get_temp_security_creds_annotation_no_field(
        admin_mc, remove_resource):
    eks = {
        access_key: "not a real access key",
        secret_key: "not a real secret key",
        region: "us-west-2",
    }

    assert_cluster_annotation("false", admin_mc, remove_resource, eks)


def test_eks_does_not_get_temp_security_creds_annotation_empty_field(
        admin_mc, remove_resource):
    eks = {
        access_key: "not a real access key",
        secret_key: "not a real secret key",
        session_token: "",
        region: "us-west-2",
    }

    assert_cluster_annotation("false", admin_mc, remove_resource, eks)


def test_editing_eks_cluster_gives_temp_creds_annotation(
        admin_mc, remove_resource):
    eks = {
        access_key: "not a real access key",
        secret_key: "not a real secret key",
        region: "us-west-2",
    }

    cluster = assert_cluster_annotation("false", admin_mc, remove_resource,
                                        eks)
    eks = cluster.amazonElasticContainerServiceConfig
    setattr(eks, session_token, "not a real session token")
    cluster = admin_mc.client.update_by_id_cluster(
        id=cluster.id,
        name=cluster.name,
        amazonElasticContainerServiceConfig=eks
    )

    wait_for(has_cluster_annotation(admin_mc.client, cluster,
                                    expected="true"), timeout=120)

    cluster = admin_mc.client.reload(cluster)

    assert cluster.annotations[annotation] == "true"
