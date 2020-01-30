from copy import deepcopy
import os
import requests
from .common import CATTLE_API_URL
from .common import create_config_file
from .common import CATTLE_TEST_URL
from .common import ADMIN_TOKEN
from .common import get_user_client
from .common import USER_TOKEN
from .common import validate_cluster
from .test_rke_cluster_provisioning import HOST_NAME
from .test_rke_cluster_provisioning import random_name
from .test_rke_cluster_provisioning import rke_config
from .test_rke_cluster_provisioning import random_test_name
from .test_rke_cluster_provisioning import get_custom_host_registration_cmd

from lib.aws import AmazonWebServices
import multiprocessing


K8S_VERSION_URL = "/settings/k8s-versions-current"
PLUGIN = ["calico", "canal", "flannel", "weave"]
CLUSTER_LIST = []
NODE_COUNT_KDM_CLUSTER = \
    int(os.environ.get("RANCHER_NODE_COUNT_KDM_CLUSTER", 4))


def test_clusters_for_kdm():
    """
    This fuction is used to check the KDM changes.
    It deploys all the different types of k8s clusters - default_k8s_versions,
    across all the network provider types - PLUGIN
    It then deploys a workload on each cluster,
    checks service discovery - DNS resolution and
    checks the ingress when enabled
    Helper function - validate_custom_cluster_kdm() to create the AWS nodes
    and add to the cluster
    """
    k8s_version_url = CATTLE_API_URL + K8S_VERSION_URL
    plugin = PLUGIN
    head = {'Authorization': 'Bearer ' + USER_TOKEN}
    response = requests.get(k8s_version_url, verify=False, headers=head)
    data = response.json()
    print(data)
    k8s_v = data["value"]
    default_k8s_versions = k8s_v.split(",")
    # default_k8s_versions = ["v1.17.2-rancher1-1"]
    list_process = []
    print("response: ", response)
    print("k8s_v: ", k8s_v)
    print("default_k8s_versions: ", default_k8s_versions)
    for k8s_version in default_k8s_versions:
        rke_config_new = deepcopy(rke_config)
        rke_config_new["kubernetesVersion"] = k8s_version
        node_count = NODE_COUNT_KDM_CLUSTER * len(plugin)
        aws_nodes = \
            AmazonWebServices().create_multiple_nodes(
                node_count, random_test_name(HOST_NAME))
        i = 0
        for plug in plugin:
            if plug == "calico" or plug == "canal" or plug == "weave":
                rke_config_new["network"]["options"] = \
                    {"flannel_backend_type": "vxlan"}
            rke_config_new["network"] = {"type": "networkConfig",
                                         "plugin": plug}
            client = get_user_client()
            cluster_name = random_name()
            cluster = client.create_cluster(name=cluster_name,
                                            driver="rancherKubernetesEngine",
                                            rancherKubernetesEngineConfig=
                                            rke_config_new)
            p1 = multiprocessing.Process(target=validate_custom_cluster_kdm,
                                         args=(cluster,
                                               aws_nodes
                                               [i:i+NODE_COUNT_KDM_CLUSTER]))
            CLUSTER_LIST.append(cluster)
            list_process.append(p1)
            p1.start()
            i = i + NODE_COUNT_KDM_CLUSTER
    failed_cluster = {}
    passed_cluster = {}
    for process in list_process:
        process.join()
    env_details = "env.CATTLE_TEST_URL='" + CATTLE_TEST_URL + "'\n"
    env_details += "env.ADMIN_TOKEN='" + ADMIN_TOKEN + "'\n"
    env_details += "env.USER_TOKEN='" + USER_TOKEN + "'\n"
    i = 0
    for cluster in CLUSTER_LIST:
        env_details += \
            "env.CLUSTER_NAME_" + str(i) + "='" + cluster.name + "'\n"
        i = i + 1
    create_config_file(env_details)
    print("env_details:", env_details)
    for cluster in CLUSTER_LIST:
        try:
            validate_cluster(client, cluster, cluster.state, False, False)
            # print the cluster details that have passed
            passed_cluster[cluster.name] = {}
            passed_cluster[cluster.name]["k8s"] = \
                cluster["rancherKubernetesEngineConfig"]["kubernetesVersion"]
            passed_cluster[cluster.name]["network"] = \
                cluster["rancherKubernetesEngineConfig"]["network"]["plugin"]
        except:
            print("Issue in ", cluster.name)
            failed_cluster[cluster.name] = {}
            failed_cluster[cluster.name]["k8s"] = \
                cluster["rancherKubernetesEngineConfig"]["kubernetesVersion"]
            failed_cluster[cluster.name]["network"] = \
                cluster["rancherKubernetesEngineConfig"]["network"]["plugin"]
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
