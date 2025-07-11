from .common import random_str


def test_rke_config_appears_correctly(admin_mc, remove_resource):
    """ Testing a single field from the RKE config to ensure that the
    schema is properly populated"""
    cluster = admin_mc.client.create_cluster(
        name=random_str(), rancherKubernetesEngineConfig={
            "kubernetesVersion": "v1.12.9-rancher1-1",
        })
    remove_resource(cluster)

    k8s_version = cluster.rancherKubernetesEngineConfig.kubernetesVersion
    assert k8s_version == "v1.12.9-rancher1-1"


def test_rke_config_no_change_k8sversion_addon(admin_mc, remove_resource):
    """ Testing if kubernetesVersion stays the same after updating
    something else in the cluster, e.g. addonJobTimeout"""
    k8s_version = "v1.12.9-rancher1-1"
    cluster = admin_mc.client.create_cluster(
        name=random_str(), rancherKubernetesEngineConfig={
            "kubernetesVersion": k8s_version,
        })
    remove_resource(cluster)
    cluster = admin_mc.client.update_by_id_cluster(
       id=cluster.id,
       name=cluster.name,
       rancherKubernetesEngineConfig={
            "addonJobTimeout": 45,
        })
    k8s_version_post = cluster.rancherKubernetesEngineConfig.kubernetesVersion
    assert k8s_version_post == k8s_version


def test_rke_config_no_change_k8sversion_np(admin_mc, remove_resource):
    """ Testing if kubernetesVersion stays the same after updating
    something else in the cluster, e.g. addonJobTimeout"""
    cluster_config_np_false = {
        "enableNetworkPolicy": "false",
        "rancherKubernetesEngineConfig": {
            "addonJobTimeout": 45,
            "kubernetesVersion": "v1.12.9-rancher1-1",
            "network": {
                "plugin": "canal",
            }
        }
    }

    cluster = admin_mc.client.create_cluster(
            name=random_str(),
            cluster=cluster_config_np_false,
    )
    remove_resource(cluster)

    cluster_config_np_true = {
        "name": cluster.name,
        "enableNetworkPolicy": "true",
        "rancherKubernetesEngineConfig": {
            "network": {
                "plugin": "canal",
            }
        }
    }

    cluster = admin_mc.client.update_by_id_cluster(
        cluster.id,
        cluster_config_np_true,
    )

    cluster_config_addonjob = {
        "name": cluster.name,
        "rancherKubernetesEngineConfig": {
            "addonJobTimeout": 55,
            "network": {
                "plugin": "canal",
            }
        }
    }

    cluster = admin_mc.client.update_by_id_cluster(
        cluster.id,
        cluster_config_addonjob,
    )
    assert cluster.enableNetworkPolicy is True
