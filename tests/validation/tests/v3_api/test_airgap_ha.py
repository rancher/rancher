from base64 import b64decode
import os
import pytest
import time

from .common import (
    ADMIN_PASSWORD,
    AmazonWebServices,
    run_command, wait_for_status_code
)
from .test_custom_host_reg import RANCHER_SERVER_VERSION

from .test_create_ha import (
    set_url_and_password,
    RANCHER_HA_CERT_OPTION,
    RANCHER_VALID_TLS_CERT,
    RANCHER_VALID_TLS_KEY,
    KUBERNETES_VERSION
)
from lib.aws import AWS_USER

AWS_AMI = os.environ.get("AWS_AMI", "ami-012fd49f6b0c404c7")
DOCKER_COMPOSE_VERSION = os.environ.get("DOCKER_COMPOSE_VERSION", "1.24.1")
RKE_VERSION = os.environ.get("RKE_VERSION", "v1.3.11")

# airgap variables
RANCHER_HELM_EXTRA_SETTINGS = os.environ.get("RANCHER_HELM_EXTRA_SETTINGS")
NUMBER_OF_INSTANCES = int(os.environ.get("RANCHER_AIRGAP_INSTANCE_COUNT", "3"))
HOST_NAME = os.environ.get('RANCHER_HOST_NAME', "testsa")
AG_HOST_NAME = HOST_NAME
RANCHER_AG_INTERNAL_HOSTNAME = AG_HOST_NAME + "-internal.qa.rancher.space"
RANCHER_AG_HOSTNAME = AG_HOST_NAME + ".qa.rancher.space"
RANCHER_HELM_REPO = os.environ.get("RANCHER_HELM_REPO", "latest")
RANCHER_HELM_URL = os.environ.get("RANCHER_HELM_URL", "https://releases.rancher.com/server-charts/")

RESOURCE_DIR = os.path.join(os.path.dirname(os.path.realpath(__file__)),
                            'resource')
SSH_KEY_DIR = os.path.join(os.path.dirname(os.path.realpath(__file__)),
                           '.ssh')

# bastion/ registry variables
PRIVATE_REGISTRY_USERNAME = os.environ.get("RANCHER_BASTION_USERNAME")
PRIVATE_REGISTRY_PASSWORD = \
    os.environ.get("RANCHER_BASTION_PASSWORD", ADMIN_PASSWORD)
RANCHER_EXTERNAL_BASTION = os.environ.get("RANCHER_EXTERNAL_BASTION", "")
RANCHER_EXTERNAL_HOST_NAME = os.environ.get(
    "RANCHER_EXTERNAL_HOST_NAME", AG_HOST_NAME)
REG_HOST_NAME = RANCHER_EXTERNAL_HOST_NAME + "-registry" + ".qa.rancher.space"
REGISTRY_HOSTNAME = os.environ.get("RANCHER_BASTION_REGISTRY", REG_HOST_NAME)
INSTALL_DEPENDENCIES_FLAG=os.environ.get("INSTALL_DEPENDENCIES_FLAG", "True")


def test_deploy_airgap_ha_rancher(check_hostname_length):
    # check for user entered bastion/registry
    if len(RANCHER_EXTERNAL_BASTION) > 5:
        bastion_node = AmazonWebServices().get_node(
            RANCHER_EXTERNAL_BASTION,
            ssh_access=True)
        print("using external node for private registry", REGISTRY_HOSTNAME)
    else:
        bastion_node = deploy_bastion_server()
        add_rancher_images_to_private_registry(
                                        bastion_node,
                                        noauth_reg_name=REGISTRY_HOSTNAME)
        print("using new node for private registry")

    kube_config = 'export KUBECONFIG="kube_config_config.yaml && '
    # check for registry, images, private IP
    assert len(bastion_node.private_ip_address) > 5, \
        "the bastion node does not have a private IP"
    assert bastion_node.execute_command("docker pull {}/{}:{}".format(
        REGISTRY_HOSTNAME,
        "rancher/rancher-agent",
        RANCHER_SERVER_VERSION))[1].find("not found") < 0, \
        "registry is missing rancher-agent image"
    
    # checks for and installs dependencies if missing from bastion node
    if INSTALL_DEPENDENCIES_FLAG.lower() != "false":
        if bastion_node.execute_command(
        "rke --version")[0].find("version") == -1:
            install_rke(bastion_node)
        if bastion_node.execute_command(
        '/snap/bin/helm')[0].find("Flags:") == -1:
            install_helm(bastion_node)
        if bastion_node.execute_command(
        kube_config+"/snap/bin/kubectl version")[0].find("Version") == -1:
            install_kubectl(bastion_node)

    # check kubectl, helm, rke
    assert bastion_node.execute_command(
        "rke --version")[1].find("not found") < 0, \
        "registry does not have RKE installed"
    assert bastion_node.execute_command(
        "/snap/bin/helm version")[1].find("not found") < 0, \
        "registry does not have helm installed"
    assert bastion_node.execute_command(
        '/snap/bin/helm')[0].find("Flags:") > -1, \
        "helm is not installed correctly"
    assert bastion_node.execute_command(
        kube_config+"/snap/bin/kubectl version")[1].find("not found") < 0, \
        "registry does not have kubectl installed"

    ag_nodes = setup_airgap_rancher(bastion_node)
    public_dns = create_nlb_and_add_targets(ag_nodes)
    print(
        "\nConnect to bastion node with:\nssh -i {}.pem {}@{}\n"
        "Connect to rancher node by connecting to bastion, then run:\n"
        "ssh -i {}.pem {}@{}\n\nOpen the Rancher UI with: https://{}\n"
        "** IMPORTANT: SET THE RANCHER SERVER URL UPON INITIAL LOGIN TO: {} **"
        "\nWhen creating a cluster, enable private registry with below"
        " settings:\nPrivate Registry URL: {}\nPrivate Registry is not"
        "using basic auth (no login required) \n".format(
            bastion_node.ssh_key_name, AWS_USER, bastion_node.host_name,
            bastion_node.ssh_key_name, AWS_USER,
            ag_nodes[0].private_ip_address,
            public_dns, RANCHER_AG_INTERNAL_HOSTNAME,
            REGISTRY_HOSTNAME))
    time.sleep(180)
    setup_rancher_server(bastion_node)


def setup_rancher_server(bastion_node):
    base_url = "https://" + RANCHER_AG_HOSTNAME
    wait_for_status_code(url=base_url + "/v3", expected_code=401)
    auth_url = base_url + "/v3-public/localproviders/local?action=login"
    wait_for_status_code(url=auth_url, expected_code=200)
    get_bootstrap_passwd = "export KUBECONFIG=~/kube_config_config.yaml && " \
    "/snap/bin/kubectl get secret --namespace cattle-system bootstrap-secret -o " \
    """go-template='{{.data.bootstrapPassword|base64decode}}{{"\\n"}}'"""
    bootstrap_passwd = bastion_node.execute_command(get_bootstrap_passwd)[0]
    print("bootstrap password:", bootstrap_passwd.replace(" ", ""))
    # currently can't set password in airgap setup without reworking below function
    set_url_and_password(base_url, "https://" + RANCHER_AG_INTERNAL_HOSTNAME)


def deploy_bastion_server():
    node_name = AG_HOST_NAME + "-bastion"
    node_name = REG_HOST_NAME
    # Create Registry Server in AWS
    registry_node = AmazonWebServices().create_node(node_name, for_bastion=True)
    setup_ssh_key(registry_node)
    # update a record if it exists
    AmazonWebServices().upsert_route_53_record_a(
            REGISTRY_HOSTNAME, registry_node.private_ip_address)
    # Get resources for private registry
    get_registry_resources(registry_node)
    # use valid certs
    overwrite_tls_certs(registry_node)
    # remove auth from nginx.conf
    overwrite_word_in_file_command = \
        "sed -i -e 's/{}/{}/g' ~/basic-registry/nginx_config/nginx.conf"
    registry_node.execute_command(overwrite_word_in_file_command.format(
        "auth_basic", "#auth_basic"))
    registry_node.execute_command(overwrite_word_in_file_command.format(
        "add_header", "#add_header"))
    # Run private registry
    run_docker_registry(registry_node)
    print("Registry Server Details:\nNAME: {}\nHOST NAME: {}\n"
          "INSTANCE ID: {}\n".format(node_name, registry_node.host_name,
                                     registry_node.provider_node_id))
    return registry_node


def overwrite_tls_certs(external_node):
    overwrite_server_name_command = \
        "sed -i -e '0,/_;/s//{};/' basic-registry/nginx_config/nginx.conf && "\
        'echo \"{}\" >> ~/basic-registry/nginx_config/domain.crt && ' \
        'echo \"{}\" >> ~/basic-registry/nginx_config/domain.key'
    external_node.execute_command(overwrite_server_name_command.format(
                REGISTRY_HOSTNAME,
                b64decode(RANCHER_VALID_TLS_CERT).decode("utf-8"),
                b64decode(RANCHER_VALID_TLS_KEY).decode("utf-8")))


def get_registry_resources(external_node):
    get_resources_command = \
        'scp -q -i {}/{}.pem -o StrictHostKeyChecking=no ' \
        '-o UserKnownHostsFile=/dev/null -r {}/airgap/basic-registry ' \
        '{}@{}:~/basic-registry'.format(
            SSH_KEY_DIR, external_node.ssh_key_name, RESOURCE_DIR,
            AWS_USER, external_node.host_name)
    run_command(get_resources_command, log_out=False)


def install_rke(external_node):
    docker_compose_command = \
        'cd ~/ && sudo wget ' \
        'https://github.com/rancher/rke/releases/download/{}' \
        '/rke_linux-amd64 && ' \
        'sudo mv rke_linux-amd64 rke && ' \
        'sudo chmod +x rke && ' \
        'sudo mv rke /usr/local/bin'.format(RKE_VERSION)
    return external_node.execute_command(docker_compose_command)


def install_helm(external_node):
    return external_node.execute_command(
        'sudo snap install helm --classic')


def install_kubectl(external_node):
    return external_node.execute_command(
        'sudo snap install kubectl --classic')


def run_docker_registry(external_node):
    docker_compose_command = \
        'cd ~/basic-registry && ' \
        'sudo curl -L "https://github.com/docker/compose/releases/' \
        'download/{}/docker-compose-$(uname -s)-$(uname -m)" ' \
        '-o /usr/local/bin/docker-compose && ' \
        'sudo chmod +x /usr/local/bin/docker-compose && ' \
        'sudo docker-compose up -d'.format(DOCKER_COMPOSE_VERSION)
    external_node.execute_command(docker_compose_command)
    time.sleep(5)


def add_rancher_images_to_private_registry(
        bastion_node, push_images=True, noauth_reg_name=""):
    get_images_command = \
        'wget -O rancher-images.txt https://github.com/rancher/rancher/' \
        'releases/download/{0}/rancher-images.txt && ' \
        'wget -O rancher-save-images.sh https://github.com/rancher/rancher/' \
        'releases/download/{0}/rancher-save-images.sh && ' \
        'wget -O rancher-load-images.sh https://github.com/rancher/rancher/' \
        'releases/download/{0}/rancher-load-images.sh'.format(
            RANCHER_SERVER_VERSION)
    bastion_node.execute_command(get_images_command)

    # comment out the "docker save" and "docker load" lines to save time
    edit_save_and_load_command = \
        "sudo sed -i -e 's/docker save /# docker/g' rancher-save-images.sh && " \
        "sudo sed -i -e 's/docker load /# docker/g' rancher-load-images.sh && " \
        "chmod +x rancher-save-images.sh && chmod +x rancher-load-images.sh"
    bastion_node.execute_command(edit_save_and_load_command)

    save_images_command = \
        "./rancher-save-images.sh --image-list ./rancher-images.txt"
    save_res = bastion_node.execute_command(save_images_command)

    if push_images:
        if len(noauth_reg_name) == 0:
            load_images_command = \
                "docker login {} -u {} -p {} && " \
                "./rancher-load-images.sh --image-list ./rancher-images.txt " \
                "--registry {}".format(
                    bastion_node.host_name, PRIVATE_REGISTRY_USERNAME,
                    PRIVATE_REGISTRY_PASSWORD, bastion_node.host_name)
            load_res = bastion_node.execute_command(load_images_command)
        else:
            load_images_command = \
                "./rancher-load-images.sh --image-list ./rancher-images.txt " \
                "--registry {}".format(noauth_reg_name)
            load_res = bastion_node.execute_command(load_images_command)
        print(load_res)
    else:
        load_res = None

    return save_res, load_res


def prepare_airgap_node(bastion_node, number_of_nodes):
    node_name = AG_HOST_NAME + "-agha"
    # Create Airgap Node in AWS
    ag_nodes = AmazonWebServices().create_multiple_nodes(
        number_of_nodes, node_name, ami=AWS_AMI, public_ip=False)

    for num, ag_node in enumerate(ag_nodes):
        # Update docker for the user in node
        ag_node_update_docker = \
            'ssh -i "{}.pem" -o StrictHostKeyChecking=no {}@{} ' \
            '"sudo usermod -aG docker {}"'.format(
                bastion_node.ssh_key_name, AWS_USER,
                ag_node.private_ip_address, AWS_USER)
        bastion_node.execute_command(ag_node_update_docker)
        if PRIVATE_REGISTRY_USERNAME is not None:
            # assuming that we setup the auth-enabled registry from the other
            # airgap job, which uses self signed certs on the registry. We 
            # now need to login and move the certs to each downstream node. 
            ss_certs_command = \
            'scp -i "{}.pem" ~/certs/ca.pem {}@{}:/home/{}/ca.pem && ' \
            'ssh -i "{}.pem" -o StrictHostKeyChecking=no {}@{} ' \
            '"sudo mkdir -p /etc/docker/certs.d/{} && ' \
            'sudo cp ~/ca.pem /etc/docker/certs.d/{}/ca.crt && ' \
            'sudo service docker restart"'.format(
                bastion_node.ssh_key_name, AWS_USER,
                ag_node.private_ip_address, AWS_USER,
                bastion_node.ssh_key_name, AWS_USER,
                ag_node.private_ip_address, bastion_node.host_name, 
                bastion_node.host_name)
            bastion_node.execute_command(ss_certs_command)
    return ag_nodes


def setup_airgap_rancher(bastion_node, number_of_nodes=NUMBER_OF_INSTANCES):
    # remove old setup files, if any
    bastion_node.execute_command("rm ingress* config.* kube_config_* tls*")
    # need an odd number of nodes for HA setup
    if number_of_nodes % 2 == 0:
        number_of_nodes += 1
        print("automatically setting number of nodes to odd number: ",
              number_of_nodes)
    if number_of_nodes < 3:
        number_of_nodes = 3
        print("automatically setting number of nodes to at least ",
              number_of_nodes)
    ag_nodes = prepare_airgap_node(bastion_node, number_of_nodes)
    # create RKE config based on number of nodes
    rke_template_node = ""
    if KUBERNETES_VERSION:
        rke_template_node = f'kubernetes_version: {KUBERNETES_VERSION}\n'
    rke_template_node += "nodes:"
    with open(RESOURCE_DIR+'/airgap/config_yamls/config_body.yaml') as f2:
        rke_template_node_body = f2.read()

    for x in range(number_of_nodes):
        rke_template_node += '\n    - address: ' \
            + ag_nodes[x].private_ip_address + "\n" + rke_template_node_body

    with open(RESOURCE_DIR+'/airgap/config_yamls/config_end.yaml') as f3:
        rke_template_node_end = f3.read()
    registry_credentials=REGISTRY_HOSTNAME
    if PRIVATE_REGISTRY_USERNAME is not None:
        registry_credentials+='\n  user: {}\n' \
            '  password: {}'.format(
                PRIVATE_REGISTRY_USERNAME, 
                PRIVATE_REGISTRY_PASSWORD)
    rke_template_node +=\
        rke_template_node_end.format(registry_credentials)
    # write config, and run rke up
    print(f'RKE Template:\n{rke_template_node}')
    bastion_node.execute_command(
        "echo '{}' > config.yaml".format(rke_template_node))
    bastion_node.execute_command(
        'cd ~/ && /usr/local/bin/rke up --config config.yaml && sleep 240')
    assert bastion_node.execute_command(
        "cd ~ && cat kube_config_config.yaml")[0].find(".") > -1, \
        "rke up failed, config file likely incorrect"
    # setup helm and rancher template
    assert bastion_node.execute_command(
        '/snap/bin/helm')[0].find("Flags:") > -1, \
        "helm is not installed correctly"
    
    repo_name = "rancher-" + RANCHER_HELM_REPO
    repo_url = RANCHER_HELM_URL + RANCHER_HELM_REPO
    bastion_node.execute_command(
        '/snap/bin/helm repo add ' + repo_name + ' ' + repo_url)
    assert bastion_node.execute_command(
        "/snap/bin/helm repo list")[0].find("rancher") > -1, \
        "helm was unable to add rancher repo"
    bastion_node.execute_command(
        "/snap/bin/helm fetch " + repo_name + "/rancher")
    bastion_node.execute_command(
        "/snap/bin/helm fetch {}/rancher --version={}".format(
            repo_name,
            RANCHER_SERVER_VERSION))
    # remove `v` from the version tag for helm formatting
    new_rancher_version = RANCHER_SERVER_VERSION
    if RANCHER_SERVER_VERSION[0] == 'v':
        new_rancher_version = RANCHER_SERVER_VERSION[1:]
    helm_template = \
        "cd ~/ && /snap/bin/helm template rancher ./rancher-{3}.tgz " \
        "--output-dir . --namespace cattle-system --set hostname={1} " \
        "--set rancherImage={2}/rancher/rancher " \
        "--set systemDefaultRegistry={2} " \
        "--set useBundledSystemChart=true --set ingress.tls.source=secret " \
        "--set rancherImageTag={0} --no-hooks --validate".format(
            RANCHER_SERVER_VERSION,
            RANCHER_AG_INTERNAL_HOSTNAME,
            REGISTRY_HOSTNAME,
            new_rancher_version)
    extra_settings = []
    if RANCHER_HELM_EXTRA_SETTINGS:
        extra_settings.append(RANCHER_HELM_EXTRA_SETTINGS)
    if extra_settings:
        for setting in extra_settings:
            helm_template = helm_template + " " + setting

    print("Executing helm install: \n", helm_template)
    print("\nUsing the following kube config: \n")
    print(bastion_node.execute_command("cat ~/kube_config_config.yaml")[0])
    bastion_node.execute_command(helm_template)
    kube_config = 'export KUBECONFIG=~/kube_config_config.yaml && '
    # install rancher
    bastion_node.execute_command(
        kube_config+"/snap/bin/kubectl create namespace cattle-system && 10")
    create_tls_secrets(bastion_node)
    bootstrap_password=""
    if RANCHER_HELM_EXTRA_SETTINGS:
        for setting in RANCHER_HELM_EXTRA_SETTINGS.split("--set "):
            settings_keypair = RANCHER_HELM_EXTRA_SETTINGS.split("=")
            if "bootstrapPassword" in settings_keypair[0]:
                bootstrap_password = settings_keypair[1]
                print ("bootstrap password: ", bootstrap_password)
                execution = bastion_node.execute_command(
                    kube_config+
                    "/snap/bin/kubectl create secret" +
                    " generic -n cattle-system " +
                    "bootstrap-secret --from-literal=bootstrapPassword=" +
                    bootstrap_password + " && 10")
                print(execution[0], execution[1])
                assert execution[0].find("create") > -1, \
                    "bootstrap password secret not created"
                break
    
    bastion_node.execute_command(
        kube_config + "/snap/bin/kubectl -n cattle-system apply -R -f"
        + " ./rancher && sleep 60")
    print("applying rancher template: \n",
          "/snap/bin/kubectl -n cattle-system apply -R -f",
          " ./rancher && sleep 60")
    assert bastion_node.execute_command(
        kube_config+"/snap/bin/kubectl get pods -A")[0].find(
        "cattle-system") > 0, \
        "install of rancher failed. cattle-system has no pods"

    # modify ingress to include internal and external LBs
    bastion_node.execute_command(
        kube_config + "/snap/bin/kubectl get ingress rancher "
        + " -n cattle-system -o yaml > ingress.yaml")
    ingress_yaml = bastion_node.execute_command("cat ingress.yaml")[0]
    beginning_ingress = ingress_yaml[:ingress_yaml.find("  tls:")]
    beginning_ingress += "  - host: {}".format(RANCHER_AG_HOSTNAME)
    beginning_ingress += """\n    http:
      paths:
      - backend:
          service:
            name: rancher
            port:
              number: 80
        pathType: ImplementationSpecific
  tls:
  - hosts:"""
    beginning_ingress += "\n    - {}\n".format(RANCHER_AG_HOSTNAME) + \
        ingress_yaml[ingress_yaml.find("- hosts:")+9:]
    bastion_node.execute_command(
        "echo '{}' > ingress2.yaml".format(beginning_ingress))
    bastion_node.execute_command(
        kube_config+"/snap/bin/kubectl apply -f ingress2.yaml")

    assert bastion_node.execute_command(
        kube_config+"/snap/bin/kubectl get ingress -n cattle-system")[0].find(
            RANCHER_AG_HOSTNAME) > 0, \
        "Ingress modification failed. "
    print("in order to upgrade, you will need to modify the ingress to have ",
          "both hosts, public and private. Here is the modified ingress: \n",
          beginning_ingress)
    return ag_nodes


def create_nlb_and_add_targets(aws_nodes):
    # Create internet-facing nlb and grab ARN & dns name
    lb = AmazonWebServices().create_network_lb(name=AG_HOST_NAME + "-nlb")
    lb_arn = lb["LoadBalancers"][0]["LoadBalancerArn"]
    public_dns = lb["LoadBalancers"][0]["DNSName"]
    # Create internal nlb and grab ARN & dns name
    internal_lb = AmazonWebServices().create_network_lb(
        name=AG_HOST_NAME + "-internal-nlb", scheme='internal')
    internal_lb_arn = internal_lb["LoadBalancers"][0]["LoadBalancerArn"]
    internal_lb_dns = internal_lb["LoadBalancers"][0]["DNSName"]

    # Upsert the route53 record -- if it exists, update, if not, insert
    AmazonWebServices().upsert_route_53_record_cname(
        RANCHER_AG_INTERNAL_HOSTNAME, internal_lb_dns)
    if RANCHER_HA_CERT_OPTION == 'byo-valid':
        AmazonWebServices().upsert_route_53_record_cname(
            RANCHER_AG_HOSTNAME, public_dns)
        public_dns = RANCHER_AG_HOSTNAME

    # Create the target groups
    tg80 = AmazonWebServices(). \
        create_ha_target_group(80, AG_HOST_NAME + "-tg-80")
    tg443 = AmazonWebServices(). \
        create_ha_target_group(443, AG_HOST_NAME + "-tg-443")
    tg80_arn = tg80["TargetGroups"][0]["TargetGroupArn"]
    tg443_arn = tg443["TargetGroups"][0]["TargetGroupArn"]
    # Create the internal target groups
    internal_tg80 = AmazonWebServices(). \
        create_ha_target_group(80, AG_HOST_NAME + "-internal-tg-80")
    internal_tg443 = AmazonWebServices(). \
        create_ha_target_group(443, AG_HOST_NAME + "-internal-tg-443")
    internal_tg80_arn = internal_tg80["TargetGroups"][0]["TargetGroupArn"]
    internal_tg443_arn = internal_tg443["TargetGroups"][0]["TargetGroupArn"]

    # Create listeners for the load balancers, to forward to the target groups
    AmazonWebServices().create_ha_nlb_listener(
        loadBalancerARN=lb_arn, port=80, targetGroupARN=tg80_arn)
    AmazonWebServices().create_ha_nlb_listener(
        loadBalancerARN=lb_arn, port=443, targetGroupARN=tg443_arn)
    AmazonWebServices().create_ha_nlb_listener(
        loadBalancerARN=internal_lb_arn, port=80,
        targetGroupARN=internal_tg80_arn)
    AmazonWebServices().create_ha_nlb_listener(
        loadBalancerARN=internal_lb_arn, port=443,
        targetGroupARN=internal_tg443_arn)

    targets = []

    for aws_node in aws_nodes:
        targets.append(aws_node.provider_node_id)

    # Register the nodes to the internet-facing targets
    targets_list = [dict(Id=target_id, Port=80) for target_id in targets]
    AmazonWebServices().register_targets(targets_list, tg80_arn)
    targets_list = [dict(Id=target_id, Port=443) for target_id in targets]
    AmazonWebServices().register_targets(targets_list, tg443_arn)
    # Wait up to approx. 5 minutes for targets to begin health checks
    for i in range(300):
        health80 = AmazonWebServices().describe_target_health(
            tg80_arn)['TargetHealthDescriptions'][0]['TargetHealth']['State']
        health443 = AmazonWebServices().describe_target_health(
            tg443_arn)['TargetHealthDescriptions'][0]['TargetHealth']['State']
        if health80 in ['initial', 'healthy'] \
                and health443 in ['initial', 'healthy']:
            break
        time.sleep(1)

    # Register the nodes to the internal targets
    targets_list = [dict(Id=target_id, Port=80) for target_id in targets]
    AmazonWebServices().register_targets(targets_list, internal_tg80_arn)
    targets_list = [dict(Id=target_id, Port=443) for target_id in targets]
    AmazonWebServices().register_targets(targets_list, internal_tg443_arn)
    # Wait up to approx. 5 minutes for targets to begin health checks
    for i in range(300):
        try:
            health80 = AmazonWebServices().describe_target_health(
                internal_tg80_arn)[
                'TargetHealthDescriptions'][0]['TargetHealth']['State']
            health443 = AmazonWebServices().describe_target_health(
                internal_tg443_arn)[
                'TargetHealthDescriptions'][0]['TargetHealth']['State']
            if health80 in ['initial', 'healthy'] \
                    and health443 in ['initial', 'healthy']:
                break
        except Exception:
            print("Target group healthchecks unavailable...")
        time.sleep(1)

    return public_dns


def setup_ssh_key(bastion_node):
    # Copy SSH Key to Bastion and local dir and give it proper permissions
    write_key_command = "cat <<EOT >> {}.pem\n{}\nEOT".format(
        bastion_node.ssh_key_name, bastion_node.ssh_key)
    bastion_node.execute_command(write_key_command)
    local_write_key_command = \
        "mkdir -p {} && cat <<EOT >> {}/{}.pem\n{}\nEOT".format(
            SSH_KEY_DIR, SSH_KEY_DIR,
            bastion_node.ssh_key_name, bastion_node.ssh_key)
    run_command(local_write_key_command, log_out=False)

    set_key_permissions_command = "chmod 400 {}.pem".format(
        bastion_node.ssh_key_name)
    bastion_node.execute_command(set_key_permissions_command)
    local_set_key_permissions_command = "chmod 400 {}/{}.pem".format(
        SSH_KEY_DIR, bastion_node.ssh_key_name)
    run_command(local_set_key_permissions_command, log_out=False)


def create_tls_secrets(bastion_node):
    # currently hard coded for byo-valid certs
    cert_path = "~/tls.crt"
    key_path = "~/tls.key"

    bastion_write_encoded_certs(cert_path,
                                RANCHER_VALID_TLS_CERT,
                                bastion_node)
    bastion_write_encoded_certs(key_path,
                                RANCHER_VALID_TLS_KEY,
                                bastion_node)
    kube_config = 'cd ~/ && export KUBECONFIG=kube_config_config.yaml '
    tls_command = kube_config+" && /snap/bin/kubectl -n cattle-system " \
                              "create secret tls tls-rancher-ingress " \
                              "--cert=tls.crt" + " --key=tls.key"

    tls_output = bastion_node.execute_command(tls_command)
    bastion_node.execute_command("sleep 2")
    print(tls_output)

    assert tls_output[0].find("secret/tls-rancher-ingress created") > -1


def bastion_write_encoded_certs(path, contents, bastion_node):
    bastion_node.execute_command(
        "echo '{}' > {}".format(b64decode(contents).decode("utf-8"), path))
    assert bastion_node.execute_command(
        "cat {}".format(path))[1].find("not found") < 0


@pytest.fixture()
def check_hostname_length():
    print("Host Name: {}".format(AG_HOST_NAME))
    assert len(AG_HOST_NAME) < 17, "Provide hostname that is 16 chars or less"
