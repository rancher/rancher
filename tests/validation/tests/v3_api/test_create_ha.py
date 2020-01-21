import os
import rancher
import time

from .common import create_config_file
from .common import create_user
from .common import random_test_name
from .common import readDataFile
from .common import run_command_with_stderr
from .common import set_url_password_token

from lib.aws import AmazonWebServices

DATA_SUBDIR = os.path.join(os.path.dirname(os.path.realpath(__file__)),
                           'resource')

RANCHER_CHART_VERSION = os.environ.get("RANCHER_CHART_VERSION")
test_run_id = random_test_name("auto")
# if hostname is not provided, generate one ( to support onTag )
RANCHER_HOSTNAME_PREFIX = os.environ.get("RANCHER_HOSTNAME_PREFIX", test_run_id)
resource_suffix = test_run_id + "-" + RANCHER_HOSTNAME_PREFIX
RANCHER_HA_HOSTNAME = RANCHER_HOSTNAME_PREFIX + ".qa.rancher.space"
RANCHER_IMAGE_TAG = os.environ.get("RANCHER_IMAGE_TAG")
RANCHER_SERVER_URL = "https://" + RANCHER_HA_HOSTNAME
RANCHER_HELM_REPO = os.environ.get("RANCHER_HELM_REPO", "latest")
AWS_SSH_KEY_NAME = os.environ.get("AWS_SSH_KEY_NAME")
kubeconfig_path = DATA_SUBDIR + "/kube_config_cluster-ha-filled.yml"
export_cmd = "export KUBECONFIG=" + kubeconfig_path


def test_create_ha():
    print(RANCHER_HA_HOSTNAME)
    nodes = create_resources()
    rke_config = create_rke_cluster_config(nodes)
    create_rke_cluster(rke_config)
    install_cert_manager()
    add_repo_create_namespace()
    install_rancher_self_signed()
    set_url_and_password()


def create_resources():
    # Create nlb and grab ARN & dns name
    lb = AmazonWebServices().create_network_lb(name="nlb-" + resource_suffix)
    lbArn = lb["LoadBalancers"][0]["LoadBalancerArn"]
    lbDns = lb["LoadBalancers"][0]["DNSName"]

    # Upsert the route53 record -- if it exists, update, if not, insert
    AmazonWebServices().upsert_route_53_record_cname(RANCHER_HA_HOSTNAME,
                                                     lbDns)

    # Create the target groups
    targetGroup80 = AmazonWebServices(). \
        create_ha_target_group(80, "tg-80-" + resource_suffix)
    targetGroup443 = AmazonWebServices(). \
        create_ha_target_group(443, "tg-443-" + resource_suffix)
    targetGroup80Arn = targetGroup80["TargetGroups"][0]["TargetGroupArn"]
    targetGroup443Arn = targetGroup443["TargetGroups"][0]["TargetGroupArn"]

    # Create listeners for the load balancer, to forward to the target groups
    AmazonWebServices().create_ha_nlb_listener(loadBalancerARN=lbArn,
                                               port=80,
                                               targetGroupARN=targetGroup80Arn)
    AmazonWebServices().create_ha_nlb_listener(loadBalancerARN=lbArn,
                                               port=443,
                                               targetGroupARN=targetGroup443Arn)

    targets = []
    aws_nodes = \
        AmazonWebServices().create_multiple_nodes(
            3, resource_suffix, wait_for_ready=True)
    assert len(aws_nodes) == 3

    for aws_node in aws_nodes:
        print(aws_node.public_ip_address)
        targets.append(aws_node.provider_node_id)

    # Register the nodes to the target groups
    targets_list = [dict(Id=target_id, Port=80) for target_id in targets]
    AmazonWebServices().register_targets(targets_list,
                                         targetGroup80Arn)
    targets_list = [dict(Id=target_id, Port=443) for target_id in targets]
    AmazonWebServices().register_targets(targets_list,
                                         targetGroup443Arn)
    return aws_nodes


def install_cert_manager():
    helm_certmanager_cmd = \
        export_cmd + " && " + \
        "kubectl apply -f " + \
        "https://raw.githubusercontent.com/jetstack/cert-manager/" + \
        "release-0.12/deploy/manifests/00-crds.yaml && " + \
        "kubectl create namespace cert-manager && " + \
        "helm_v3 repo add jetstack https://charts.jetstack.io && " + \
        "helm_v3 repo update && " + \
        "helm_v3 install cert-manager jetstack/cert-manager " + \
        "--namespace cert-manager --version v0.12.0"

    print(helm_certmanager_cmd)
    out = run_command_with_stderr(helm_certmanager_cmd)
    print(out)
    time.sleep(120)


def add_repo_create_namespace(repo=RANCHER_HELM_REPO):
    helm_repo_cmd = \
        export_cmd + " && helm_v3 repo add rancher-" + repo + \
        " https://releases.rancher.com/server-charts/" + repo + " && " + \
        "helm_v3 repo update"

    print(helm_repo_cmd)
    out = run_command_with_stderr(helm_repo_cmd)
    print(out)

    helm_init_cmd = \
        export_cmd + \
        " && kubectl create namespace cattle-system"
    print(helm_init_cmd)
    out = run_command_with_stderr(helm_init_cmd)
    print(out)


def install_rancher_self_signed(repo=RANCHER_HELM_REPO):
    helm_rancher_cmd = \
        export_cmd + " && helm_v3 install rancher " + \
        "rancher-" + repo + "/rancher " + \
        "--version " + RANCHER_CHART_VERSION + " " \
        "--namespace cattle-system " + \
        "--set hostname=" + RANCHER_HA_HOSTNAME

    if RANCHER_IMAGE_TAG != "":
        helm_rancher_cmd = helm_rancher_cmd + \
            " --set rancherImageTag=${RANCHER_IMAGE_TAG}"

    print(helm_rancher_cmd)
    out = run_command_with_stderr(helm_rancher_cmd)
    print(out)

    time.sleep(120)


def set_url_and_password():
    admin_token = set_url_password_token(RANCHER_SERVER_URL)
    admin_client = rancher.Client(url=RANCHER_SERVER_URL + "/v3",
                                  token=admin_token, verify=False)
    AUTH_URL = RANCHER_SERVER_URL + \
        "/v3-public/localproviders/local?action=login"
    user, user_token = create_user(admin_client, AUTH_URL)
    env_details = "env.CATTLE_TEST_URL='" + RANCHER_SERVER_URL + "'\n"
    env_details += "env.ADMIN_TOKEN='" + admin_token + "'\n"
    env_details += "env.USER_TOKEN='" + user_token + "'\n"
    create_config_file(env_details)


def create_rke_cluster(config_path):
    rke_cmd = "rke --version && rke up --config " + config_path
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
