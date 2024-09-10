from python_terraform import * # NOQA
from pkg_resources import packaging

from .common import *  # NOQA
from .test_boto_create_eks import get_eks_kubeconfig
from .test_import_k3s_cluster import create_multiple_control_cluster
from .test_import_rke2_cluster import (
    RANCHER_RKE2_VERSION, create_rke2_multiple_control_cluster
)
from .test_rke_cluster_provisioning import AZURE_CLIENT_ID, AZURE_CLIENT_SECRET, rke_config
from packaging import version

# RANCHER_HA_KUBECONFIG and RANCHER_HA_HOSTNAME are provided
# when installing Rancher into a k3s setup
RANCHER_HA_KUBECONFIG = os.environ.get("RANCHER_HA_KUBECONFIG")
RANCHER_HA_HARDENED = ast.literal_eval(os.environ.get("RANCHER_HA_HARDENED", "False"))
RANCHER_PSP_ENABLED = ast.literal_eval(os.environ.get("RANCHER_PSP_ENABLED", "True"))
RANCHER_HA_HOSTNAME = os.environ.get(
    "RANCHER_HA_HOSTNAME", RANCHER_HOSTNAME_PREFIX + ".qa.rancher.space")
resource_prefix = RANCHER_HA_HOSTNAME.split(".qa.rancher.space")[0]
RANCHER_SERVER_URL = "https://" + RANCHER_HA_HOSTNAME

RANCHER_CHART_VERSION = os.environ.get("RANCHER_CHART_VERSION")
RANCHER_HELM_EXTRA_SETTINGS = os.environ.get("RANCHER_HELM_EXTRA_SETTINGS")
RANCHER_IMAGE_TAG = os.environ.get("RANCHER_IMAGE_TAG")
RANCHER_HELM_REPO = os.environ.get("RANCHER_HELM_REPO", "latest")
RANCHER_HELM_URL = os.environ.get("RANCHER_HELM_URL", "https://releases.rancher.com/server-charts/")
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
KUBERNETES_VERSION = os.environ.get("RANCHER_LOCAL_KUBERNETES_VERSION","")

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
    profile = 'rke-cis-1.5'
    if "byo-" in RANCHER_HA_CERT_OPTION:
        cm_install = False
    print("The hostname is: {}".format(RANCHER_HA_HOSTNAME))

    # prepare an RKE cluster and other resources
    # if no kubeconfig file is provided
    if RANCHER_HA_KUBECONFIG is None:
        if RANCHER_LOCAL_CLUSTER_TYPE == "RKE":
            print("RKE cluster is provisioning for the local cluster")
            nodes = create_resources()
            if RANCHER_HA_HARDENED:
                node_role = [["worker", "controlplane", "etcd"]]
                node_roles =[]
                for role in node_role:
                    node_roles.extend([role, role, role])
                prepare_hardened_nodes(nodes, profile, node_roles)
            config_path = create_rke_cluster_config(nodes)
            create_rke_cluster(config_path)
        elif RANCHER_LOCAL_CLUSTER_TYPE == "K3S":
            print("K3S cluster is provisioning for the local cluster")
            k3s_kubeconfig_path = \
                create_multiple_control_cluster()
            cmd = "cp {0} {1}".format(k3s_kubeconfig_path, kubeconfig_path)
            run_command_with_stderr(cmd)
        elif RANCHER_LOCAL_CLUSTER_TYPE == "RKE2":
            print("RKE2 cluster is provisioning for the local cluster")
            rke2_kubeconfig_path = \
                create_rke2_multiple_control_cluster("rke2",
                                                     RANCHER_RKE2_VERSION)
            cmd = "cp {0} {1}".format(rke2_kubeconfig_path, kubeconfig_path)
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
        elif RANCHER_LOCAL_CLUSTER_TYPE == "AKS":
            create_aks_cluster()
            install_aks_ingress()
            extra_settings.append(
                "--set ingress."
                "extraAnnotations.\"kubernetes\\.io/ingress\\.class\"=nginx"
            )
        elif RANCHER_LOCAL_CLUSTER_TYPE == "GKE":
            create_gke_cluster()
            install_gke_ingress()
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

    print_kubeconfig(kubeconfig_path)
    if (RANCHER_HA_HARDENED and RANCHER_LOCAL_CLUSTER_TYPE == "RKE") and RANCHER_HA_KUBECONFIG == "" and RANCHER_HA_HOSTNAME=="":
        prepare_hardened_cluster(profile, kubeconfig_path)
    if RANCHER_LOCAL_CLUSTER_TYPE == "RKE":
        check_rke_ingress_rollout()
    elif RANCHER_LOCAL_CLUSTER_TYPE in ["K3S", "RKE2"]:
        print("Skipping ingress rollout check for k3s and rke2 clusters")
    else:
        check_ingress_rollout()
    if cm_install:
        install_cert_manager()
    add_repo_create_namespace()
    # Here we use helm to install the Rancher chart
    install_rancher(extra_settings=extra_settings)
    set_route53_with_ingress()
    wait_for_status_code(url=RANCHER_SERVER_URL + "/v3", expected_code=401)
    auth_url = \
        RANCHER_SERVER_URL + "/v3-public/localproviders/local?action=login"
    wait_for_status_code(url=auth_url, expected_code=200)
    admin_client = set_url_and_password(RANCHER_SERVER_URL)
    cluster = get_cluster_by_name(admin_client, "local")
    validate_cluster_state(admin_client, cluster, False)
    print("Local HA Rancher cluster created successfully! "
          "Access the UI via:\n{}".format(RANCHER_SERVER_URL))
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
                "{0}/cert-manager.crds.yaml".format(CERT_MANAGER_VERSION)
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
                            "--version {0}".format(CERT_MANAGER_VERSION))
    time.sleep(120)


def install_eks_ingress():
    run_command_with_stderr(export_cmd + " && kubectl apply -f " +
                            DATA_SUBDIR + "/eks_nlb.yml")


def set_route53_with_ingress():
    ingress_address = None
    if RANCHER_LOCAL_CLUSTER_TYPE == "EKS":
        kubectl_ingress = "kubectl get ingress -n cattle-system -o " \
                          "jsonpath=\"" \
                          "{.items[0].status.loadBalancer.ingress[0].hostname}\""
        time.sleep(10)
        ingress_address = run_command_with_stderr(export_cmd
                                                  + " && " +
                                                  kubectl_ingress).decode()
        AmazonWebServices().upsert_route_53_record_cname(RANCHER_HA_HOSTNAME,
                                                         ingress_address)

    elif RANCHER_LOCAL_CLUSTER_TYPE == "AKS":
        kubectl_ingress = "kubectl get svc -n ingress-nginx " \
                          "ingress-nginx-controller -o " \
                          "jsonpath=\"" \
                          "{.status.loadBalancer.ingress[0].ip}\""
        time.sleep(10)
        ingress_address = run_command_with_stderr(export_cmd
                                                  + " && " +
                                                  kubectl_ingress).decode()

        AmazonWebServices().upsert_route_53_record_cname(RANCHER_HA_HOSTNAME,
                                                         ingress_address,
                                                         record_type='A')
    elif RANCHER_LOCAL_CLUSTER_TYPE == "GKE":
        kubectl_ingress = "kubectl get svc -n ingress-nginx " \
                          "ingress-nginx-controller -o " \
                          "jsonpath=\"" \
                          "{.status.loadBalancer.ingress[0].ip}\""
        time.sleep(10)
        ingress_address = run_command_with_stderr(export_cmd
                                                  + " && " +
                                                  kubectl_ingress).decode()

        AmazonWebServices().upsert_route_53_record_cname(RANCHER_HA_HOSTNAME,
                                                         ingress_address,
                                                         record_type='A')
    elif RANCHER_LOCAL_CLUSTER_TYPE in ["RKE", "K3S", "RKE2"]:
        return
    else:
        pytest.fail("Wrong RANCHER_LOCAL_CLUSTER_TYPE: {}"
                    .format(RANCHER_LOCAL_CLUSTER_TYPE))

    print("INGRESS ADDRESS:")
    print(ingress_address)
    time.sleep(60)


def add_repo_create_namespace(repo=RANCHER_HELM_REPO, url=RANCHER_HELM_URL):
    repo_name = "rancher-" + repo
    repo_url = url + repo

    run_command_with_stderr("helm_v3 repo add " + repo_name + " " + repo_url)
    run_command_with_stderr("helm_v3 repo update")
    run_command_with_stderr(export_cmd + " && " +
                            "kubectl create namespace cattle-system")


def install_rancher(type=RANCHER_HA_CERT_OPTION, repo=RANCHER_HELM_REPO,
                    upgrade=False, extra_settings=[]):
    operation = "install"

    if upgrade:
        operation = "upgrade"

    helm_rancher_cmd = \
        export_cmd + " && helm_v3 " + operation + " rancher " + \
        "rancher-" + repo + "/rancher " + \
        "--version " + RANCHER_CHART_VERSION + " " + \
        "--namespace cattle-system " + \
        "--set hostname=" + RANCHER_HA_HOSTNAME

    if version.parse(RANCHER_CHART_VERSION) > version.parse("2.7.1"):
        helm_rancher_cmd = helm_rancher_cmd + " --set global.cattle.psp.enabled=" + str(RANCHER_PSP_ENABLED).lower()

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

    if RANCHER_HELM_EXTRA_SETTINGS:
        extra_settings.append(RANCHER_HELM_EXTRA_SETTINGS)

    if extra_settings:
        for setting in extra_settings:
            helm_rancher_cmd = helm_rancher_cmd + " " + setting

    run_command_with_stderr(helm_rancher_cmd)
    time.sleep(120)

    # set trace logging
    set_trace_cmd = "kubectl -n cattle-system get pods -l app=rancher " + \
        "--no-headers -o custom-columns=name:.metadata.name | " + \
        "while read rancherpod; do kubectl -n cattle-system " + \
        "exec $rancherpod -c rancher -- loglevel --set trace; done"
    run_command_with_stderr(set_trace_cmd)


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


def set_url_and_password(rancher_url, server_url=None, version=""):
    admin_token = set_url_password_token(rancher_url, server_url, version=version)
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


def check_ingress_rollout():
    run_command_with_stderr(
        export_cmd + " && " +
        "kubectl -n ingress-nginx rollout status deploy/ingress-nginx-controller")
    run_command_with_stderr(
        export_cmd + " && " +
        "kubectl -n ingress-nginx wait --for=condition=complete job/ingress-nginx-admission-create")
    run_command_with_stderr(
        export_cmd + " && " +
        "kubectl -n ingress-nginx wait --for=condition=complete job/ingress-nginx-admission-patch")


def check_rke_ingress_rollout():
    rke_version = run_command_with_stderr('rke -v | cut -d " " -f 3')
    rke_version = ''.join(rke_version.decode('utf-8').split())
    print("RKE VERSION: " + rke_version)
    k8s_version = run_command_with_stderr(export_cmd + " && " +
                                          'kubectl version --short | grep -i server | cut -d " " -f 3')
    k8s_version = ''.join(k8s_version.decode('utf-8').split())
    print("KUBERNETES VERSION: " + k8s_version)
    if packaging.version.parse(rke_version) > packaging.version.parse("v1.2"):
        if packaging.version.parse(k8s_version) >= packaging.version.parse("v1.21"):
            run_command_with_stderr(
                export_cmd + " && " +
                "kubectl -n ingress-nginx rollout status ds/nginx-ingress-controller")
            run_command_with_stderr(
                export_cmd + " && " +
                "kubectl -n ingress-nginx wait --for=condition=complete job/ingress-nginx-admission-create")
            run_command_with_stderr(
                export_cmd + " && " +
                "kubectl -n ingress-nginx wait --for=condition=complete job/ingress-nginx-admission-patch")


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
    rkeconfig = rkeconfig.replace("$KUBERNETES_VERSION", KUBERNETES_VERSION)

    if RANCHER_HA_HARDENED:
        rkeconfig_hardened = readDataFile(DATA_SUBDIR, "hardened-cluster.yml")
        rkeconfig += "\n"
        rkeconfig += rkeconfig_hardened
    print("cluster-ha-filled.yml: \n" + rkeconfig + "\n")

    clusterfilepath = DATA_SUBDIR + "/" + "cluster-ha-filled.yml"

    f = open(clusterfilepath, "w")
    f.write(rkeconfig)
    f.close()
    return clusterfilepath


def create_aks_cluster():
    tf_dir = DATA_SUBDIR + "/" + "terraform/aks"
    aks_k8_s_version = os.environ.get('RANCHER_AKS_K8S_VERSION', '')
    aks_location = os.environ.get('RANCHER_AKS_LOCATION', '')
    client_id = os.environ.get('AZURE_CLIENT_ID', '')
    client_secret = os.environ.get('AZURE_CLIENT_SECRET', '')

    tf = Terraform(working_dir=tf_dir,
                   variables={'kubernetes_version': aks_k8_s_version,
                              'location': aks_location,
                              'cluster_name': resource_prefix})

    tffile = tf_dir + "/aks.tf"

    config = readDataFile(DATA_SUBDIR, tffile)
    config = config.replace('id_placeholder', AZURE_CLIENT_ID)
    config = config.replace('secret_placeholder', AZURE_CLIENT_SECRET)

    f = open(tffile, "w")
    f.write(config)
    f.close()

    print("Creating cluster")
    tf.init()
    print(tf.plan(out="aks_plan_server.out"))
    print("\n\n")
    print(tf.apply("--auto-approve"))
    print("\n\n")
    out_string = tf.output("kube_config", full_value=True)
    with open(kubeconfig_path, "w") as kubefile:
        kubefile.write(out_string)


def create_gke_cluster():
    tf_dir = DATA_SUBDIR + "/" + "terraform/gke"
    credentials = os.environ.get('RANCHER_GKE_CREDENTIAL', '')
    gke_k8_s_version = os.environ.get('RANCHER_GKE_K8S_VERSION', '')

    tf = Terraform(working_dir=tf_dir,
                   variables={'kubernetes_version': gke_k8_s_version,
                              'credentials': credentials,
                              'cluster_name': resource_prefix})

    print("Creating cluster")
    tf.init()
    print(tf.plan(out="gks_plan_server.out"))
    print("\n\n")
    print(tf.apply("--auto-approve"))
    print("\n\n")
    out_string = tf.output("kube_config", full_value=True)
    print("GKE KUBECONFIG")
    print("\n\n")
    print(out_string)
    print("\n\n")
    with open(kubeconfig_path, "w") as kubefile:
        kubefile.write(out_string)


def install_aks_ingress():
    run_command_with_stderr(export_cmd + " && kubectl apply -f " +
                            DATA_SUBDIR + "/aks_nlb.yml")


def install_gke_ingress():
    run_command_with_stderr(export_cmd + " && kubectl apply -f " +
                            DATA_SUBDIR + "/gke_nlb.yml")


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
