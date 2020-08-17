from python_terraform import *  # NOQA
from .common import *  # NOQA


RANCHER_AWS_AMI = os.environ.get("AWS_AMI", "")
RANCHER_AWS_USER = os.environ.get("AWS_USER", "ubuntu")
RANCHER_REGION = os.environ.get("AWS_REGION")
RANCHER_VPC_ID = os.environ.get("AWS_VPC")
RANCHER_SUBNETS = os.environ.get("AWS_SUBNET")
RANCHER_AWS_SG = os.environ.get("AWS_SG")
RANCHER_AVAILABILITY_ZONE = os.environ.get("AWS_AVAILABILITY_ZONE")
RANCHER_QA_SPACE = os.environ.get("RANCHER_QA_SPACE", "qa.rancher.space.")
RANCHER_EC2_INSTANCE_CLASS = os.environ.get("RANCHER_EC2_INSTANCE_CLASS",
                                            "t3a.medium")
HOST_NAME = os.environ.get('RANCHER_HOST_NAME', "sa")
RANCHER_IAM_ROLE = os.environ.get("RANCHER_IAM_ROLE")

RANCHER_RKE2_VERSION = os.environ.get("RANCHER_RKE2_VERSION", "")
RANCHER_RKE2_NO_OF_SERVER_NODES = \
    os.environ.get("RANCHER_RKE2_NO_OF_SERVER_NODES", 3)
RANCHER_RKE2_NO_OF_WORKER_NODES = \
    os.environ.get("RANCHER_RKE2_NO_OF_WORKER_NODES", 0)
RANCHER_RKE2_SERVER_FLAGS = os.environ.get("RANCHER_RKE2_SERVER_FLAGS", "server")
RANCHER_RKE2_WORKER_FLAGS = os.environ.get("RANCHER_RKE2_WORKER_FLAGS", "agent")
RANCHER_RKE2_CLUSTERTYPE = os.environ.get("RANCHER_RKE2_CLUSTERTYPE")

RANCHER_RKE2_RHEL_USERNAME = os.environ.get("RANCHER_RKE2_RHEL_USERNAME", "")
RANCHER_RKE2_RHEL_PASSWORD = os.environ.get("RANCHER_RKE2_RHEL_PASSWORD", "")
RANCHER_RKE2_KUBECONFIG_PATH = DATA_SUBDIR + "/rke2_kubeconfig.yaml"


def test_create_rke2_multiple_control_cluster():
    rke2_clusterfilepath = create_rke2_multiple_control_cluster()


def test_import_rke2_multiple_control_cluster():
    client = get_user_client()
    rke2_clusterfilepath = create_rke2_multiple_control_cluster()
    cluster = create_rancher_cluster(client, rke2_clusterfilepath)


def create_rke2_multiple_control_cluster():

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
                              'rke2_version': RANCHER_RKE2_VERSION,
                              'no_of_server_nodes': no_of_servers,
                              'server_flags': RANCHER_RKE2_SERVER_FLAGS,
                              'qa_space': RANCHER_QA_SPACE,
                              'ctype': RANCHER_RKE2_CLUSTERTYPE,
                              'iam_role': RANCHER_IAM_ROLE})
    print("Creating cluster")
    tf.init()
    print(tf.plan(out="plan_server.out"))
    print("\n\n")
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
                                  'rke2_version': RANCHER_RKE2_VERSION,
                                  'username': RANCHER_RKE2_RHEL_USERNAME,
                                  'password': RANCHER_RKE2_RHEL_PASSWORD,
                                  'ctype': RANCHER_RKE2_CLUSTERTYPE,
                                  'no_of_worker_nodes': int(RANCHER_RKE2_NO_OF_WORKER_NODES),
                                  'worker_flags': RANCHER_RKE2_WORKER_FLAGS,
                                  'iam_role': RANCHER_IAM_ROLE})

    print("Joining worker nodes")
    tf.init()
    print(tf.plan(out="plan_worker.out"))
    print("\n\n")
    print(tf.apply("--auto-approve"))
    print("\n\n")
    cmd = "cp /tmp/" + RANCHER_HOSTNAME_PREFIX + "_kubeconfig " + rke2_clusterfilepath
    os.system(cmd)
    is_file = os.path.isfile(rke2_clusterfilepath)
    assert is_file
    print(rke2_clusterfilepath)
    with open(rke2_clusterfilepath, 'r') as f:
        print(f.read())
    print("RKE2 Cluster Created")
    return rke2_clusterfilepath


def create_rancher_cluster(client, rke2_clusterfilepath):
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
