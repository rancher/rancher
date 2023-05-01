from python_terraform import *  # NOQA
from .common import *  # NOQA


RANCHER_AWS_AMI = os.environ.get("AWS_AMI", "")
RANCHER_AWS_USER = os.environ.get("AWS_USER", "ubuntu")
RANCHER_AWS_WINDOWS_AMI = os.environ.get("AWS_WINDOWS_AMI", "")
RANCHER_AWS_WINDOWS_USER = os.environ.get("AWS_WINDOWS_USER", "Administrator")
RANCHER_REGION = os.environ.get("AWS_REGION")
RANCHER_VPC_ID = os.environ.get("AWS_VPC")
RANCHER_SUBNETS = os.environ.get("AWS_SUBNET")
RANCHER_AWS_SG = os.environ.get("AWS_SECURITY_GROUPS")
RANCHER_AVAILABILITY_ZONE = os.environ.get("AWS_AVAILABILITY_ZONE")
RANCHER_QA_SPACE = os.environ.get("RANCHER_QA_SPACE", "qa.rancher.space.")
RANCHER_EC2_INSTANCE_CLASS = os.environ.get("AWS_INSTANCE_TYPE", "t3a.medium")
RANCHER_EC2_WINDOWS_INSTANCE_CLASS = os.environ.get("AWS_WINDOWS_INSTANCE_TYPE", "t3.xlarge")
HOST_NAME = os.environ.get('RANCHER_HOST_NAME', "sa")
RANCHER_IAM_ROLE = os.environ.get("RANCHER_IAM_ROLE")
RKE2_CREATE_LB = os.environ.get("RKE2_CREATE_LB", False)

RANCHER_RKE2_VERSION = os.environ.get("RANCHER_RKE2_VERSION", "")
RANCHER_RKE2_CHANNEL = os.environ.get("RANCHER_RKE2_CHANNEL", "null")
RANCHER_RANCHERD_VERSION = os.environ.get("RANCHER_RANCHERD_VERSION", "")
RANCHER_RKE2_NO_OF_SERVER_NODES = \
    os.environ.get("RANCHER_RKE2_NO_OF_SERVER_NODES", 3)
RANCHER_RKE2_NO_OF_WORKER_NODES = \
    os.environ.get("RANCHER_RKE2_NO_OF_WORKER_NODES", 0)
RANCHER_RKE2_NO_OF_WINDOWS_WORKER_NODES = \
    os.environ.get("RANCHER_RKE2_NO_OF_WINDOWS_WORKER_NODES", 0)
RANCHER_RKE2_SERVER_FLAGS = os.environ.get("RANCHER_RKE2_SERVER_FLAGS", "server")
RANCHER_RKE2_WORKER_FLAGS = os.environ.get("RANCHER_RKE2_WORKER_FLAGS", "agent")
RANCHER_RKE2_OPERATING_SYSTEM = os.environ.get("RANCHER_RKE2_OPERATING_SYSTEM")
AWS_VOLUME_SIZE = os.environ.get("AWS_VOLUME_SIZE", "20")
RANCHER_RKE2_RHEL_USERNAME = os.environ.get("RANCHER_RKE2_RHEL_USERNAME", "")
RANCHER_RKE2_RHEL_PASSWORD = os.environ.get("RANCHER_RKE2_RHEL_PASSWORD", "")
RANCHER_RKE2_KUBECONFIG_PATH = DATA_SUBDIR + "/rke2_kubeconfig.yaml"
RKE2_INSTALL_MODE = os.environ.get("RKE2_INSTALL_MODE", "INSTALL_RKE2_VERSION")
RKE2_INSTALL_METHOD = os.environ.get("RKE2_INSTALL_METHOD", "")

RKE2_SPLIT_ROLES = os.environ.get("RKE2_SPLIT_ROLES", False)
RKE2_ETCD_ONLY_NODES = os.environ.get("RKE2_ETCD_ONLY_NODES", 0)
RKE2_ETCD_CP_NODES = os.environ.get("RKE2_ETCD_CP_NODES", 0)
RKE2_ETCD_WORKER_NODES = os.environ.get("RKE2_ETCD_WORKER_NODES", 0)
RKE2_CP_ONLY_NODES = os.environ.get("RKE2_CP_ONLY_NODES", 0)
RKE2_CP_WORKER_NODES = os.environ.get("RKE2_CP_WORKER_NODES", 0)
# Role order corresponds to 1=RANCHER_RKE2_NO_OF_SERVER_NODES,
# 2=RKE2_ETCD_ONLY_NODES 3=RKE2_ETCD_CP_NODES, 4=RKE2_ETCD_WORKER_NODES,
# 5=RKE2_CP_ONLY_NODES, 6=RKE2_CP_WORKER_NODES
RKE2_ROLE_ORDER = os.environ.get("RKE2_ROLE_ORDER", "1,2,3,4,5,6")
RANCHER_OPTIONAL_FILES = os.environ.get("RANCHER_OPTIONAL_FILES")


def test_create_rancherd_multiple_control_cluster():
    cluster_version = RANCHER_RANCHERD_VERSION
    cluster_type = "rancherd"
    rke2_clusterfilepath = create_rke2_multiple_control_cluster(cluster_type, \
                                                                cluster_version)
    fqdn_file = "/tmp/" + RANCHER_HOSTNAME_PREFIX + "_fixed_reg_addr"
    with open(fqdn_file, 'r') as f:
        fqdn = f.read()
        fqdn = fqdn.strip()
    print("RANCHERD URL\nhttps://{0}:8443\n".format(fqdn), flush=True)
    ip_file = "/tmp/" + RANCHER_HOSTNAME_PREFIX + "_master_ip"
    with open(ip_file, 'r') as f:
        ip = f.read()
        ip = ip.strip()
    keyPath = os.path.abspath('.') + '/.ssh/' + AWS_SSH_KEY_NAME
    os.chmod(keyPath, 0o400)
    print("\n\nRANCHERD USERNAME AND PASSWORD\n", flush=True)
    cmd = "ssh -o StrictHostKeyChecking=no -i " + keyPath + " " + RANCHER_AWS_USER + \
          "@" + ip + " rancherd reset-admin"
    result = run_command(cmd, True)
    print(result)


def test_create_rke2_multiple_control_cluster():
    cluster_version = RANCHER_RKE2_VERSION
    cluster_type = "rke2"
    create_rke2_multiple_control_cluster(cluster_type, cluster_version)


def test_import_rke2_multiple_control_cluster():
    client = get_user_client()
    cluster_version = RANCHER_RKE2_VERSION
    cluster_type = "rke2"
    rke2_clusterfilepath = create_rke2_multiple_control_cluster(
        cluster_type, cluster_version)
    cluster = create_rancher_cluster(client, rke2_clusterfilepath)


def create_rke2_multiple_control_cluster(cluster_type, cluster_version):

    rke2_kubeconfig_file = "rke2_kubeconfig.yaml"
    rke2_clusterfilepath = DATA_SUBDIR + "/" + rke2_kubeconfig_file

    tf_dir = DATA_SUBDIR + "/" + "terraform/rke2/master"
    keyPath = os.path.abspath('.') + '/.ssh/' + AWS_SSH_KEY_NAME
    os.chmod(keyPath, 0o400)

    split_roles = str(RKE2_SPLIT_ROLES).lower()
    if split_roles == "true":
        no_of_servers = int(RANCHER_RKE2_NO_OF_SERVER_NODES) + \
            int(RKE2_ETCD_ONLY_NODES) + int(RKE2_ETCD_CP_NODES) + \
            int(RKE2_ETCD_WORKER_NODES) + int(RKE2_CP_ONLY_NODES) + \
            int(RKE2_CP_WORKER_NODES) - 1

        role_order = RKE2_ROLE_ORDER.split(",")
        if int(RANCHER_RKE2_NO_OF_SERVER_NODES) > 0:
            assert "1" in role_order
        if int(RKE2_ETCD_ONLY_NODES) > 0:
            assert "2" in role_order
        if int(RKE2_ETCD_CP_NODES) > 0:
            assert "3" in role_order
        if int(RKE2_ETCD_WORKER_NODES) > 0:
            assert "4" in role_order
        if int(RKE2_CP_ONLY_NODES) > 0:
            assert "5" in role_order
        if int(RKE2_CP_WORKER_NODES) > 0:
            assert "6" in role_order
    else:
        no_of_servers = int(RANCHER_RKE2_NO_OF_SERVER_NODES) - 1

    # Server nodes
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
                              'ec2_instance_class': RANCHER_EC2_INSTANCE_CLASS,
                              'username': RANCHER_RKE2_RHEL_USERNAME,
                              'password': RANCHER_RKE2_RHEL_PASSWORD,
                              'rke2_version': cluster_version,
                              'install_mode': RKE2_INSTALL_MODE,
                              'install_method': RKE2_INSTALL_METHOD,
                              'rke2_channel': RANCHER_RKE2_CHANNEL,
                              'no_of_server_nodes': no_of_servers,
                              'server_flags': RANCHER_RKE2_SERVER_FLAGS,
                              'qa_space': RANCHER_QA_SPACE,
                              'node_os': RANCHER_RKE2_OPERATING_SYSTEM,
                              'cluster_type': cluster_type,
                              'iam_role': RANCHER_IAM_ROLE,
                              'volume_size': AWS_VOLUME_SIZE,
                              'create_lb': str(RKE2_CREATE_LB).lower(),
                              'split_roles': split_roles,
                              'all_role_nodes': RANCHER_RKE2_NO_OF_SERVER_NODES,
                              'etcd_only_nodes': RKE2_ETCD_ONLY_NODES,
                              'etcd_cp_nodes': RKE2_ETCD_CP_NODES,
                              'etcd_worker_nodes': RKE2_ETCD_WORKER_NODES,
                              'cp_only_nodes': RKE2_CP_ONLY_NODES,
                              'cp_worker_nodes': RKE2_CP_WORKER_NODES,
                              'role_order': RKE2_ROLE_ORDER,
                              'optional_files': RANCHER_OPTIONAL_FILES})
    print("Creating cluster")
    tf.init()
    tf.plan(out="plan_server.out")
    print(tf.apply("--auto-approve"))
    print("\n\n")

    # Worker nodes
    if int(RANCHER_RKE2_NO_OF_WORKER_NODES) > 0:
        tf_dir = DATA_SUBDIR + "/" + "terraform/rke2/worker"
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
                                  'rke2_version': cluster_version,
                                  'install_mode': RKE2_INSTALL_MODE,
                                  'install_method': RKE2_INSTALL_METHOD,
                                  'rke2_channel': RANCHER_RKE2_CHANNEL,
                                  'username': RANCHER_RKE2_RHEL_USERNAME,
                                  'password': RANCHER_RKE2_RHEL_PASSWORD,
                                  'node_os': RANCHER_RKE2_OPERATING_SYSTEM,
                                  'cluster_type': cluster_type,
                                  'no_of_worker_nodes': int(RANCHER_RKE2_NO_OF_WORKER_NODES),
                                  'worker_flags': RANCHER_RKE2_WORKER_FLAGS,
                                  'iam_role': RANCHER_IAM_ROLE,
                                  'volume_size': AWS_VOLUME_SIZE})
    
        print("Creating worker nodes")
        tf.init()
        tf.plan(out="plan_worker.out")
        print(tf.apply("--auto-approve"))
        print("Finished Creating worker nodes")
        print("\n\n")
    
    # Windows worker nodes
    if int(RANCHER_RKE2_NO_OF_WINDOWS_WORKER_NODES) > 0:
        tf_dir = DATA_SUBDIR + "/" + "terraform/rke2/windows_worker"
        tf = Terraform(working_dir=tf_dir,
                       variables={'region': RANCHER_REGION,
                                  'vpc_id': RANCHER_VPC_ID,
                                  'subnets': RANCHER_SUBNETS,
                                  'sg_id': RANCHER_AWS_SG,
                                  'availability_zone': RANCHER_AVAILABILITY_ZONE,
                                  'aws_ami': RANCHER_AWS_WINDOWS_AMI,
                                  'aws_user': RANCHER_AWS_WINDOWS_USER,
                                  'ec2_instance_class': RANCHER_EC2_WINDOWS_INSTANCE_CLASS,
                                  'resource_name': RANCHER_HOSTNAME_PREFIX,
                                  'access_key': keyPath,
                                  'access_key_name': AWS_SSH_KEY_NAME.replace(".pem", ""),
                                  'rke2_version': cluster_version,
                                  'node_os': "windows",
                                  'cluster_type': cluster_type,
                                  'no_of_windows_worker_nodes': int(RANCHER_RKE2_NO_OF_WINDOWS_WORKER_NODES),
                                  'iam_role': RANCHER_IAM_ROLE})
    
        print("Creating Windows worker nodes")
        tf.init()
        tf.plan(out="plan_windows_worker.out")
        print(tf.apply("--auto-approve"))
        print("Finished Creating Windows worker nodes")
        print("\n\n")
        
    cmd = "cp /tmp/" + RANCHER_HOSTNAME_PREFIX + "_kubeconfig " + \
          rke2_clusterfilepath
    os.system(cmd)
    is_file = os.path.isfile(rke2_clusterfilepath)
    assert is_file
    print_kubeconfig(rke2_clusterfilepath)
    check_cluster_status(rke2_clusterfilepath)
    print("\n\nRKE2 Cluster Created\n")
    cmd = "kubectl get nodes --kubeconfig=" + rke2_clusterfilepath
    print(run_command(cmd))
    cmd = "kubectl get pods -A --kubeconfig=" + rke2_clusterfilepath
    print(run_command(cmd))
    print("\n\n")
    return rke2_clusterfilepath


def create_rancher_cluster(client, rke2_clusterfilepath):
    if CLUSTER_NAME:
        clustername = CLUSTER_NAME
    else:
        clustername = random_test_name("testcustom-rke2")
    cluster = client.create_cluster(name=clustername)
    cluster_token = create_custom_host_registration_token(client, cluster)
    command = cluster_token.insecureCommand
    finalimportcommand = command + " --kubeconfig " + rke2_clusterfilepath
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


def check_cluster_status(kubeconfig):
    print("Checking cluster status for {} server and {} agents nodes...".format(RANCHER_RKE2_NO_OF_SERVER_NODES, (int(RANCHER_RKE2_NO_OF_WORKER_NODES) + int(RANCHER_RKE2_NO_OF_WINDOWS_WORKER_NODES))))
    retries = 0
    actual_count_of_nodes = 0
    expected_count_of_nodes = int(RANCHER_RKE2_NO_OF_SERVER_NODES) + \
                              int(RANCHER_RKE2_NO_OF_WORKER_NODES) + \
                              int(RANCHER_RKE2_NO_OF_WINDOWS_WORKER_NODES)
    try:
        # Retry logic for matching node count for 5 mins
        while (actual_count_of_nodes < expected_count_of_nodes):
            actual_count_of_nodes = len(get_states("nodes",kubeconfig))
            print("Retrying for 1 min to check node count...")
            time.sleep(60)
            retries = retries + 1
            print("Waiting for agent nodes to join the cluster, retry count: {}".format(retries))
            if (actual_count_of_nodes == expected_count_of_nodes):
                break
            if (retries == 5):
                if (actual_count_of_nodes < expected_count_of_nodes):
                    raise AssertionError("Nodes failed to join the cluster after 5 min, \
                    Expected: {} Actual: {}".format(expected_count_of_nodes, actual_count_of_nodes))
        
        # Retry logic for node status to be Ready for 5 mins
        print("Checking node status....")
        states = get_states("nodes",kubeconfig)
        if 'NotReady' in states:
            nodeNotReady = True
            print("Found one or more nodes in 'NotReady' state")
            retries = 0
            while nodeNotReady and (retries < 5):
                print("Retrying for 1 min to check node status...")
                time.sleep(60)
                retries = retries + 1
                print("Waiting for agent nodes to be ready, retry count: {}".format(retries))
                states = get_states("nodes",kubeconfig)
                if not 'NotReady' in states:
                    nodeNotReady = False
                if (retries == 5):
                    if nodeNotReady:
                        raise AssertionError("Nodes failed to be in Ready state after 5 min, please check logs...")
        print('All nodes found to be in Ready state')
        
        # Retry logic for pods status to be Ready or Completed for 5 mins
        print('Checking pods status...')
        states = get_states("pods",kubeconfig)
        retries = 0
        if not all(state in ['Running','Completed'] for state in states):
            print("Found one or more pods in un-desired status")
            podsNotReady = True
            while podsNotReady and (retries < 5):
                print("Retrying for 1 min to check pod status...")
                time.sleep(60)
                retries = retries + 1
                print("Waiting for pods to be in desired state, retry count: {}".format(retries))
                states = get_states("pods",kubeconfig)
                if all(state in ['Running','Completed'] for state in states):
                    podsNotReady = False
                if (retries == 5):
                    if podsNotReady:
                        raise AssertionError("Pods are not found to be in desired state after 5 min, please check logs...")
        print('All pods found to be in desired state')
    except AssertionError as e:
        print("FAIL: {}".format(str(e)))


def execute_command(command, log_out=True):
    if log_out:
        print("run cmd: \t{0}".format(command))
    for i in range(3):
        try:
            res = subprocess.check_output(command, shell=True, text=True)
        except subprocess.CalledProcessError:
            print("Re-trying...")
            time.sleep(10)
    return res

def get_states(type, kubeconfig):
    if type == "nodes":
        cmd = "kubectl get nodes --no-headers -A --kubeconfig=" + kubeconfig
        cmd_res = execute_command(cmd, False)
        nodes = cmd_res.strip().split("\n")
        states = [node.split()[1] for node in nodes]
        return states
    elif type == "pods":
        cmd = "kubectl get pods --no-headers -A --kubeconfig=" + kubeconfig
        cmd_res = execute_command(cmd, False)
        pods = cmd_res.strip().split("\n")
        states = [pod.split()[3] for pod in pods]
        return states
    else:
        raise AssertionError("Invalid type: {}, only nodes and pods are allowed".format(type))
