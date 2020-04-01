import base64
import os
import pytest
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
RANCHER_LETSENCRYPT_EMAIL = os.environ.get("RANCHER_LETSENCRYPT_EMAIL")
# cert type for HA install [rancher-self-signed, byo-valid, byo-self-signed, letsencrypt]
RANCHER_HA_CERT_OPTION = os.environ.get("RANCHER_HA_CERT_OPTION", "rancher-self-signed")
RANCHER_VALID_TLS_CERT = os.environ.get("RANCHER_VALID_TLS_CERT")
RANCHER_VALID_TLS_KEY = os.environ.get("RANCHER_VALID_TLS_KEY")
RANCHER_BYO_TLS_CERT = os.environ.get("RANCHER_BYO_TLS_CERT")
RANCHER_BYO_TLS_KEY = os.environ.get("RANCHER_BYO_TLS_KEY")
RANCHER_PRIVATE_CA_CERT = os.environ.get("RANCHER_PRIVATE_CA_CERT")
AWS_SSH_KEY_NAME = os.environ.get("AWS_SSH_KEY_NAME")
kubeconfig_path = DATA_SUBDIR + "/kube_config_cluster-ha-filled.yml"
export_cmd = "export KUBECONFIG=" + kubeconfig_path


def test_create_ha():
    cm_install = True
    
    if "byo-" in RANCHER_HA_CERT_OPTION:
        cm_install = False

    ha_setup(install_cm=cm_install)
    install_rancher()
    ha_finalize()

def ha_setup(install_cm=True):
    print(RANCHER_HA_HOSTNAME)
    nodes = create_resources()
    rke_config = create_rke_cluster_config(nodes)
    create_rke_cluster(rke_config)
    if install_cm == True:
        install_cert_manager()
    add_repo_create_namespace()

def ha_finalize():
    set_url_and_password()
    print_kubeconfig()

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

    run_command_with_stderr(helm_certmanager_cmd)
    time.sleep(120)


def add_repo_create_namespace(repo=RANCHER_HELM_REPO):
    helm_repo_cmd = \
        export_cmd + " && helm_v3 repo add rancher-" + repo + \
        " https://releases.rancher.com/server-charts/" + repo + " && " + \
        "helm_v3 repo update"

    run_command_with_stderr(helm_repo_cmd)

    helm_init_cmd = \
        export_cmd + \
        " && kubectl create namespace cattle-system"

    run_command_with_stderr(helm_init_cmd)


def install_rancher(type=RANCHER_HA_CERT_OPTION, repo=RANCHER_HELM_REPO):
    helm_rancher_cmd = \
        export_cmd + " && helm_v3 install rancher " + \
        "rancher-" + repo + "/rancher " + \
        "--version " + RANCHER_CHART_VERSION + " " \
        "--namespace cattle-system " + \
        "--set hostname=" + RANCHER_HA_HOSTNAME
        
    if type == 'letsencrypt':
        helm_rancher_cmd = helm_rancher_cmd + \
            " --set ingress.tls.source=letsEncrypt " + \
            "--set letsEncrypt.email=" + RANCHER_LETSENCRYPT_EMAIL
    elif type == 'byo-self-signed':
        helm_rancher_cmd = helm_rancher_cmd + \
            " --set ingress.tls.source=secret " + \
            "--set privateCA=true"
    elif type == 'byo-valid':
        helm_rancher_cmd = helm_rancher_cmd + \
            " --set ingress.tls.source=secret"
    
    if RANCHER_IMAGE_TAG != "" and RANCHER_IMAGE_TAG is not None:
        helm_rancher_cmd = helm_rancher_cmd + \
            " --set rancherImageTag=" + RANCHER_IMAGE_TAG

    if type == "byo-self-signed":
        create_tls_secrets(valid_cert=False)
    elif type == "byo-valid":
        create_tls_secrets(valid_cert=True)

    run_command_with_stderr(helm_rancher_cmd)
    time.sleep(120)


def create_tls_secrets(valid_cert):
    cert_path = DATA_SUBDIR + "/tls.crt"
    key_path = DATA_SUBDIR + "/tls.key"
    ca_path = DATA_SUBDIR + "/cacerts.pem"

    if valid_cert == True:
        # write files from env var
        write_encoded_certs(cert_path, RANCHER_VALID_TLS_CERT)
        write_encoded_certs(key_path, RANCHER_VALID_TLS_KEY)
    else:
        write_encoded_certs(cert_path, RANCHER_BYO_TLS_CERT)
        write_encoded_certs(key_path, RANCHER_BYO_TLS_KEY)
        write_encoded_certs(ca_path, RANCHER_PRIVATE_CA_CERT)

    tls_command = export_cmd + " && kubectl -n cattle-system create secret tls tls-rancher-ingress --cert=" + cert_path + " --key=" + key_path
    ca_command = export_cmd + " && kubectl -n cattle-system create secret generic tls-ca --from-file=" + ca_path

    run_command_with_stderr(tls_command)

    if valid_cert == False:
        run_command_with_stderr(ca_command)


def write_encoded_certs(path, contents):
    file = open(path, "w")
    file.write(base64.b64decode(contents).decode("utf-8"))
    file.close()

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
    run_command_with_stderr(rke_cmd)


def print_kubeconfig():
    kubeconfig_file = open(kubeconfig_path, "r")
    kubeconfig_contents = kubeconfig_file.read()
    kubeconfig_file.close()
    print("\n\n" + kubeconfig_contents + "\n\n")


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

@pytest.fixture(scope='module', autouse=True)
def precheck_certificate_options():
    if RANCHER_HA_CERT_OPTION == 'byo-valid':
        if RANCHER_VALID_TLS_CERT == '' or RANCHER_VALID_TLS_KEY == '' or \
            RANCHER_VALID_TLS_CERT is None or RANCHER_VALID_TLS_KEY is None:
                raise pytest.skip('Valid certificates not found in environment variables')
    elif RANCHER_HA_CERT_OPTION == 'byo-self-signed':
        if RANCHER_BYO_TLS_CERT == '' or RANCHER_BYO_TLS_KEY == '' or RANCHER_PRIVATE_CA_CERT == '' or \
            RANCHER_BYO_TLS_CERT is None or RANCHER_BYO_TLS_KEY is None or RANCHER_PRIVATE_CA_CERT is None:
            raise pytest.skip('Self signed certificates not found in environment variables')
    elif RANCHER_HA_CERT_OPTION == 'letsencrypt':
        if RANCHER_LETSENCRYPT_EMAIL == '' or RANCHER_LETSENCRYPT_EMAIL is None:
            raise pytest.skip('LetsEncrypt email is not found in environment variables')