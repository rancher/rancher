from python_terraform import *  # NOQA
from .common import *  # NOQA


RANCHER_AWS_AMI = os.environ.get("AWS_AMI", "")
RANCHER_AWS_USER = os.environ.get("AWS_USER", "ubuntu")
RANCHER_REGION = os.environ.get("AWS_REGION")
RANCHER_VPC_ID = os.environ.get("AWS_VPC")
RANCHER_SUBNETS = os.environ.get("AWS_SUBNET")
RANCHER_AWS_SG = os.environ.get("AWS_SECURITY_GROUPS")
RANCHER_AVAILABILITY_ZONE = os.environ.get("AWS_AVAILABILITY_ZONE")
RANCHER_QA_SPACE = os.environ.get("RANCHER_QA_SPACE", "qa.rancher.space.")
RANCHER_EC2_INSTANCE_CLASS = os.environ.get("AWS_INSTANCE_TYPE", "t3a.medium")
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
RANCHER_RKE2_SERVER_FLAGS = os.environ.get("RANCHER_RKE2_SERVER_FLAGS", "server")
RANCHER_RKE2_WORKER_FLAGS = os.environ.get("RANCHER_RKE2_WORKER_FLAGS", "agent")
RANCHER_RKE2_OPERATING_SYSTEM = os.environ.get("RANCHER_RKE2_OPERATING_SYSTEM")
AWS_VOLUME_SIZE = os.environ.get("AWS_VOLUME_SIZE", "20")
RANCHER_RKE2_RHEL_USERNAME = os.environ.get("RANCHER_RKE2_RHEL_USERNAME", "")
RANCHER_RKE2_RHEL_PASSWORD = os.environ.get("RANCHER_RKE2_RHEL_PASSWORD", "")
RANCHER_RKE2_KUBECONFIG_PATH = DATA_SUBDIR + "/rke2_kubeconfig.yaml"


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
    no_of_servers = int(RANCHER_RKE2_NO_OF_SERVER_NODES) - 1

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
                              'ec2_instance_class': RANCHER_EC2_INSTANCE_CLASS,
                              'username': RANCHER_RKE2_RHEL_USERNAME,
                              'password': RANCHER_RKE2_RHEL_PASSWORD,
                              'rke2_version': cluster_version,
                              'rke2_channel': RANCHER_RKE2_CHANNEL,
                              'no_of_server_nodes': no_of_servers,
                              'server_flags': RANCHER_RKE2_SERVER_FLAGS,
                              'qa_space': RANCHER_QA_SPACE,
                              'node_os': RANCHER_RKE2_OPERATING_SYSTEM,
                              'cluster_type': cluster_type,
                              'iam_role': RANCHER_IAM_ROLE,
                              'volume_size': AWS_VOLUME_SIZE,
                              'create_lb': str(RKE2_CREATE_LB).lower()})
    print("Creating cluster")
    tf.init()
    tf.plan(out="plan_server.out")
    print(tf.apply("--auto-approve"))
    print("\n\n")
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
                                  'rke2_version': cluster_version,
                                  'rke2_channel': RANCHER_RKE2_CHANNEL,
                                  'username': RANCHER_RKE2_RHEL_USERNAME,
                                  'password': RANCHER_RKE2_RHEL_PASSWORD,
                                  'node_os': RANCHER_RKE2_OPERATING_SYSTEM,
                                  'cluster_type': cluster_type,
                                  'no_of_worker_nodes': int(RANCHER_RKE2_NO_OF_WORKER_NODES),
                                  'worker_flags': RANCHER_RKE2_WORKER_FLAGS,
                                  'iam_role': RANCHER_IAM_ROLE,
                                  'volume_size': AWS_VOLUME_SIZE})

        print("Joining worker nodes")
        tf.init()
        tf.plan(out="plan_worker.out")
        print(tf.apply("--auto-approve"))
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
    nodeNotReady = True
    retries =0
    try:
        while nodeNotReady and (retries < 10):
            cmd = "kubectl get nodes --no-headers -A --kubeconfig=" + kubeconfig
            nodes = execute_command(cmd, False)
            nodeNotReady = False
            for node in nodes.strip().split("\n"):
                state = node.split()[1]
                if state != "Ready":
                    nodeNotReady = True
                if not nodeNotReady:
                    break
            time.sleep(60)
            retries = retries + 1
        if nodeNotReady:
            raise AssertionError("Nodes failed to be in Ready state after 5 min")
        actual_count_of_nodes = len(nodes.strip().split("\n"))
        expected_count_of_nodes = int(RANCHER_RKE2_NO_OF_SERVER_NODES) - 1 + \
                                  int(RANCHER_RKE2_NO_OF_WORKER_NODES)
        if actual_count_of_nodes < expected_count_of_nodes:
            raise AssertionError("Nodes failed to join the cluster, \
            Expected: {} Actual: {}".format(expected_count_of_nodes, actual_count_of_nodes))

        podsNotReady = True
        retries = 0
        while podsNotReady and (retries < 10):
            cmd = "kubectl get pods --no-headers -A --kubeconfig=" + kubeconfig
            pods = execute_command(cmd, False)
            podsNotReady = False
            for pod in pods.strip().split("\n"):
                status = pod.split()[3]
                if status != "Running" and status != "Completed":
                    podsNotReady = True
            if not podsNotReady:
                break
            time.sleep(60)
            retries = retries + 1
        if podsNotReady:
            raise AssertionError("Pods are not in desired state")
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
