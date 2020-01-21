from .common import random_str
from .conftest import cluster_and_client


def test_run_scan_not_available_on_not_ready_cluster(admin_mc,
                                                     remove_resource):
    client = admin_mc.client
    cluster = client.create_cluster(
        name=random_str(),
        rancherKubernetesEngineConfig={
            "accessKey": "junk"
        }
    )
    remove_resource(cluster)
    _, cluster_client = cluster_and_client(cluster.id, client)
    assert 'runSecurityScan' not in cluster.actions
