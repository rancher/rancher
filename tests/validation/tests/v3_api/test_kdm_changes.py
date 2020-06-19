from copy import deepcopy
import os
import ast
import requests
from .common import json
from .common import CATTLE_API_URL
from .common import create_config_file
from .common import CATTLE_TEST_URL
from .common import ADMIN_TOKEN
from .common import get_setting_value_by_name
from .common import get_user_client
from .common import USER_TOKEN
from .common import validate_cluster
from .test_rke_cluster_provisioning import HOST_NAME
from .test_rke_cluster_provisioning import random_name
from .test_rke_cluster_provisioning import rke_config
from .test_rke_cluster_provisioning import K8S_VERSION
from .test_rke_cluster_provisioning import random_test_name
from .test_rke_cluster_provisioning import get_custom_host_registration_cmd

from lib.aws import AmazonWebServices
import multiprocessing


K8S_VERSION_URL = "/settings/k8s-versions-current"
NETWORK_PLUGINS = ["calico", "canal", "flannel", "weave"]
DNS_PROVIDERS = ["coredns", "kube-dns"]
CLUSTER_LIST = []
NODE_COUNT_KDM_CLUSTER = \
    int(os.environ.get("RANCHER_NODE_COUNT_KDM_CLUSTER", 4))
DNS_MATRIX = \
    ast.literal_eval(os.environ.get('RANCHER_DNS_PROVIDER_MATRIX', "False"))


def test_clusters_for_kdm():
    """
    This fuction is used to check the KDM changes.
    It deploys all the different types of k8s clusters - default_k8s_versions,
    across all the network provider types -
    NETWORK_PLUGINS, and all dns provider types - DNS_PROVIDERS
    It then deploys a workload on each cluster,
    checks service discovery - DNS resolution and
    checks the ingress when enabled
    Helper function - validate_custom_cluster_kdm() to create the AWS nodes
    and add to the cluster
    """
    rancher_version = get_setting_value_by_name('server-version')
    if K8S_VERSION == "":
        if str(rancher_version).startswith('v2.2'):
            k8s_v = get_setting_value_by_name('k8s-version-to-images')
            default_k8s_versions = json.loads(k8s_v).keys()
        else:
            k8s_v = get_setting_value_by_name('k8s-versions-current')
            default_k8s_versions = k8s_v.split(",")

    else:
        default_k8s_versions = [K8S_VERSION]
    list_process = []
    network_plugins = NETWORK_PLUGINS
    dns_providers = DNS_PROVIDERS
    print("default_k8s_versions: ", default_k8s_versions)
    for k8s_version in default_k8s_versions:
        rke_config_new = deepcopy(rke_config)
        rke_config_new["kubernetesVersion"] = k8s_version
        node_count = NODE_COUNT_KDM_CLUSTER * len(network_plugins)
        if DNS_MATRIX:
            node_count = node_count * len(dns_providers) * 2
        aws_nodes = \
            AmazonWebServices().create_multiple_nodes(
                node_count, random_test_name(HOST_NAME))
        i = 0
        for network_plugin in network_plugins:
            if network_plugin == "calico" or \
                    network_plugin == "canal" or network_plugin == "weave":
                rke_config_new["network"]["options"] = \
                    {"flannel_backend_type": "vxlan"}
            rke_config_new["network"] = {"type": "networkConfig",
                                         "plugin": network_plugin}
            if DNS_MATRIX:
                for dns_provider in dns_providers:
                    for nodelocaldns in True, False:
                        dns_entry, cluster_name = get_dns_rke_config(
                            dns_provider,
                            network_plugin,
                            nodelocaldns
                        )
                        rke_config_new.update(dns_entry)
                        list_process = create_kdm_clusters(
                            cluster_name,
                            rke_config_new,
                            aws_nodes,
                            i,
                            list_process
                        )
                        i = i + NODE_COUNT_KDM_CLUSTER
            else:
                cluster_name = random_test_name(network_plugin)
                list_process = create_kdm_clusters(
                    cluster_name,
                    rke_config_new,
                    aws_nodes,
                    i,
                    list_process
                )
                i = i + NODE_COUNT_KDM_CLUSTER
    failed_cluster = {}
    passed_cluster = {}
    for process in list_process:
        process.join()
    # setting environment variables
    env_details = "env.CATTLE_TEST_URL='" + CATTLE_TEST_URL + "'\n"
    env_details += "env.ADMIN_TOKEN='" + ADMIN_TOKEN + "'\n"
    env_details += "env.USER_TOKEN='" + USER_TOKEN + "'\n"
    names = ""
    i = 0
    for cluster in CLUSTER_LIST:
        env_details += \
            "env.CLUSTER_NAME_" + str(i) + "='" + cluster.name + "'\n"
        names += cluster.name + ","
        i = i + 1
    create_config_file(env_details)
    print("env_details:", env_details)
    print("list of cluster names: " + names[:-1])
    client = get_user_client()
    for cluster in CLUSTER_LIST:
        try:
            validate_cluster(client, cluster, cluster.state, False, False)
            # details of cluster that have passed
            passed_cluster = save_cluster_details(passed_cluster, cluster)

        except Exception as e:
            print("Issue in {}:\n{}".format(cluster.name, e))
            # details of cluster that have failed
            failed_cluster = save_cluster_details(failed_cluster, cluster)

    # printing results
    print("--------------Passed Cluster information--------------'\n")
    for key, value in passed_cluster.items():
        print(key + "-->" + str(value) + "\n")
    print("--------------Failed Cluster information--------------'\n")
    for key, value in failed_cluster.items():
        print(key + "-->" + str(value) + "\n")
    assert len(failed_cluster) == 0, "Clusters have failed to provision. " \
                                     "Check logs for more info"


def validate_custom_cluster_kdm(cluster, aws_nodes):
    if NODE_COUNT_KDM_CLUSTER == 4:
        node_roles = [["controlplane"], ["etcd"], ["worker"], ["worker"]]
    elif NODE_COUNT_KDM_CLUSTER == 2:
        node_roles = [["controlplane", "etcd", "worker"], ["worker"]]
    else:
        node_roles = [["worker", "controlplane", "etcd"]]
    client = get_user_client()
    assert cluster.state == "provisioning"
    i = 0
    for aws_node in aws_nodes:
        docker_run_cmd = \
            get_custom_host_registration_cmd(client,
                                             cluster,
                                             node_roles[i],
                                             aws_node)
        for nr in node_roles[i]:
            aws_node.roles.append(nr)
        aws_node.execute_command(docker_run_cmd)
        i += 1


def get_dns_rke_config(dns_provider, network_plugin, nodelocaldns):
    dns_entry = dict()
    dns_entry["dns"] = {"type": "dnsConfig",
                        "provider": dns_provider}
    cluster_options = network_plugin + "-" + dns_provider
    if nodelocaldns:
        dns_entry["dns"]["nodelocal"] = \
            {"type": "nodelocal", "ipAddress": "169.254.20.10"}
        cluster_options += "-nodelocaldns"
    cluster_name = random_test_name(cluster_options)
    return dns_entry, cluster_name


def create_kdm_clusters(cluster_name, rke_config_new,
                        aws_nodes, aws_nodes_index, list_process):
    client = get_user_client()
    cluster = client.create_cluster(name=cluster_name,
                                    driver="rancherKubernetesEngine",
                                    rancherKubernetesEngineConfig=
                                    rke_config_new)
    p1 = multiprocessing.Process(
        target=validate_custom_cluster_kdm,
        args=(
            cluster,
            aws_nodes[aws_nodes_index:aws_nodes_index + NODE_COUNT_KDM_CLUSTER]
        )
    )
    CLUSTER_LIST.append(cluster)
    list_process.append(p1)
    p1.start()
    return list_process


def save_cluster_details(cluster_detail, cluster):
    cluster_detail[cluster.name] = {}
    cluster_detail[cluster.name]["k8s"] = \
        cluster["rancherKubernetesEngineConfig"]["kubernetesVersion"]
    cluster_detail[cluster.name]["network"] = \
        cluster["rancherKubernetesEngineConfig"]["network"]["plugin"]
    if DNS_MATRIX:
        cluster_detail[cluster.name]["dns"] = \
            cluster["rancherKubernetesEngineConfig"]["dns"]["provider"]
        if "-nodelocaldns" in cluster.name:
            cluster_detail[cluster.name]["nodelocaldns"] = \
                "enabled"
        else:
            cluster_detail[cluster.name]["nodelocaldns"] = \
                "disabled"
    return cluster_detail
