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
