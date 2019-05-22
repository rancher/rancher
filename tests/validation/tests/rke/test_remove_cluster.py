from .conftest import *  # NOQA
from .common import *  # NOQA


def test_remove_1(test_name, cloud_provider, rke_client, kubectl):
    """
    Create a three node cluster and runs validation to create pods
    Removes cluster and validates components are removed
    Then creates new cluster on the same nodes and validates
    """
    rke_template = 'cluster_install_config_1.yml.j2'
    nodes = cloud_provider.create_multiple_nodes(3, test_name)
    create_and_validate(
        cloud_provider, rke_client, kubectl, rke_template, nodes)
    rke_client.remove()

    validate_remove_cluster(nodes)

    create_and_validate(
        cloud_provider, rke_client, kubectl, rke_template, nodes,
        remove_nodes=True)
