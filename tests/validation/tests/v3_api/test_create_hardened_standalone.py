import pytest
from .common import *  # NOQA
from lib.aws import AmazonWebServices
from .test_create_ha import create_rke_cluster
HOST_NAME = os.environ.get('RANCHER_HOST_NAME', "test-hardened")
DATA_SUBDIR = os.path.join(os.path.dirname(os.path.realpath(__file__)),
                           'resource')
kubeconfig_path = DATA_SUBDIR + "/kube_config_hardened-cluster-filled.yml"                           
RKE_K8S_VERSION = os.environ.get("RANCHER_RKE_K8S_VERSION","")

def test_create_hardened_standalone():
    node_role = [["worker", "controlplane", "etcd"]]
    node_roles =[]
    for role in node_role:
        node_roles.extend([role, role, role])
    aws_nodes = \
        AmazonWebServices().create_multiple_nodes(
            3, random_test_name(HOST_NAME))
    profile = "rke-cis-1.5"
    aws_nodes = prepare_hardened_nodes(aws_nodes, profile, node_roles)
    
    configfile = "hardened-cluster.yml"
    clusterfilepath = create_rkeconfig_file(configfile, aws_nodes)
    print("clusterfilepath: ", clusterfilepath)
    time.sleep(30)
    rkecommand = "rke up --config {}".format(clusterfilepath)
    print(rkecommand)
    result = run_command_with_stderr(rkecommand)
    print("RKE up result: ", result)

    print_kubeconfig()
    cluster = prepare_hardened_cluster(cluster, profile)


def print_kubeconfig():
    kubeconfig_file = open(kubeconfig_path, "r")
    kubeconfig_contents = kubeconfig_file.read()
    kubeconfig_file.close()
    kubeconfig_contents_encoded = base64.b64encode(
        kubeconfig_contents.encode("utf-8")).decode("utf-8")
    print("\n\n" + kubeconfig_contents + "\n\n")
    print("\nBase64 encoded: \n\n" + kubeconfig_contents_encoded + "\n\n")


def create_rkeconfig_file(configfile, aws_nodes):
    rkeconfig = readDataFile(DATA_SUBDIR, configfile)
    for i in range(0, len(aws_nodes)):
        ipstring = "$ip" + str(i)
        intipstring = "$internalIp" + str(i)
        rkeconfig = rkeconfig.replace(ipstring, aws_nodes[i].public_ip_address)
        rkeconfig = rkeconfig.replace(intipstring,
                                      aws_nodes[i].private_ip_address)

    rkeconfig = rkeconfig.replace("$user0", aws_nodes[0].ssh_user)
    rkeconfig = rkeconfig.replace("$user1", aws_nodes[1].ssh_user)
    rkeconfig = rkeconfig.replace("$user2", aws_nodes[2].ssh_user)
    rkeconfig = rkeconfig.replace("$AWS_SSH_KEY_NAME", AWS_SSH_KEY_NAME)
    rkeconfig = rkeconfig.replace("$KUBERNETES_VERSION", RKE_K8S_VERSION)
    
    print("hardened-cluster-filled.yml: \n" + rkeconfig + "\n")

    clusterfilepath = DATA_SUBDIR + "/" + "hardened-cluster-filled.yml"

    f = open(clusterfilepath, "w")
    f.write(rkeconfig)
    f.close()
    return clusterfilepath