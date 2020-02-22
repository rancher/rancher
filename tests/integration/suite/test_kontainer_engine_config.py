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


def test_eks_config_appears_correctly(admin_mc, remove_resource):
    """ Simple test to ensure that cluster returned from POST is correct"""
    cluster = admin_mc.client.create_cluster(
        name=random_str(), amazonElasticContainerServiceConfig={
            "accessKey": "MyAccessKey",
            "ami": "",
            "associateWorkerNodePublicIp": True,
            "displayName": "EKS-api-cluster",
            "driverName": "amazonelasticcontainerservice",
            "instanceType": "t3.small",
            "kubernetesVersion": "1.14",
            "maximumNodes": 3,
            "minimumNodes": 1,
            "region": "us-east-2",
            "secretKey": "secret-key",
            "serviceRole": "",
            "sessionToken": "",
            "userData": "!#/bin/bash\ntouch /tmp/testfile.txt",
            "virtualNetwork": "",
        })
    remove_resource(cluster)

    # test cluster returned from POST has correct config
    assert cluster.amazonElasticContainerServiceConfig.maximumNodes == 3

    assert (cluster.amazonElasticContainerServiceConfig.userData ==
            "!#/bin/bash\ntouch /tmp/testfile.txt")

    clusters = admin_mc.client.list_cluster(name=cluster.name)

    # test that a cluster returned from a list has the correct config
    assert len(clusters) == 1
    assert (clusters.data[0].amazonElasticContainerServiceConfig.maximumNodes
            == 3)

    cluster = admin_mc.client.by_id_cluster(cluster.id)
    # test that a cluster returned from a GET has the correct config
    assert cluster.amazonElasticContainerServiceConfig.maximumNodes == 3

    cluster.amazonElasticContainerServiceConfig.maximumNodes = 5
    cluster = admin_mc.client.update_by_id_cluster(cluster.id, cluster)

    # test that cluster returned from PUT has correct config
    assert cluster.amazonElasticContainerServiceConfig.maximumNodes == 5


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
