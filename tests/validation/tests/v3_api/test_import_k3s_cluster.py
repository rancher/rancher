import os
from .common import *  # NOQA
from lib.aws import AmazonWebServices
from python_terraform import *


DATA_SUBDIR = os.path.join(os.path.dirname(os.path.realpath(__file__)),
                           'resource')
RANCHER_REGION = os.environ.get("AWS_REGION")
RANCHER_VPC_ID = os.environ.get("AWS_VPC")
RANCHER_SUBNETS = os.environ.get("AWS_SUBNET")
RANCHER_AWS_AMI = os.environ.get("AWS_AMI", "")
RANCHER_AWS_USER = os.environ.get("AWS_USER", "ubuntu")
AWS_SSH_KEY_NAME = os.environ.get("AWS_SSH_KEY_NAME", "")
HOST_NAME = os.environ.get('RANCHER_HOST_NAME', "sa")

RANCHER_RESOURCE_NAME = os.environ.get("RANCHER_RESOURCE_NAME", "")
RANCHER_K3S_VERSION = os.environ.get("RANCHER_K3S_VERSION", "")
RANCHER_K3S_NO_OF_SERVER_NODES = \
    os.environ.get("RANCHER_K3S_NO_OF_SERVER_NODES", 3)
RANCHER_K3S_NO_OF_WORKER_NODES = \
    os.environ.get("RANCHER_K3S_NO_OF_WORKER_NODES", 3)
RANCHER_K3S_SERVER_FLAGS = os.environ.get("RANCHER_K3S_SERVER_FLAGS", "server")
RANCHER_K3S_WORKER_FLAGS = os.environ.get("RANCHER_K3S_WORKER_FLAGS", "agent")
RANCHER_QA_SPACE = os.environ.get("RANCHER_QA_SPACE", "")

RANCHER_EXTERNAL_DB = os.environ.get("RANCHER_EXTERNAL_DB", "mysql")
RANCHER_EXTERNAL_DB_VERSION = os.environ.get("RANCHER_EXTERNAL_DB_VERSION", "5.7")
RANCHER_INSTANCE_CLASS = os.environ.get("RANCHER_INSTANCE_CLASS", "db.t2.micro")
RANCHER_DB_GROUP_NAME = os.environ.get("RANCHER_DB_GROUP_NAME", "default.mysql5.7")
RANCHER_DB_USERNAME = os.environ.get("RANCHER_DB_USERNAME", "")
RANCHER_DB_PASSWORD = os.environ.get("RANCHER_DB_PASSWORD", "")


def test_create_k3s_single_control_cluster():
    aws_nodes, client, k3s_clusterfilepath = create_single_control_cluster()


def test_create_k3s_multiple_control_cluster():
    k3s_clusterfilepath = create_multiple_control_cluster()


def test_import_k3s_single_control_cluster():
    aws_nodes, client, k3s_clusterfilepath = create_single_control_cluster()
    cluster = create_rancher_cluster(client, k3s_clusterfilepath)
    cluster_cleanup(client, cluster, aws_nodes)


def test_import_k3s_multiple_control_cluster():
    client = get_user_client()
    k3s_clusterfilepath = create_multiple_control_cluster()
    cluster = create_rancher_cluster(client, k3s_clusterfilepath)


def create_single_control_cluster():
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
    with open(k3s_clusterfilepath, 'r') as f:
        print(f.read())
    return aws_nodes, client, k3s_clusterfilepath


def create_multiple_control_cluster():
    k3s_kubeconfig_file = "k3s_multiple_kubeconfig.yaml"
    k3s_clusterfilepath = DATA_SUBDIR + "/" + k3s_kubeconfig_file

    tf_dir = DATA_SUBDIR + "/" + "terraform/master"
    keyPath = os.path.abspath('.') + '/.ssh/' + AWS_SSH_KEY_NAME
    os.chmod(keyPath, 0o400)
    no_of_servers = int(RANCHER_K3S_NO_OF_SERVER_NODES)
    no_of_servers = no_of_servers - 1

    tf = Terraform(working_dir=tf_dir,
                   variables={'region': RANCHER_REGION,
                              'vpc_id': RANCHER_VPC_ID,
                              'subnets': RANCHER_SUBNETS,
                              'aws_ami': RANCHER_AWS_AMI,
                              'aws_user': RANCHER_AWS_USER,
                              'resource_name': RANCHER_RESOURCE_NAME,
                              'access_key': keyPath,
                              'external_db': RANCHER_EXTERNAL_DB,
                              'external_db_version': RANCHER_EXTERNAL_DB_VERSION,
                              'db_group_name': RANCHER_DB_GROUP_NAME,
                              'instance_class': RANCHER_INSTANCE_CLASS,
                              'username': RANCHER_DB_USERNAME,
                              'password': RANCHER_DB_PASSWORD,
                              'k3s_version': RANCHER_K3S_VERSION,
                              'no_of_server_nodes': no_of_servers,
                              'server_flags': RANCHER_K3S_SERVER_FLAGS,
                              'qa_space': RANCHER_QA_SPACE})
    print("Creating cluster")
    tf.init()
    print(tf.plan(out="plan_server.out"))
    print("\n\n")
    print(tf.apply("--auto-approve"))
    print("\n\n")
    tf_dir = DATA_SUBDIR + "/" + "terraform/worker"
    tf = Terraform(working_dir=tf_dir,
                   variables={'region': RANCHER_REGION,
                              'vpc_id': RANCHER_VPC_ID,
                              'subnets': RANCHER_SUBNETS,
                              'aws_ami': RANCHER_AWS_AMI,
                              'aws_user': RANCHER_AWS_USER,
                              'resource_name': RANCHER_RESOURCE_NAME,
                              'access_key': keyPath,
                              'k3s_version': RANCHER_K3S_VERSION,
                              'no_of_worker_nodes': int(RANCHER_K3S_NO_OF_WORKER_NODES),
                              'worker_flags': RANCHER_K3S_WORKER_FLAGS})

    print("Joining worker nodes")
    tf.init()
    print(tf.plan(out="plan_worker.out"))
    print("\n\n")
    print(tf.apply("--auto-approve"))
    print("\n\n")
    cmd = "cp /tmp/multinode_kubeconfig1 " + k3s_clusterfilepath
    os.system(cmd)
    is_file = os.path.isfile(k3s_clusterfilepath)
    assert is_file
    print(k3s_clusterfilepath)
    with open(k3s_clusterfilepath, 'r') as f:
        print(f.read())
    print("K3s Cluster Created")
    return k3s_clusterfilepath


def create_rancher_cluster(client, k3s_clusterfilepath):
    clustername = random_test_name("testcustom-k3s")
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

    return cluster


def create_nodes():
    aws_nodes = \
        AmazonWebServices().create_multiple_nodes(
            int(RANCHER_K3S_NO_OF_WORKER_NODES),
            random_test_name("testcustom-k3s"+"-"+HOST_NAME))
    assert len(aws_nodes) == int(RANCHER_K3S_NO_OF_WORKER_NODES)
    for aws_node in aws_nodes:
        print("AWS NODE PUBLIC IP {}".format(aws_node.public_ip_address))
    return aws_nodes


def install_k3s_master_node(master):
    # Connect to the node and install k3s on master
    print("K3s VERSION {}".format(RANCHER_K3S_VERSION))
    cmd = "curl -sfL https://get.k3s.io | \
     {} sh -s - server --node-external-ip {}".\
        format("INSTALL_K3S_VERSION={}".format(RANCHER_K3S_VERSION) if RANCHER_K3S_VERSION else "", master.public_ip_address)
    print("Master Install {}".format(cmd))
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
    print("NO OF WORKER NODES: {}".format(RANCHER_K3S_NO_OF_WORKER_NODES))
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
