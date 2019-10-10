import os
from lib.aws import AmazonWebServices
from .common import *  # NOQA
RANCHER_CLEANUP_CLUSTER = os.environ.get('RANCHER_CLEANUP_CLUSTER', "True")

DATA_SUBDIR = os.path.join(os.path.dirname(os.path.realpath(__file__)),
                           'resource')
AWS_SSH_KEY_NAME = os.environ.get("AWS_SSH_KEY_NAME")


def test_import_rke_cluster():

    client = get_user_client()

    # Create nodes in AWS
    aws_nodes = create_nodes()
    clusterfilepath = create_rke_cluster_config(aws_nodes)

    is_file = os.path.isfile(clusterfilepath)
    assert is_file

    # Create RKE K8s Cluster
    clustername = random_test_name("testimport")
    rkecommand = 'rke ' + "up" \
                 + ' --config ' + clusterfilepath
    print(rkecommand)
    result = run_command_with_stderr(rkecommand)

    cluster = client.create_cluster(name=clustername)
    print(cluster)
    cluster_token = create_custom_host_registration_token(client, cluster)
    command = cluster_token.insecureCommand
    print(command)
    rke_config_file = "kube_config_clusternew.yml"
    finalimportcommand = command + " --kubeconfig " + DATA_SUBDIR + "/" + \
        rke_config_file
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


def create_nodes():

    aws_nodes = \
        AmazonWebServices().create_multiple_nodes(
            3, random_test_name("testcustom"), wait_for_ready=True)
    assert len(aws_nodes) == 3
    for aws_node in aws_nodes:
        print(aws_node)
        print(aws_node.public_ip_address)

    return aws_nodes


def create_rke_cluster_config(aws_nodes):

    configfile = "cluster.yml"

    rkeconfig = readDataFile(DATA_SUBDIR, configfile)
    rkeconfig = rkeconfig.replace("$ip1", aws_nodes[0].public_ip_address)
    rkeconfig = rkeconfig.replace("$ip2", aws_nodes[1].public_ip_address)
    rkeconfig = rkeconfig.replace("$ip3", aws_nodes[2].public_ip_address)
    rkeconfig = rkeconfig.replace("$AWS_SSH_KEY_NAME", AWS_SSH_KEY_NAME)

    print(rkeconfig)
    clusterfilepath = DATA_SUBDIR + "/" + "clusternew.yml"
    print(clusterfilepath)

    f = open(clusterfilepath, "w")
    f.write(rkeconfig)
    f.close()
    return clusterfilepath


def readDataFile(data_dir, name):

    fname = os.path.join(data_dir, name)
    print("File Name is: ")
    print(fname)
    is_file = os.path.isfile(fname)
    assert is_file
    with open(fname) as f:
        return f.read()
