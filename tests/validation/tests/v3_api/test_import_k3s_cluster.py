import os
from lib.aws import AmazonWebServices
from .common import *  # NOQA

DATA_SUBDIR = os.path.join(os.path.dirname(os.path.realpath(__file__)),
                           'resource')
RANCHER_K3S_VERSION = os.environ.get("RANCHER_K3S_VERSION", "")
RANCHER_K3S_NO_OF_WORKER_NODES = os.environ.get("AWS_NO_OF_WORKER_NODES", 3)

def test_import_k3s_cluster():

    # Get URL and User_Token
    client = get_user_client()

    # Create nodes in AWS
    aws_nodes = create_nodes()

    # Install k3s on master node
    kubeconfig, node_token = install_k3s_master_node(aws_nodes[0])

    # Join worker nodes
    join_k3s_worker_nodes(aws_nodes[0], aws_nodes[1:], node_token)

    # Verify cluster health
    verify_cluster_health(aws_nodes[0])

    # Update master node IP in kubeconfig file
    localhost = "127.0.0.1"
    kubeconfig = kubeconfig.replace(localhost, aws_nodes[0].public_ip_address)

    k3s_kubeconfig_file = "k3s_kubeconfig.yaml"
    k3s_clusterfilepath = create_kube_config_file(kubeconfig, k3s_kubeconfig_file)
    print(k3s_clusterfilepath)

    k3s_kubeconfig_file = "k3s_kubeconfig.yaml"
    k3s_clusterfilepath = DATA_SUBDIR + "/" + k3s_kubeconfig_file
    is_file = os.path.isfile(k3s_clusterfilepath)
    assert is_file

    clustername = random_test_name("testk3simport")
    cluster = client.create_cluster(name=clustername)
    cluster_token = create_custom_host_registration_token(client, cluster)
    command = cluster_token.insecureCommand
    finalimportcommand = command + " --kubeconfig " + k3s_clusterfilepath
    print(finalimportcommand)

    result = run_command(finalimportcommand)

    clusters = client.list_cluster(name=clustername).data
    assert len(clusters) > 0
    print("Cluster is")
    print(clusters[0])

    # Validate the cluster
    cluster = validate_cluster(client, clusters[0],
                               check_intermediate_state=False)
    cluster_cleanup(client, cluster, aws_nodes)


def create_nodes():
    aws_nodes = \
        AmazonWebServices().create_multiple_nodes(
            RANCHER_K3S_NO_OF_WORKER_NODES, random_test_name("testk3s"))
    assert len(aws_nodes) == RANCHER_K3S_NO_OF_WORKER_NODES
    for aws_node in aws_nodes:
        print("AWS NODE PUBLIC IP {}".format(aws_node.public_ip_address))
    return aws_nodes


def install_k3s_master_node(master):
    # Connect to the node and install k3s on master
    cmd = "curl -sfL https://get.k3s.io | \
     {} sh -s - server --node-external-ip {}".\
        format("INSTALL_K3S_VERSION={}".format(RANCHER_K3S_VERSION) if RANCHER_K3S_VERSION else "", master.public_ip_address)
    install_result = master.execute_command(cmd)
    print(install_result)

    # Get node token from master
    cmd = "sudo cat /var/lib/rancher/k3s/server/node-token"
    print(cmd)
    node_token = master.execute_command(cmd)
    print(node_token)

    # Get kube_config from master
    cmd = "sudo cat /etc/rancher/k3s/k3s.yaml"
    kubeconfig = master.execute_command(cmd)
    print(kubeconfig)

    print("NODE TOKEN: \n{}".format(node_token))
    print("KUBECONFIG: \n{}".format(kubeconfig))

    return kubeconfig[0].strip("\n"), node_token[0].strip("\n")


def join_k3s_worker_nodes(master, workers, node_token):
    for worker in workers:
        cmd = "curl -sfL https://get.k3s.io | \
             {} K3S_URL=https://{}:6443 K3S_TOKEN={} sh -s - ". \
            format("INSTALL_K3S_VERSION={}".format(RANCHER_K3S_VERSION) \
                       if RANCHER_K3S_VERSION else "", master.public_ip_address, node_token)
        cmd = cmd + " {} {}".format("--node-external-ip", worker.public_ip_address)
        print("Joining k3s master")
        print(cmd)
        install_result = worker.execute_command(cmd)
        print(install_result)


def verify_cluster_health(master):
    cmd = "sudo k3s kubectl get nodes"
    install_result = master.execute_command(cmd)
    print(install_result)


def create_kube_config_file(kubeconfig, k3s_kubeconfig_file):
    k3s_clusterfilepath = DATA_SUBDIR + "/" + k3s_kubeconfig_file
    f = open(k3s_clusterfilepath, "w")
    f.write(kubeconfig)
    f.close()
    return k3s_clusterfilepath
