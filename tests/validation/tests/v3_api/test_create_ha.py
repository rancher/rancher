from .common import *  # NOQA
from .test_boto_create_eks import get_eks_kubeconfig
from .test_import_k3s_cluster import create_multiple_control_cluster
from .test_rke_cluster_provisioning import rke_config

# RANCHER_HA_KUBECONFIG and RANCHER_HA_HOSTNAME are provided
# when installing Rancher into a k3s setup
RANCHER_HA_KUBECONFIG = os.environ.get("RANCHER_HA_KUBECONFIG")
RANCHER_HA_HOSTNAME = os.environ.get(
    "RANCHER_HA_HOSTNAME", RANCHER_HOSTNAME_PREFIX + ".qa.rancher.space")
resource_prefix = RANCHER_HA_HOSTNAME.split(".qa.rancher.space")[0]
RANCHER_SERVER_URL = "https://" + RANCHER_HA_HOSTNAME

RANCHER_CHART_VERSION = os.environ.get("RANCHER_CHART_VERSION")
RANCHER_IMAGE_TAG = os.environ.get("RANCHER_IMAGE_TAG")
RANCHER_HELM_REPO = os.environ.get("RANCHER_HELM_REPO", "latest")
RANCHER_LETSENCRYPT_EMAIL = os.environ.get("RANCHER_LETSENCRYPT_EMAIL")
# Here is the list of cert types for HA install
# [rancher-self-signed, byo-valid, byo-self-signed, letsencrypt]
RANCHER_HA_CERT_OPTION = os.environ.get("RANCHER_HA_CERT_OPTION",
                                        "rancher-self-signed")
RANCHER_VALID_TLS_CERT = os.environ.get("RANCHER_VALID_TLS_CERT")
RANCHER_VALID_TLS_KEY = os.environ.get("RANCHER_VALID_TLS_KEY")
RANCHER_BYO_TLS_CERT = os.environ.get("RANCHER_BYO_TLS_CERT")
RANCHER_BYO_TLS_KEY = os.environ.get("RANCHER_BYO_TLS_KEY")
RANCHER_PRIVATE_CA_CERT = os.environ.get("RANCHER_PRIVATE_CA_CERT")

RANCHER_LOCAL_CLUSTER_TYPE = os.environ.get("RANCHER_LOCAL_CLUSTER_TYPE")
RANCHER_ADD_CUSTOM_CLUSTER = os.environ.get("RANCHER_ADD_CUSTOM_CLUSTER",
                                            "True")

kubeconfig_path = DATA_SUBDIR + "/kube_config_cluster-ha-filled.yml"
export_cmd = "export KUBECONFIG=" + kubeconfig_path


def test_remove_rancher_ha():
    assert CATTLE_TEST_URL.endswith(".qa.rancher.space"), \
        "the CATTLE_TEST_URL need to end with .qa.rancher.space"
    if not check_if_ok(CATTLE_TEST_URL):
        print("skip deleting clusters within the setup")
    else:
        print("the CATTLE_TEST_URL is accessible")
        admin_token = get_user_token("admin", ADMIN_PASSWORD)
        client = get_client_for_token(admin_token)

        # delete clusters except the local cluster
        clusters = client.list_cluster(id_ne="local").data
        print("deleting the following clusters: {}"
              .format([cluster.name for cluster in clusters]))
        for cluster in clusters:
            print("deleting the following cluster : {}".format(cluster.name))
            delete_cluster(client, cluster)

    resource_prefix = \
        CATTLE_TEST_URL.split(".qa.rancher.space")[0].split("//")[1]
    delete_resource_in_AWS_by_prefix(resource_prefix)


def test_install_rancher_ha(precheck_certificate_options):
    cm_install = True
    extra_settings = []
    if "byo-" in RANCHER_HA_CERT_OPTION:
        cm_install = False
    print("The hostname is: {}".format(RANCHER_HA_HOSTNAME))

    # prepare an RKE cluster and other resources
    # if no kubeconfig file is provided
    if RANCHER_HA_KUBECONFIG is None:
        if RANCHER_LOCAL_CLUSTER_TYPE == "RKE":
            print("RKE cluster is provisioning for the local cluster")
            nodes = create_resources()
            config_path = create_rke_cluster_config(nodes)
            create_rke_cluster(config_path)
        elif RANCHER_LOCAL_CLUSTER_TYPE == "K3S":
            print("K3S cluster is provisioning for the local cluster")
            k3s_kubeconfig_path = \
                create_multiple_control_cluster()
            cmd = "cp {0} {1}".format(k3s_kubeconfig_path, kubeconfig_path)
            run_command_with_stderr(cmd)
        elif RANCHER_LOCAL_CLUSTER_TYPE == "EKS":
            create_resources_eks()
            eks_kubeconfig_path = get_eks_kubeconfig(resource_prefix +
                                                     "-ekscluster")
            cmd = "cp {0} {1}".format(eks_kubeconfig_path, kubeconfig_path)
            run_command_with_stderr(cmd)
            install_eks_ingress()
            extra_settings.append(
                "--set ingress."
                "extraAnnotations.\"kubernetes\\.io/ingress\\.class\"=nginx"
            )
    else:
        write_kubeconfig()

    # wait until the cluster is ready
    def valid_response():
        output = run_command_with_stderr(export_cmd + " && kubectl get nodes")
        return "Ready" in output.decode()

    try:
        wait_for(valid_response)
    except Exception as e:
        print("Error: {0}".format(e))
        assert False, "check the logs in console for details"

    if cm_install:
        install_cert_manager()
    add_repo_create_namespace()
    # Here we use helm to install the Rancher chart
    install_rancher(extra_settings=extra_settings)
    if RANCHER_LOCAL_CLUSTER_TYPE == "EKS":
        # For EKS we need to wait for EKS to generate the nlb and then configure
        # a Route53 record with the ingress address value
        set_route53_with_eks_ingress()
    wait_for_status_code(url=RANCHER_SERVER_URL + "/v3", expected_code=401)
    print_kubeconfig()
    auth_url = \
        RANCHER_SERVER_URL + "/v3-public/localproviders/local?action=login"
    wait_for_status_code(url=auth_url, expected_code=200)
    admin_client = set_url_and_password(RANCHER_SERVER_URL)
    cluster = get_cluster_by_name(admin_client, "local")
    validate_cluster_state(admin_client, cluster, False)
    if RANCHER_ADD_CUSTOM_CLUSTER.upper() == "TRUE":
        print("creating an custom cluster")
        create_custom_cluster(admin_client)


def create_custom_cluster(admin_client):
    auth_url = RANCHER_SERVER_URL + \
        "/v3-public/localproviders/local?action=login"
    wait_for_status_code(url=auth_url, expected_code=200)
    user, user_token = create_user(admin_client, auth_url)

    aws_nodes = \
        AmazonWebServices().create_multiple_nodes(
            5, random_test_name(resource_prefix + "-custom"))
    node_roles = [["controlplane"], ["etcd"],
                  ["worker"], ["worker"], ["worker"]]
    client = rancher.Client(url=RANCHER_SERVER_URL + "/v3",
                            token=user_token, verify=False)
    cluster = client.create_cluster(
        name=random_name(),
        driver="rancherKubernetesEngine",
        rancherKubernetesEngineConfig=rke_config)
    assert cluster.state == "provisioning"
    i = 0
    for aws_node in aws_nodes:
        docker_run_cmd = \
            get_custom_host_registration_cmd(
                client, cluster, node_roles[i], aws_node)
        aws_node.execute_command(docker_run_cmd)
        i += 1
    validate_cluster(client, cluster, userToken=user_token)


def test_upgrade_rancher_ha(precheck_upgrade_options):
    write_kubeconfig()
    add_repo_create_namespace()
    install_rancher(upgrade=True)


def create_resources_eks():
    cluster_name = resource_prefix + "-ekscluster"
    AmazonWebServices().create_eks_cluster(cluster_name)
    AmazonWebServices().wait_for_eks_cluster_state(cluster_name, "ACTIVE")


def create_resources():
    # Create nlb and grab ARN & dns name
    lb = AmazonWebServices().create_network_lb(name=resource_prefix + "-nlb")
    lbArn = lb["LoadBalancers"][0]["LoadBalancerArn"]
    lbDns = lb["LoadBalancers"][0]["DNSName"]

    # Upsert the route53 record -- if it exists, update, if not, insert
    AmazonWebServices().upsert_route_53_record_cname(RANCHER_HA_HOSTNAME,
                                                     lbDns)

    # Create the target groups
    tg80 = AmazonWebServices(). \
        create_ha_target_group(80, resource_prefix + "-tg-80")
    tg443 = AmazonWebServices(). \
        create_ha_target_group(443, resource_prefix + "-tg-443")
    tg80Arn = tg80["TargetGroups"][0]["TargetGroupArn"]
    tg443Arn = tg443["TargetGroups"][0]["TargetGroupArn"]

    # Create listeners for the load balancer, to forward to the target groups
    AmazonWebServices().create_ha_nlb_listener(loadBalancerARN=lbArn,
                                               port=80,
                                               targetGroupARN=tg80Arn)
    AmazonWebServices().create_ha_nlb_listener(loadBalancerARN=lbArn,
                                               port=443,
                                               targetGroupARN=tg443Arn)
    targets = []
    aws_nodes = AmazonWebServices().\
        create_multiple_nodes(3, resource_prefix + "-server")
    assert len(aws_nodes) == 3

    for aws_node in aws_nodes:
        print(aws_node.public_ip_address)
        targets.append(aws_node.provider_node_id)

    # Register the nodes to the target groups
    targets_list = [dict(Id=target_id, Port=80) for target_id in targets]
    AmazonWebServices().register_targets(targets_list, tg80Arn)
    targets_list = [dict(Id=target_id, Port=443) for target_id in targets]
    AmazonWebServices().register_targets(targets_list, tg443Arn)
    return aws_nodes


def install_cert_manager():
    manifests = "https://github.com/jetstack/cert-manager/releases/download/" \
                "v0.15.0/cert-manager.crds.yaml"
    cm_repo = "https://charts.jetstack.io"

    run_command_with_stderr(export_cmd + " && kubectl apply -f " + manifests)
    run_command_with_stderr("helm_v3 repo add jetstack " + cm_repo)
    run_command_with_stderr("helm_v3 repo update")
    run_command_with_stderr(export_cmd + " && " +
                            "kubectl create namespace cert-manager")
    run_command_with_stderr(export_cmd + " && " +
                            "helm_v3 install cert-manager "
                            "jetstack/cert-manager "
                            "--namespace cert-manager "
                            "--version v0.15.0")
    time.sleep(120)


def install_eks_ingress():
    run_command_with_stderr(export_cmd + " && kubectl apply -f " +
                            DATA_SUBDIR + "/eks_nlb.yml")


def set_route53_with_eks_ingress():
    kubectl_ingress = "kubectl get ingress -n cattle-system -o " \
                      "jsonpath=\"" \
                      "{.items[0].status.loadBalancer.ingress[0].hostname}\""
    ingress_address = run_command_with_stderr(export_cmd
                                              + " && " +
                                              kubectl_ingress).decode()
    AmazonWebServices().upsert_route_53_record_cname(RANCHER_HA_HOSTNAME,
                                                     ingress_address)
    time.sleep(60)


def add_repo_create_namespace(repo=RANCHER_HELM_REPO):
    repo_name = "rancher-" + repo
    repo_url = "https://releases.rancher.com/server-charts/" + repo

    run_command_with_stderr("helm_v3 repo add " + repo_name + " " + repo_url)
    run_command_with_stderr("helm_v3 repo update")
    run_command_with_stderr(export_cmd + " && " +
                            "kubectl create namespace cattle-system")


def install_rancher(type=RANCHER_HA_CERT_OPTION, repo=RANCHER_HELM_REPO,
                    upgrade=False, extra_settings=None):
    operation = "install"

    if upgrade:
        operation = "upgrade"

    helm_rancher_cmd = \
        export_cmd + " && helm_v3 " + operation + " rancher " + \
        "rancher-" + repo + "/rancher " + \
        "--version " + RANCHER_CHART_VERSION + " " + \
        "--namespace cattle-system " + \
        "--set hostname=" + RANCHER_HA_HOSTNAME

    if type == 'letsencrypt':
        helm_rancher_cmd = \
            helm_rancher_cmd + \
            " --set ingress.tls.source=letsEncrypt " + \
            "--set letsEncrypt.email=" + \
            RANCHER_LETSENCRYPT_EMAIL
    elif type == 'byo-self-signed':
        helm_rancher_cmd = \
            helm_rancher_cmd + \
            " --set ingress.tls.source=secret " + \
            "--set privateCA=true"
    elif type == 'byo-valid':
        helm_rancher_cmd = \
            helm_rancher_cmd + \
            " --set ingress.tls.source=secret"

    if RANCHER_IMAGE_TAG != "" and RANCHER_IMAGE_TAG is not None:
        helm_rancher_cmd = \
            helm_rancher_cmd + \
            " --set rancherImageTag=" + RANCHER_IMAGE_TAG

    if operation == "install":
        if type == "byo-self-signed":
            create_tls_secrets(valid_cert=False)
        elif type == "byo-valid":
            create_tls_secrets(valid_cert=True)

    if extra_settings:
        for setting in extra_settings:
            helm_rancher_cmd = helm_rancher_cmd + " " + setting

    run_command_with_stderr(helm_rancher_cmd)
    time.sleep(120)


def create_tls_secrets(valid_cert):
    cert_path = DATA_SUBDIR + "/tls.crt"
    key_path = DATA_SUBDIR + "/tls.key"
    ca_path = DATA_SUBDIR + "/cacerts.pem"

    if valid_cert:
        # write files from env var
        write_encoded_certs(cert_path, RANCHER_VALID_TLS_CERT)
        write_encoded_certs(key_path, RANCHER_VALID_TLS_KEY)
    else:
        write_encoded_certs(cert_path, RANCHER_BYO_TLS_CERT)
        write_encoded_certs(key_path, RANCHER_BYO_TLS_KEY)
        write_encoded_certs(ca_path, RANCHER_PRIVATE_CA_CERT)

    tls_command = export_cmd + " && kubectl -n cattle-system " \
                               "create secret tls tls-rancher-ingress " \
                               "--cert=" + cert_path + " --key=" + key_path
    ca_command = export_cmd + " && kubectl -n cattle-system " \
                              "create secret generic tls-ca " \
                              "--from-file=" + ca_path

    run_command_with_stderr(tls_command)

    if not valid_cert:
        run_command_with_stderr(ca_command)


def write_encoded_certs(path, contents):
    file = open(path, "w")
    file.write(base64.b64decode(contents).decode("utf-8"))
    file.close()


def write_kubeconfig():
    file = open(kubeconfig_path, "w")
    file.write(base64.b64decode(RANCHER_HA_KUBECONFIG).decode("utf-8"))
    file.close()


def set_url_and_password(rancher_url, server_url=None):
    admin_token = set_url_password_token(rancher_url, server_url)
    admin_client = rancher.Client(url=rancher_url + "/v3",
                                  token=admin_token, verify=False)
    auth_url = rancher_url + "/v3-public/localproviders/local?action=login"
    user, user_token = create_user(admin_client, auth_url)
    env_details = "env.CATTLE_TEST_URL='" + rancher_url + "'\n"
    env_details += "env.ADMIN_TOKEN='" + admin_token + "'\n"
    env_details += "env.USER_TOKEN='" + user_token + "'\n"
    create_config_file(env_details)
    return admin_client


def create_rke_cluster(config_path):
    rke_cmd = "rke --version && rke up --config " + config_path
    run_command_with_stderr(rke_cmd)


def print_kubeconfig():
    kubeconfig_file = open(kubeconfig_path, "r")
    kubeconfig_contents = kubeconfig_file.read()
    kubeconfig_file.close()
    kubeconfig_contents_encoded = base64.b64encode(
        kubeconfig_contents.encode("utf-8")).decode("utf-8")
    print("\n\n" + kubeconfig_contents + "\n\n")
    print("\nBase64 encoded: \n\n" + kubeconfig_contents_encoded + "\n\n")


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

    rkeconfig = rkeconfig.replace("$user1", aws_nodes[0].ssh_user)
    rkeconfig = rkeconfig.replace("$user2", aws_nodes[1].ssh_user)
    rkeconfig = rkeconfig.replace("$user3", aws_nodes[2].ssh_user)

    rkeconfig = rkeconfig.replace("$AWS_SSH_KEY_NAME", AWS_SSH_KEY_NAME)
    print("cluster-ha-filled.yml: \n" + rkeconfig + "\n")

    clusterfilepath = DATA_SUBDIR + "/" + "cluster-ha-filled.yml"

    f = open(clusterfilepath, "w")
    f.write(rkeconfig)
    f.close()
    return clusterfilepath


@pytest.fixture(scope='module')
def precheck_certificate_options():
    if RANCHER_HA_CERT_OPTION == 'byo-valid':
        if RANCHER_VALID_TLS_CERT == '' or \
           RANCHER_VALID_TLS_KEY == '' or \
           RANCHER_VALID_TLS_CERT is None or \
           RANCHER_VALID_TLS_KEY is None:
            raise pytest.skip(
                'Valid certificates not found in environment variables')
    elif RANCHER_HA_CERT_OPTION == 'byo-self-signed':
        if RANCHER_BYO_TLS_CERT == '' or \
           RANCHER_BYO_TLS_KEY == '' or \
           RANCHER_PRIVATE_CA_CERT == '' or \
           RANCHER_BYO_TLS_CERT is None or \
           RANCHER_BYO_TLS_KEY is None or \
           RANCHER_PRIVATE_CA_CERT is None:
            raise pytest.skip(
                'Self signed certificates not found in environment variables')
    elif RANCHER_HA_CERT_OPTION == 'letsencrypt':
        if RANCHER_LETSENCRYPT_EMAIL == '' or \
           RANCHER_LETSENCRYPT_EMAIL is None:
            raise pytest.skip(
                'LetsEncrypt email is not found in environment variables')


@pytest.fixture(scope='module')
def precheck_upgrade_options():
    if RANCHER_HA_KUBECONFIG == '' or RANCHER_HA_KUBECONFIG is None:
        raise pytest.skip('Kubeconfig is not found for upgrade!')
    if RANCHER_HA_HOSTNAME == '' or RANCHER_HA_HOSTNAME is None:
        raise pytest.skip('Hostname is not found for upgrade!')
