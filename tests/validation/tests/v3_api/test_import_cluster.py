import os
from lib.aws import AmazonWebServices
from .common import get_user_client
from .common import run_command
from .common import random_test_name
from .common import run_command_with_stderr
from .common import create_custom_host_registration_token
from .common import validate_cluster
from .common import cluster_cleanup
from .common import readDataFile


RANCHER_CLEANUP_CLUSTER = os.environ.get('RANCHER_CLEANUP_CLUSTER', "True")

DATA_SUBDIR = os.path.join(os.path.dirname(os.path.realpath(__file__)),
                           'resource')
AWS_SSH_KEY_NAME = os.environ.get("AWS_SSH_KEY_NAME")
AWS_NODE_COUNT = int(os.environ.get("AWS_NODE_COUNT", 3))
HOST_NAME = os.environ.get('RANCHER_HOST_NAME', "testcustom")


def test_import_rke_cluster():

    client = get_user_client()

    # Create AWS nodes for the cluster
    aws_nodes = \
        AmazonWebServices().create_multiple_nodes(
            AWS_NODE_COUNT, random_test_name(HOST_NAME),
            wait_for_ready=True)
    assert len(aws_nodes) == AWS_NODE_COUNT
    # Create RKE config
    clusterfilepath = create_rke_cluster_config(aws_nodes)
    is_file = os.path.isfile(clusterfilepath)
    assert is_file

    # Print config file to be used for rke cluster create
    configfile = run_command("cat " + clusterfilepath)
    print("RKE Config file generated:\n")
    print(configfile)

    # Create RKE K8s Cluster
    clustername = random_test_name("testimport")
    rkecommand = "rke up --config {}".format(clusterfilepath)
    print(rkecommand)
    result = run_command_with_stderr(rkecommand)

    # Import the RKE cluster
    cluster = client.create_cluster(name=clustername)
    print(cluster)
    cluster_token = create_custom_host_registration_token(client, cluster)
    command = cluster_token.insecureCommand
    print(command)
    rke_config_file = "kube_config_clusternew.yml"
    finalimportcommand = "{} --kubeconfig {}/{}".format(command, DATA_SUBDIR,
                                                        rke_config_file)
    print("Final command to import cluster is:")
    print(finalimportcommand)
    result = run_command(finalimportcommand)
    print(result)
    clusters = client.list_cluster(name=clustername).data
    assert len(clusters) > 0
    print("Cluster is")
    print(clusters[0])

    # Validate the cluster
    cluster = validate_cluster(client, clusters[0],
                               check_intermediate_state=False)

    cluster_cleanup(client, cluster, aws_nodes)


def test_generate_rke_config():

    aws_nodes = \
        AmazonWebServices().create_multiple_nodes(
            AWS_NODE_COUNT, random_test_name(HOST_NAME),
            wait_for_ready=True)
    assert len(aws_nodes) == AWS_NODE_COUNT
    # Create RKE config
    rkeconfigpath = create_rke_cluster_config(aws_nodes)
    rkeconfig = run_command("cat " + rkeconfigpath)
    print("RKE Config file generated\n")
    print(rkeconfig)


def create_rke_cluster_config(aws_nodes):

    """
    Generates RKE config file with a minimum of 3 nodes with ALL roles(etcd,
    worker and control plane). If the requested number of nodes is greater
    than 3, additional nodes with worker role are created
    """
    # Create RKE Config file
    configfile = "cluster.yml"
    rkeconfig = readDataFile(DATA_SUBDIR, configfile)
    print(rkeconfig)
    for i in range(0, AWS_NODE_COUNT):
        ipstring = "$ip" + str(i)
        intipstring = "$intip" + str(i)
        rkeconfig = rkeconfig.replace(ipstring, aws_nodes[i].public_ip_address)
        rkeconfig = rkeconfig.replace(intipstring,
                                      aws_nodes[i].private_ip_address)
    rkeconfig = rkeconfig.replace("$AWS_SSH_KEY_NAME", AWS_SSH_KEY_NAME)

    clusterfilepath = DATA_SUBDIR + "/" + "clusternew.yml"
    print(clusterfilepath)
    f = open(clusterfilepath, "w")
    f.write(rkeconfig)
    if(AWS_NODE_COUNT > 3):
        for i in range(3, AWS_NODE_COUNT):
            for j in range(i, i+1):
                f.write("  - address: {}\n".format(
                    aws_nodes[j].public_ip_address))
                f.write("    internaladdress: {}\n".format(
                    aws_nodes[j].private_ip_address))
                f.write("    user: ubuntu\n")
                f.write("    role: [worker]\n")

    f.close()
    return clusterfilepath
