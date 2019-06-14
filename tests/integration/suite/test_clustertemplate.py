from .common import random_str
from rancher import ApiError
import pytest
import time


def test_create_cluster_template_with_revision(admin_mc, remove_resource):
    cluster_template = create_cluster_template(admin_mc, remove_resource)
    templateId = cluster_template.id
    _ = \
        create_cluster_template_revision(admin_mc, templateId)
    _ = \
        create_cluster_template_revision(admin_mc, templateId)
    client = admin_mc.client
    template_reloaded = client.by_id_cluster_template(cluster_template.id)
    assert template_reloaded.links.revisions is not None


def test_create_cluster_with_template(admin_mc, remove_resource):
    cluster_template = create_cluster_template(admin_mc, remove_resource)
    templateId = cluster_template.id

    template_revision = \
        create_cluster_template_revision(admin_mc, templateId)

    # create a cluster with this template
    answers = {
                "values": {
                    "dockerRootDir": "/var/lib/docker123",
                    "rancherKubernetesEngineConfig.ignoreDockerVersion":
                    "false"
                }
              }

    revId = template_revision.id
    client = admin_mc.client
    cluster = client.create_cluster(name=random_str(),
                                    clusterTemplateId=cluster_template.id,
                                    clusterTemplateRevisionId=revId,
                                    description="template from cluster",
                                    answers=answers)
    remove_resource(cluster)
    assert cluster.conditions[0].type == 'Pending'
    assert cluster.conditions[0].status == 'True'

    # delete the cluster template, it should error out
    with pytest.raises(ApiError) as e:
        client.delete(cluster_template)
        assert e.value.error.status == 403

    client.delete(cluster)
    wait_for_cluster_to_be_deleted(client, cluster.id)


def test_create_cluster_validations(admin_mc, remove_resource):
    cluster_template = create_cluster_template(admin_mc, remove_resource)
    templateId = cluster_template.id
    template_revision = \
        create_cluster_template_revision(admin_mc, templateId)
    # create a cluster with this template
    revId = template_revision.id
    client = admin_mc.client
    rConfig = getRKEConfig()
    with pytest.raises(ApiError) as e:
        client.create_cluster(name=random_str(),
                              clusterTemplateId=cluster_template.id,
                              clusterTemplateRevisionId=revId,
                              description="template from cluster",
                              rancherKubernetesEngineConfig=rConfig)
        assert e.value.error.status == 500


def create_cluster_template(admin_mc, remove_resource):
    client = admin_mc.client
    template_name = random_str()

    cluster_template = \
        client.create_cluster_template(
                                         name=template_name,
                                         description="demo template")
    remove_resource(cluster_template)
    return cluster_template


def create_cluster_template_revision(admin_mc, clusterTemplateId):
    client = admin_mc.client
    rke_config = getRKEConfig()

    cluster_config = {
        "dockerRootDir": "/var/lib/docker",
        "enableClusterAlerting": "false",
        "enableClusterMonitoring": "false",
        "enableNetworkPolicy": "false",
        "type": "clusterSpecBase",
        "localClusterAuthEndpoint": {
            "enabled": "true",
            "type": "localClusterAuthEndpoint"
        },
        "rancherKubernetesEngineConfig": rke_config
    }

    questions = [{
                  "variable": "dockerRootDir",
                  "required": "false",
                  "type": "string",
                  "default": "/var/lib/docker"
                 },
                 {
                  "variable":
                  "rancherKubernetesEngineConfig.ignoreDockerVersion",
                  "required": "false",
                  "type": "boolean",
                  "default": "true"
                 }]

    cluster_template_revision = \
        client.create_cluster_template_revision(
                                        clusterConfig=cluster_config,
                                        clusterTemplateId=clusterTemplateId,
                                        disabled="false",
                                        kubernetesVersion="v1.13.5-rancher1-3",
                                        questions=questions
                                        )

    return cluster_template_revision


def getRKEConfig():
    rke_config = {
        "addonJobTimeout": 30,
        "ignoreDockerVersion": "true",
        "sshAgentAuth": "false",
        "type": "rancherKubernetesEngineConfig",
        "kubernetesVersion": "v1.13.5-rancher1-3",
        "authentication": {
            "strategy": "x509",
            "type": "authnConfig"
        },
        "network": {
            "plugin": "canal",
            "type": "networkConfig",
            "options": {
                "flannel_backend_type": "vxlan"
            }
        },
        "ingress": {
            "provider": "nginx",
            "type": "ingressConfig"
        },
        "monitoring": {
            "provider": "metrics-server",
            "type": "monitoringConfig"
        },
        "services": {
            "type": "rkeConfigServices",
            "kubeApi": {
                "alwaysPullImages": "false",
                "podSecurityPolicy": "false",
                "serviceNodePortRange": "30000-32767",
                "type": "kubeAPIService"
            },
            "etcd": {
                "creation": "12h",
                "extraArgs": {
                    "heartbeat-interval": 500,
                    "election-timeout": 5000
                },
                "retention": "72h",
                "snapshot": "false",
                "type": "etcdService",
                "backupConfig": {
                    "enabled": "true",
                    "intervalHours": 12,
                    "retention": 6,
                    "type": "backupConfig"
                }
            }
        }
    }
    return rke_config


def wait_for_cluster_to_be_deleted(client, clusterId, timeout=45):
    deleted = False
    start = time.time()
    interval = 0.5
    while not deleted:
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for clusters")
        cluster = client.by_id_cluster(clusterId)
        if cluster is None:
            deleted = True
        time.sleep(interval)
        interval *= 2
