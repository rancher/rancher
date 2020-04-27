import os
from .common import (get_user_client, validate_cluster)
from .test_rke_cluster_provisioning import create_and_validate_cluster
from .test_rke_cluster_provisioning import random_node_name
from .test_rke_cluster_provisioning import rke_config_zerodowntime
from .test_rke_cluster_provisioning import DO_ACCESSKEY
from .test_rke_cluster_provisioning import random_name
from .test_rke_cluster_provisioning import K8S_VERSION_UPGRADE

CONTROL_NODES = os.environ.get('RANCHER_CONTROL_NODES', "1")
ETCD_NODES = os.environ.get('RANCHER_ETCD_NODES', "1")
WORKER_NODES = os.environ.get('RANCHER_WORKER_NODES', "1")
LOOP_COUNT = os.environ.get('RANCHER_LOOP_COUNT', "1")


def test_zero_down_time_upgrade():
    node_template = node_template_do()
    client = get_user_client()
    for i in range(0, int(LOOP_COUNT)):
        nodes = []
        node_name = random_node_name()
        node = {"hostnamePrefix": node_name,
                "nodeTemplateId": node_template.id,
                "requestedHostname": node_name,
                "controlPlane": True,
                "quantity": CONTROL_NODES}
        nodes.append(node)
        node_name = random_node_name()
        node = {"hostnamePrefix": node_name,
                "nodeTemplateId": node_template.id,
                "requestedHostname": node_name,
                "etcd": True,
                "quantity": ETCD_NODES}
        nodes.append(node)
        node_name = random_node_name()
        node = {"hostnamePrefix": node_name,
                "nodeTemplateId": node_template.id,
                "requestedHostname": node_name,
                "worker": True,
                "quantity": WORKER_NODES}
        nodes.append(node)
        cluster, node_pools = \
            create_and_validate_cluster(client,
                                        nodes,
                                        rke_config_zerodowntime,
                                        None)
        rke_config = cluster.rancherKubernetesEngineConfig
        rke_updated_config = rke_config.copy()
        rke_updated_config["kubernetesVersion"] = K8S_VERSION_UPGRADE
        cluster = client.update(cluster,
                                name=cluster.name,
                                rancherKubernetesEngineConfig=
                                rke_updated_config)
        cluster = validate_cluster(client, cluster,
                                   intermediate_state="upgrading",
                                   k8s_version=K8S_VERSION_UPGRADE)

        client.delete(cluster)


def node_template_do():
    client = get_user_client()
    do_cloud_credential_config = {"accessToken": DO_ACCESSKEY}
    do_cloud_credential = client.create_cloud_credential(
        digitaloceancredentialConfig=do_cloud_credential_config
    )
    node_template = client.create_node_template(
        digitaloceanConfig={"region": "nyc3",
                            "size": "2gb",
                            "image": "ubuntu-16-04-x64"},
        name=random_name(),
        driver="digitalocean",
        cloudCredentialId=do_cloud_credential.id,
        useInternalIpAddress=True)
    node_template = client.wait_success(node_template)
    return node_template
