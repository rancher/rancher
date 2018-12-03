from .common import random_str


def test_gke_config_appears_correctly(admin_mc, remove_resource):
    cluster = admin_mc.client.create_cluster(
        name=random_str(), googleKubernetesEngineConfig={
            "credentials": "bad credentials",
            "nodeCount": 3
        })
    remove_resource(cluster)

    # test that a cluster returned from a POST has the correct config
    assert cluster.googleKubernetesEngineConfig.nodeCount == 3

    clusters = admin_mc.client.list_cluster(name=cluster.name)

    # test that a cluster returned from a list has the correct config
    assert len(clusters) == 1
    assert clusters.data[0].googleKubernetesEngineConfig.nodeCount == 3

    cluster = admin_mc.client.by_id_cluster(id=cluster.id)
    # test that a cluster returned from a GET has the correct config
    assert cluster.googleKubernetesEngineConfig.nodeCount == 3

    cluster.googleKubernetesEngineConfig.nodeCount = 4
    cluster = admin_mc.client.update_by_id_cluster(cluster.id, cluster)

    # test that a cluster returned from a PUT has the correct config
    assert cluster.googleKubernetesEngineConfig.nodeCount == 4
