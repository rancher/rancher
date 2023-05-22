from python_terraform import * # NOQA
from .common import *  # NOQA

RANCHER_REGION = os.environ.get("AWS_REGION")
RANCHER_VPC_ID = os.environ.get("AWS_VPC")
RANCHER_SUBNETS = os.environ.get("AWS_SUBNET")
RANCHER_AWS_SG = os.environ.get("AWS_SECURITY_GROUPS")
RANCHER_AVAILABILITY_ZONE = os.environ.get("AWS_AVAILABILITY_ZONE")
RANCHER_AWS_AMI = os.environ.get("AWS_AMI", "")
RANCHER_AWS_USER = os.environ.get("AWS_USER", "ubuntu")
HOST_NAME = os.environ.get('RANCHER_HOST_NAME', "sa")

K3S_CHANNEL = os.environ.get("K3S_CHANNEL", "null")
RANCHER_K3S_VERSION = os.environ.get("RANCHER_K3S_VERSION", "")
RANCHER_K3S_VERSIONS = os.environ.get('RANCHER_K3S_VERSIONS', "").split(",")
RANCHER_K3S_NO_OF_SERVER_NODES = \
    os.environ.get("RANCHER_K3S_NO_OF_SERVER_NODES", 2)
RANCHER_K3S_NO_OF_WORKER_NODES = \
    os.environ.get("RANCHER_K3S_NO_OF_WORKER_NODES", 0)
RANCHER_K3S_SERVER_FLAGS = os.environ.get("RANCHER_K3S_SERVER_FLAGS", "")
RANCHER_K3S_WORKER_FLAGS = os.environ.get("RANCHER_K3S_WORKER_FLAGS", "agent")
RANCHER_QA_SPACE = os.environ.get("RANCHER_QA_SPACE", "qa.rancher.space.")
RANCHER_EC2_INSTANCE_CLASS = os.environ.get("AWS_INSTANCE_TYPE", "t3a.medium")

RANCHER_EXTERNAL_DB = os.environ.get("RANCHER_EXTERNAL_DB", "mysql")
RANCHER_EXTERNAL_DB_VERSION = os.environ.get("RANCHER_EXTERNAL_DB_VERSION")
RANCHER_DB_GROUP_NAME = os.environ.get("RANCHER_DB_GROUP_NAME")
RANCHER_DB_MAX_CONNECTIONS = os.environ.get("RANCHER_DB_MAX_CONNECTIONS", 80)
RANCHER_INSTANCE_CLASS = os.environ.get("RANCHER_INSTANCE_CLASS",
                                        "db.t2.micro")
RANCHER_DB_USERNAME = os.environ.get("RANCHER_DB_USERNAME", "adminuser")
RANCHER_DB_PASSWORD = os.environ.get("RANCHER_DB_PASSWORD", "")
RANCHER_K3S_KUBECONFIG_PATH = DATA_SUBDIR + "/k3s_kubeconfig.yaml"
RANCHER_NODE_OS = os.environ.get("RANCHER_NODE_OS", "ubuntu")
RANCHER_INSTALL_MODE = os.environ.get("RANCHER_INSTALL_MODE", "INSTALL_K3S_VERSION")
RANCHER_RDS_ENVIRONMENT = os.environ.get("RANCHER_RDS_ENVIRONMENT", "dev")
RANCHER_RDS_ENGINE_MODE = os.environ.get("RANCHER_RDS_ENGINE_MODE", "provisioned")
RANCHER_CLUSTER_TYPE = os.environ.get("RANCHER_CLUSTER_TYPE", "external_db")
AWS_VOLUME_SIZE = os.environ.get("AWS_VOLUME_SIZE", "8")
RANCHER_RHEL_USERNAME = os.environ.get("RANCHER_RHEL_USERNAME")
RANCHER_RHEL_PASSWORD = os.environ.get("RANCHER_RHEL_PASSWORD")
K3S_CREATE_LB = os.environ.get("K3S_CREATE_LB", False)
RANCHER_OPTIONAL_FILES = os.environ.get("RANCHER_OPTIONAL_FILES")


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


def test_delete_k3s():
    delete_resource_in_AWS_by_prefix(RANCHER_HOSTNAME_PREFIX)


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
    cmd = "kubectl get nodes -o wide --kubeconfig=" + k3s_clusterfilepath
    print(run_command(cmd))
    cmd = "kubectl get pods -A -o wide --kubeconfig=" + k3s_clusterfilepath
    print(run_command(cmd))
    return aws_nodes, client, k3s_clusterfilepath


def create_multiple_control_cluster():
    global RANCHER_EXTERNAL_DB_VERSION
    global RANCHER_DB_GROUP_NAME
    k3s_kubeconfig_file = "k3s_kubeconfig.yaml"
    k3s_clusterfilepath = DATA_SUBDIR + "/" + k3s_kubeconfig_file

    tf_dir = DATA_SUBDIR + "/" + "terraform/k3s/master"
    keyPath = os.path.abspath('.') + '/.ssh/' + AWS_SSH_KEY_NAME
    os.chmod(keyPath, 0o400)
    no_of_servers = int(RANCHER_K3S_NO_OF_SERVER_NODES)
    no_of_servers = no_of_servers - 1

    if RANCHER_EXTERNAL_DB == "MariaDB":
        RANCHER_EXTERNAL_DB_VERSION = "10.3.20" if not RANCHER_EXTERNAL_DB_VERSION \
            else RANCHER_EXTERNAL_DB_VERSION
        RANCHER_DB_GROUP_NAME = "mariadb10.3" if not RANCHER_DB_GROUP_NAME \
            else RANCHER_DB_GROUP_NAME
    elif RANCHER_EXTERNAL_DB == "postgres":
        RANCHER_EXTERNAL_DB_VERSION = "11.5" if not RANCHER_EXTERNAL_DB_VERSION \
            else RANCHER_EXTERNAL_DB_VERSION
        RANCHER_DB_GROUP_NAME = "postgres11" if not RANCHER_DB_GROUP_NAME \
            else RANCHER_DB_GROUP_NAME
    elif RANCHER_EXTERNAL_DB == "aurora-mysql":
        RANCHER_EXTERNAL_DB_VERSION = "5.7.mysql_aurora.2.09.0" if not RANCHER_EXTERNAL_DB_VERSION \
            else RANCHER_EXTERNAL_DB_VERSION
        RANCHER_DB_GROUP_NAME = "aurora-mysql5.7" if not RANCHER_DB_GROUP_NAME \
            else RANCHER_DB_GROUP_NAME
    else:
        RANCHER_EXTERNAL_DB_VERSION = "5.7" if not RANCHER_EXTERNAL_DB_VERSION \
            else RANCHER_EXTERNAL_DB_VERSION
        RANCHER_DB_GROUP_NAME = "mysql5.7" if not RANCHER_DB_GROUP_NAME \
            else RANCHER_DB_GROUP_NAME
    tf = Terraform(working_dir=tf_dir,
                   variables={'region': RANCHER_REGION,
                              'vpc_id': RANCHER_VPC_ID,
                              'subnets': RANCHER_SUBNETS,
                              'sg_id': RANCHER_AWS_SG,
                              'availability_zone': RANCHER_AVAILABILITY_ZONE,
                              'aws_ami': RANCHER_AWS_AMI,
                              'aws_user': RANCHER_AWS_USER,
                              'resource_name': RANCHER_HOSTNAME_PREFIX,
                              'access_key': keyPath,
                              'access_key_name': AWS_SSH_KEY_NAME.replace(".pem", ""),
                              'external_db': RANCHER_EXTERNAL_DB,
                              'external_db_version': RANCHER_EXTERNAL_DB_VERSION,
                              'db_group_name': RANCHER_DB_GROUP_NAME,
                              'instance_class': RANCHER_INSTANCE_CLASS,
                              'max_connections': RANCHER_DB_MAX_CONNECTIONS,
                              'ec2_instance_class': RANCHER_EC2_INSTANCE_CLASS,
                              'db_username': RANCHER_DB_USERNAME,
                              'db_password': RANCHER_DB_PASSWORD,
                              'k3s_version': RANCHER_K3S_VERSION,
                              'k3s_channel': K3S_CHANNEL,
                              'no_of_server_nodes': no_of_servers,
                              'server_flags': RANCHER_K3S_SERVER_FLAGS,
                              'qa_space': RANCHER_QA_SPACE,
                              'node_os': RANCHER_NODE_OS,
                              'username': RANCHER_RHEL_USERNAME,
                              'password': RANCHER_RHEL_PASSWORD,
                              'install_mode': RANCHER_INSTALL_MODE,
                              'engine_mode': RANCHER_RDS_ENGINE_MODE,
                              'environment': RANCHER_RDS_ENVIRONMENT,
                              'cluster_type': RANCHER_CLUSTER_TYPE,
                              'volume_size': AWS_VOLUME_SIZE,
                              'create_lb': str(K3S_CREATE_LB).lower(),
                              'optional_files': RANCHER_OPTIONAL_FILES})
    print("Creating cluster")
    tf.init()
    tf.plan(out="plan_server.out")
    print(tf.apply("--auto-approve"))

    if int(RANCHER_K3S_NO_OF_WORKER_NODES) > 0:
        tf_dir = DATA_SUBDIR + "/" + "terraform/k3s/worker"
        tf = Terraform(working_dir=tf_dir,
                       variables={'region': RANCHER_REGION,
                                  'vpc_id': RANCHER_VPC_ID,
                                  'subnets': RANCHER_SUBNETS,
                                  'sg_id': RANCHER_AWS_SG,
                                  'availability_zone': RANCHER_AVAILABILITY_ZONE,
                                  'aws_ami': RANCHER_AWS_AMI,
                                  'aws_user': RANCHER_AWS_USER,
                                  'ec2_instance_class': RANCHER_EC2_INSTANCE_CLASS,
                                  'resource_name': RANCHER_HOSTNAME_PREFIX,
                                  'access_key': keyPath,
                                  'access_key_name': AWS_SSH_KEY_NAME.replace(".pem", ""),
                                  'k3s_version': RANCHER_K3S_VERSION,
                                  'k3s_channel': K3S_CHANNEL,
                                  'no_of_worker_nodes': int(RANCHER_K3S_NO_OF_WORKER_NODES),
                                  'node_os': RANCHER_NODE_OS,
                                  'username': RANCHER_RHEL_USERNAME,
                                  'password': RANCHER_RHEL_PASSWORD,
                                  'install_mode': RANCHER_INSTALL_MODE,
                                  'volume_size': AWS_VOLUME_SIZE,
                                  'worker_flags': RANCHER_K3S_WORKER_FLAGS})

        print("Joining worker nodes")
        tf.init()
        tf.plan(out="plan_worker.out")
        print(tf.apply("--auto-approve"))

    cmd = "cp /tmp/" + RANCHER_HOSTNAME_PREFIX + "_kubeconfig " + k3s_clusterfilepath
    os.system(cmd)
    is_file = os.path.isfile(k3s_clusterfilepath)
    assert is_file
    print_kubeconfig(k3s_clusterfilepath)
    print("K3s Cluster Created")
    cmd = "kubectl get nodes -o wide --kubeconfig=" + k3s_clusterfilepath
    print(run_command(cmd))
    cmd = "kubectl get pods -o wide -A --kubeconfig=" + k3s_clusterfilepath
    print(run_command(cmd))
    return k3s_clusterfilepath


def create_rancher_cluster(client, k3s_clusterfilepath):
    if CLUSTER_NAME:
        clustername = CLUSTER_NAME
    else:
        clustername = random_test_name("testcustom-k3s")
    cluster = client.create_cluster(name=clustername)
    cluster_token = create_custom_host_registration_token(client, cluster)
    command = cluster_token.insecureCommand
    finalimportcommand = command + " --kubeconfig " + k3s_clusterfilepath

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
        format("INSTALL_K3S_VERSION={}".format(RANCHER_K3S_VERSION) if RANCHER_K3S_VERSION \
                             else "", master.public_ip_address)
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
