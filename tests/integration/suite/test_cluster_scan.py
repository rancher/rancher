from .common import random_str
from .conftest import cluster_and_client

def test_run_scan_not_available_on_not_ready_cluster(admin_mc):
    client = admin_mc.client
    cluster = client.create_cluster(
        name=random_str(),
        rancherKubernetesEngineConfig={
            "accessKey": "junk"
        }
    )
    _, cluster_client = cluster_and_client(cluster.id, client)
    try:
        admin_mc.client.action(obj=cluster, action_name="runSecurityScan", )
    except AttributeError as e:
        assert e is not None
    client.delete(cluster)

