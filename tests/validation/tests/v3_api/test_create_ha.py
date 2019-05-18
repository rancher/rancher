from .common import * # NOQA 
from lib.aws import * # NOQA 

DATA_SUBDIR = os.path.join(os.path.dirname(os.path.realpath(__file__)),
                           'resource')
RANCHER_HA_TARGET_GROUP_80 = os.environ.get("RANCHER_HA_TARGET_GROUP_80")
RANCHER_HA_TARGET_GROUP_443 = os.environ.get("RANCHER_HA_TARGET_GROUP_443")
RANCHER_CHART_VERSION = os.environ.get("RANCHER_CHART_VERSION")
RANCHER_HA_HOSTNAME = os.environ.get("RANCHER_HA_HOSTNAME")
RANCHER_HA_INSTANCE_NAME = "qa-ha-" + RANCHER_HA_HOSTNAME

# Requires the following env vars to be available:
# RANCHER_HA_HOSTNAME - hostname only, no prefix
# RANCHER_HA_TARGET_GROUP_80 - ARN of target group for port 80 
# RANCHER_HA_TARGET_GROUP_443 - ARN of target group for port 443
# AWS_SSH_KEY_NAME - Keypair name (must be in selected region)
# AWS_SSH_PEM_KEY - Key content
# AWS_ACCESS_KEY_ID
# AWS_SECRET_ACCESS_KEY
# DOCKER_INSTALLED - Should be false
# AWS_REGION - Region for the instances / target groups
# AWS_REGION_AZ - AZ for the instances
# AWS_SECURITY_GROUPS - SG for the instances
# RANCHER_CHART_VERSION - Chart version, not Rancher tag (2.2.2, not v2.2.2)
# ADMIN_PASSWORD - Admin password will be set to this
 
def test_create_ha():
    nodes = create_nodes_and_register()
    rke_config = create_rke_cluster_config(nodes)
    create_rke_cluster(rke_config)
    install_helm_tiller_rancher()


def create_nodes_and_register():
    AmazonWebServices().deregister_all_targets(RANCHER_HA_TARGET_GROUP_80)
    AmazonWebServices().deregister_all_targets(RANCHER_HA_TARGET_GROUP_443)

    targets = []
    aws_nodes = \
        AmazonWebServices().create_multiple_nodes(
            3, random_test_name(RANCHER_HA_INSTANCE_NAME), wait_for_ready=True)
    assert len(aws_nodes) == 3

    for aws_node in aws_nodes:
        print(aws_node.public_ip_address)
        targets.append(aws_node.provider_node_id)

    targets_list = [dict(Id=target_id, Port=80) for target_id in targets]
    AmazonWebServices().register_targets(targets_list,
                                         RANCHER_HA_TARGET_GROUP_80)
    targets_list = [dict(Id=target_id, Port=443) for target_id in targets]
    AmazonWebServices().register_targets(targets_list,
                                         RANCHER_HA_TARGET_GROUP_443)
    return aws_nodes


def install_helm_tiller_rancher():
    kubeconfig_path = DATA_SUBDIR + "/kube_config_cluster-ha-filled.yml"
    export_cmd = "export KUBECONFIG=" + kubeconfig_path
    print(export_cmd)

    helm_init_cmd = \
        export_cmd + \
        " && kubectl -n kube-system create serviceaccount tiller && " + \
        "kubectl create clusterrolebinding tiller " + \
        "--clusterrole cluster-admin " + \
        "--serviceaccount=kube-system:tiller && " + \
        "helm init --service-account tiller"

    out = run_command_with_stderr(helm_init_cmd)
    print(out)
    time.sleep(60)

    helm_certmanager_cmd = \
        export_cmd + " && helm install stable/cert-manager " + \
        "--name cert-manager " + \
        "--namespace kube-system " + \
        "--version v0.5.2"

    out = run_command_with_stderr(helm_certmanager_cmd)
    print(out)
    time.sleep(15)

    helm_repo_cmd = \
        export_cmd + " && helm repo add rancher-latest " + \
        "https://releases.rancher.com/server-charts/latest && " + \
        "helm repo update"

    out = run_command_with_stderr(helm_repo_cmd)
    print(out)
    time.sleep(15)

    helm_rancher_cmd = \
        export_cmd + " && helm install rancher-latest/rancher " + \
        "--version " + RANCHER_CHART_VERSION + " " \
        "--name rancher " + \
        "--namespace cattle-system " + \
        "--set hostname=" + RANCHER_HA_HOSTNAME

    out = run_command_with_stderr(helm_rancher_cmd)
    print(out)

    RANCHER_SERVER_URL = "https://" + RANCHER_HA_HOSTNAME
    
    time.sleep(60)

    admin_token = set_url_password_token(RANCHER_SERVER_URL)
    env_details = "env.CATTLE_TEST_URL='" + RANCHER_SERVER_URL + "'\n"
    env_details += "env.ADMIN_TOKEN='" + admin_token + "'\n"
    create_config_file(env_details)    


def create_rke_cluster(config_path):
    rke_cmd = "rke up --config " + config_path
    result = run_command_with_stderr(rke_cmd)
    print(result)


def create_rke_cluster_config(aws_nodes):
    configfile = "cluster-ha.yml"

    rkeconfig = readDataFile(DATA_SUBDIR, configfile)
    rkeconfig = rkeconfig.replace("$ip1", aws_nodes[0].public_ip_address)
    rkeconfig = rkeconfig.replace("$ip2", aws_nodes[1].public_ip_address)
    rkeconfig = rkeconfig.replace("$ip3", aws_nodes[2].public_ip_address)

    rkeconfig = rkeconfig.replace("$internalIp1",
                                  aws_nodes[0].private_ip_address)
    rkeconfig = rkeconfig.replace("$internalIp2",
                                  aws_nodes[1].private_ip_address)
    rkeconfig = rkeconfig.replace("$internalIp3",
                                  aws_nodes[2].private_ip_address)

    rkeconfig = rkeconfig.replace("$AWS_SSH_KEY_NAME", AWS_SSH_KEY_NAME)

    clusterfilepath = DATA_SUBDIR + "/" + "cluster-ha-filled.yml"

    f = open(clusterfilepath, "w")
    f.write(rkeconfig)
    f.close()
    return clusterfilepath
