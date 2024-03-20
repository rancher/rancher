import base64
import os
import pytest
import re
import time
from lib.aws import AWS_USER
from .common import (
    ADMIN_PASSWORD, AmazonWebServices, run_command, wait_for_status_code,
    TEST_IMAGE, TEST_IMAGE_REDIS, TEST_IMAGE_OS_BASE, readDataFile,
    DEFAULT_CLUSTER_STATE_TIMEOUT, compare_versions
)
from .test_custom_host_reg import (
    random_test_name, RANCHER_SERVER_VERSION, HOST_NAME, AGENT_REG_CMD
)
from .test_create_ha import (
    set_url_and_password,
    RANCHER_HA_CERT_OPTION, RANCHER_VALID_TLS_CERT, RANCHER_VALID_TLS_KEY
)

PRIVATE_REGISTRY_USERNAME = os.environ.get("RANCHER_BASTION_USERNAME")
PRIVATE_REGISTRY_PASSWORD = \
    os.environ.get("RANCHER_BASTION_PASSWORD", ADMIN_PASSWORD)
BASTION_ID = os.environ.get("RANCHER_BASTION_ID", "")
NUMBER_OF_INSTANCES = int(os.environ.get("RANCHER_AIRGAP_INSTANCE_COUNT", "1"))
IMAGE_LIST = os.environ.get("RANCHER_IMAGE_LIST", ",".join(
    [TEST_IMAGE, TEST_IMAGE_REDIS, TEST_IMAGE_OS_BASE])).split(",")
TARBALL_TYPE = os.environ.get("K3S_TARBALL_TYPE", "tar.gz")
ARCH = os.environ.get("K3S_ARCH", "amd64")

AG_HOST_NAME = random_test_name(HOST_NAME)
RANCHER_AG_INTERNAL_HOSTNAME = AG_HOST_NAME + "-internal.qa.rancher.space"
RANCHER_AG_HOSTNAME = AG_HOST_NAME + ".qa.rancher.space"
RESOURCE_DIR = os.path.join(os.path.dirname(os.path.realpath(__file__)),
                            'resource')
SSH_KEY_DIR = os.path.join(os.path.dirname(os.path.realpath(__file__)),
                           '.ssh')


def test_deploy_bastion():
    node = deploy_bastion_server()
    assert node.public_ip_address is not None


def test_deploy_airgap_rancher(check_hostname_length):
    bastion_node = deploy_bastion_server()
    save_res, load_res = add_rancher_images_to_private_registry(bastion_node)
    assert "Image pull success: rancher/rancher:{}".format(
        RANCHER_SERVER_VERSION) in save_res[0]
    assert "The push refers to repository [{}/rancher/rancher]".format(
        bastion_node.host_name) in load_res[0]
    ag_node = deploy_airgap_rancher(bastion_node)
    public_dns = create_nlb_and_add_targets([ag_node])
    print(
        "\nConnect to bastion node with:\nssh -i {}.pem {}@{}\n"
        "Connect to rancher node by connecting to bastion, then run:\n"
        "ssh -i {}.pem {}@{}\n\nOpen the Rancher UI with: https://{}\n"
        "** IMPORTANT: SET THE RANCHER SERVER URL UPON INITIAL LOGIN TO: {} **"
        "\nWhen creating a cluster, enable private registry with below"
        " settings:\nPrivate Registry URL: {}\nPrivate Registry User: {}\n"
        "Private Registry Password: (default admin password or "
        "whatever you set in RANCHER_BASTION_PASSWORD)\n".format(
            bastion_node.ssh_key_name, AWS_USER, bastion_node.host_name,
            bastion_node.ssh_key_name, AWS_USER, ag_node.private_ip_address,
            public_dns, RANCHER_AG_INTERNAL_HOSTNAME,
            bastion_node.host_name, PRIVATE_REGISTRY_USERNAME))
    time.sleep(180)
    setup_rancher_server()


def test_prepare_airgap_nodes():
    bastion_node = get_bastion_node()
    ag_nodes = prepare_airgap_node(bastion_node, NUMBER_OF_INSTANCES)
    assert len(ag_nodes) == NUMBER_OF_INSTANCES

    print(
        '{} airgapped instance(s) created.\n'
        'Connect to these and run commands by connecting to bastion node, '
        'then running the following command (with the quotes):\n'
        'ssh -i {}.pem {}@NODE_PRIVATE_IP '
        '"docker login {} -u {} -p {} && COMMANDS"'.format(
            NUMBER_OF_INSTANCES, bastion_node.ssh_key_name, AWS_USER,
            bastion_node.host_name, PRIVATE_REGISTRY_USERNAME,
            PRIVATE_REGISTRY_PASSWORD))
    for ag_node in ag_nodes:
        assert ag_node.private_ip_address is not None
        assert ag_node.public_ip_address is None


def test_deploy_airgap_nodes():
    bastion_node = get_bastion_node()
    ag_nodes = prepare_airgap_node(bastion_node, NUMBER_OF_INSTANCES)
    assert len(ag_nodes) == NUMBER_OF_INSTANCES

    print(
        '{} airgapped instance(s) created.\n'
        'Connect to these and run commands by connecting to bastion node, '
        'then running the following command (with the quotes):\n'
        'ssh -i {}.pem {}@NODE_PRIVATE_IP '
        '"docker login {} -u {} -p {} && COMMANDS"'.format(
            NUMBER_OF_INSTANCES, bastion_node.ssh_key_name, AWS_USER,
            bastion_node.host_name, PRIVATE_REGISTRY_USERNAME,
            PRIVATE_REGISTRY_PASSWORD))
    for ag_node in ag_nodes:
        assert ag_node.private_ip_address is not None
        assert ag_node.public_ip_address is None
    results = []
    for ag_node in ag_nodes:
        deploy_result = run_command_on_airgap_node(bastion_node, ag_node,
                                                   AGENT_REG_CMD)
        results.append(deploy_result)
    for result in results:
        assert "Downloaded newer image for " in result[1]
        assert "/rancher/rancher-agent" in result[1]


def test_add_rancher_images_to_private_registry():
    bastion_node = get_bastion_node()
    save_res, load_res = add_rancher_images_to_private_registry(bastion_node)
    assert "Image pull success: rancher/rancher:{}".format(
        RANCHER_SERVER_VERSION) in save_res[0]
    assert "The push refers to repository " in load_res[0]
    assert "/rancher/rancher]" in load_res[0]


def test_add_images_to_private_registry():
    bastion_node = get_bastion_node()
    failures = add_images_to_private_registry(bastion_node, IMAGE_LIST)
    assert failures == [], "Failed to add images: {}".format(failures)


def test_deploy_private_registry_without_image_push():
    bastion_node = deploy_bastion_server()
    save_res, load_res = add_rancher_images_to_private_registry(
        bastion_node, push_images=False)
    assert "Image pull success: rancher/rancher:{}".format(
        RANCHER_SERVER_VERSION) in save_res[0]
    assert load_res is None


def setup_rancher_server():
    base_url = "https://" + RANCHER_AG_HOSTNAME
    wait_for_status_code(url=base_url + "/v3", expected_code=401)
    auth_url = base_url + "/v3-public/localproviders/local?action=login"
    wait_for_status_code(url=auth_url, expected_code=200)
    set_url_and_password(base_url, "https://" + RANCHER_AG_INTERNAL_HOSTNAME, version=RANCHER_SERVER_VERSION)


def deploy_noauth_bastion_server():
    node_name = AG_HOST_NAME + "-noauthbastion"
    # Create Bastion Server in AWS
    bastion_node = AmazonWebServices().create_node(node_name, for_bastion=True)
    setup_ssh_key(bastion_node)

    # Generate self signed certs
    generate_certs_command = \
        'mkdir -p certs && sudo openssl req -newkey rsa:4096 -nodes -sha256 ' \
        '-keyout certs/domain.key -x509 -days 365 -out certs/domain.crt ' \
        '-subj "/C=US/ST=AZ/O=Rancher QA/CN={0}" ' \
        '-addext "subjectAltName = DNS:{0}"'.format(bastion_node.host_name)
    bastion_node.execute_command(generate_certs_command)

    # Ensure docker uses the certs that were generated
    update_docker_command = \
        'sudo mkdir -p /etc/docker/certs.d/{0} && ' \
        'sudo cp ~/certs/domain.crt /etc/docker/certs.d/{0}/ca.crt && ' \
        'sudo service docker restart'.format(bastion_node.host_name)
    bastion_node.execute_command(update_docker_command)

    # Run private registry
    run_private_registry_command = \
        'sudo docker run -d --restart=always --name registry ' \
        '-v "$(pwd)"/certs:/certs -e REGISTRY_HTTP_ADDR=0.0.0.0:443 ' \
        '-e REGISTRY_HTTP_TLS_CERTIFICATE=/certs/domain.crt ' \
        '-e REGISTRY_HTTP_TLS_KEY=/certs/domain.key -p 443:443 registry:2'
    bastion_node.execute_command(run_private_registry_command)
    time.sleep(5)

    print("Bastion Server Details:\nNAME: {}\nHOST NAME: {}\n"
          "INSTANCE ID: {}\n".format(node_name, bastion_node.host_name,
                                     bastion_node.provider_node_id))

    return bastion_node


def deploy_bastion_server():
    node_name = AG_HOST_NAME + "-bastion"
    # Create Bastion Server in AWS
    bastion_node = AmazonWebServices().create_node(node_name, for_bastion=True)
    setup_ssh_key(bastion_node)

    # Get resources for private registry and generate self signed certs
    get_resources_command = \
        'scp -q -i {}/{}.pem -o StrictHostKeyChecking=no ' \
        '-o UserKnownHostsFile=/dev/null -r {}/airgap/basic-registry ' \
        '{}@{}:~/basic-registry'.format(
            SSH_KEY_DIR, bastion_node.ssh_key_name, RESOURCE_DIR,
            AWS_USER, bastion_node.host_name)
    run_command(get_resources_command, log_out=False)

    generate_certs_command = \
        'docker run -v $PWD/certs:/certs ' \
        '-e CA_SUBJECT="My own root CA" ' \
        '-e CA_EXPIRE="1825" -e SSL_EXPIRE="365" ' \
        '-e SSL_SUBJECT="{}" -e SSL_DNS="{}" ' \
        '-e SILENT="true" ' \
        'superseb/omgwtfssl'.format(bastion_node.host_name,
                                    bastion_node.host_name)
    bastion_node.execute_command(generate_certs_command)

    move_certs_command = \
        'sudo cat certs/cert.pem certs/ca.pem > ' \
        'basic-registry/nginx_config/domain.crt && ' \
        'sudo cat certs/key.pem > basic-registry/nginx_config/domain.key'
    bastion_node.execute_command(move_certs_command)

    # Add credentials for private registry
    store_creds_command = \
        'docker run --rm melsayed/htpasswd "{}" "{}" >> ' \
        'basic-registry/nginx_config/registry.password'.format(
            PRIVATE_REGISTRY_USERNAME, PRIVATE_REGISTRY_PASSWORD)
    bastion_node.execute_command(store_creds_command)

    # Ensure docker uses the certs that were generated
    update_docker_command = \
        'sudo mkdir -p /etc/docker/certs.d/{} && ' \
        'sudo cp ~/certs/ca.pem /etc/docker/certs.d/{}/ca.crt && ' \
        'sudo service docker restart'.format(
            bastion_node.host_name, bastion_node.host_name)
    bastion_node.execute_command(update_docker_command)

    # Run private registry
    docker_compose_command = \
        'cd basic-registry && ' \
        'sudo curl -L "https://github.com/docker/compose/releases/' \
        'download/1.24.1/docker-compose-$(uname -s)-$(uname -m)" ' \
        '-o /usr/local/bin/docker-compose && ' \
        'sudo chmod +x /usr/local/bin/docker-compose && ' \
        'sudo docker-compose up -d'
    bastion_node.execute_command(docker_compose_command)
    time.sleep(5)

    print("Bastion Server Details:\nNAME: {}\nHOST NAME: {}\n"
          "INSTANCE ID: {}\n".format(node_name, bastion_node.host_name,
                                     bastion_node.provider_node_id))

    return bastion_node


def add_rancher_images_to_private_registry(bastion_node, push_images=True):
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
        "./rancher-save-images.sh --image-list ./rancher-images.txt" \

    save_res = bastion_node.execute_command(save_images_command)

    if push_images:
        load_images_command = \
            "docker login {} -u \"{}\" -p \"{}\" && " \
            "./rancher-load-images.sh --image-list ./rancher-images.txt " \
            "--registry {}".format(
                bastion_node.host_name, PRIVATE_REGISTRY_USERNAME,
                PRIVATE_REGISTRY_PASSWORD, bastion_node.host_name)
        load_res = bastion_node.execute_command(load_images_command)
        print(load_res)
    else:
        load_res = None

    return save_res, load_res


def add_cleaned_images(bastion_node, images):
    failures = []
    for image in images:
        pull_image(bastion_node, image)
        cleaned_image = re.search(".*(rancher/.*)", image).group(1)
        tag_image(bastion_node, cleaned_image)
        push_image(bastion_node, cleaned_image)

        validate_result = validate_image(bastion_node, cleaned_image)
        if bastion_node.host_name not in validate_result[0]:
            failures.append(image)
    return failures


def add_images_to_private_registry(bastion_node, image_list):
    failures = []
    for image in image_list:
        pull_image(bastion_node, image)
        tag_image(bastion_node, image)
        push_image(bastion_node, image)

        validate_result = validate_image(bastion_node, image)
        if bastion_node.host_name not in validate_result[0]:
            failures.append(image)
    return failures


def pull_image(bastion_node, image):
    pull_image_command = "docker pull {}".format(image)
    bastion_node.execute_command(pull_image_command)


def tag_image(bastion_node, image):
    tag_image_command = "docker image tag {0} {1}/{0}".format(
        image, bastion_node.host_name)
    bastion_node.execute_command(tag_image_command)


def push_image(bastion_node, image):
    push_image_command = \
        "docker login {} -u \"{}\" -p \"{}\" && docker push {}/{}".format(
            bastion_node.host_name, PRIVATE_REGISTRY_USERNAME,
            PRIVATE_REGISTRY_PASSWORD, bastion_node.host_name, image)
    bastion_node.execute_command(push_image_command)


def validate_image(bastion_node, image):
    validate_image_command = "docker image ls {}/{}".format(
        bastion_node.host_name, image)
    return bastion_node.execute_command(validate_image_command)


def prepare_airgap_node(bastion_node, number_of_nodes):
    node_name = AG_HOST_NAME + "-airgap"
    # Create Airgap Node in AWS
    ag_nodes = AmazonWebServices().create_multiple_nodes(
        number_of_nodes, node_name, public_ip=False)

    for num, ag_node in enumerate(ag_nodes):
        # Update docker for the user in node
        ag_node_update_docker = \
            'ssh -i "{}.pem" -o StrictHostKeyChecking=no {}@{} ' \
            '"sudo usermod -aG docker {}"'.format(
                bastion_node.ssh_key_name, AWS_USER,
                ag_node.private_ip_address, AWS_USER)
        bastion_node.execute_command(ag_node_update_docker)

        # Update docker in node with bastion cert details
        ag_node_create_dir = \
            'ssh -i "{}.pem" -o StrictHostKeyChecking=no {}@{} ' \
            '"sudo mkdir -p /etc/docker/certs.d/{} && ' \
            'sudo chown {} /etc/docker/certs.d/{}"'.format(
                bastion_node.ssh_key_name, AWS_USER,
                ag_node.private_ip_address, bastion_node.host_name,
                AWS_USER, bastion_node.host_name)
        bastion_node.execute_command(ag_node_create_dir)

        ag_node_write_cert = \
            'scp -i "{}.pem" -o StrictHostKeyChecking=no ' \
            '/etc/docker/certs.d/{}/ca.crt ' \
            '{}@{}:/etc/docker/certs.d/{}/ca.crt'.format(
                bastion_node.ssh_key_name, bastion_node.host_name,
                AWS_USER, ag_node.private_ip_address, bastion_node.host_name)
        bastion_node.execute_command(ag_node_write_cert)

        ag_node_restart_docker = \
            'ssh -i "{}.pem" -o StrictHostKeyChecking=no {}@{} ' \
            '"sudo service docker restart"'.format(
                bastion_node.ssh_key_name, AWS_USER,
                ag_node.private_ip_address)
        bastion_node.execute_command(ag_node_restart_docker)

        ag_node_user_own_docker = \
            'ssh -i "{0}.pem" -o StrictHostKeyChecking=no {1}@{2} ' \
            '"sudo chown {1}:{1} /home/{1}/.docker -R && ' \
            'sudo chmod g+rwx "/home/{1}/.docker" -R"'.format(
                bastion_node.ssh_key_name, AWS_USER,
                ag_node.private_ip_address)
        bastion_node.execute_command(ag_node_user_own_docker)

        print("Airgapped Instance Details:\nNAME: {}-{}\nPRIVATE IP: {}\n"
              "".format(node_name, num, ag_node.private_ip_address))
    return ag_nodes


def copy_certs_to_node(bastion_node, ag_node):
    ag_node_copy_certs = \
        'scp -i "{0}.pem" -o StrictHostKeyChecking=no certs/* ' \
        '{1}@{2}:~/'.format(bastion_node.ssh_key_name, AWS_USER,
                            ag_node.private_ip_address)
    bastion_node.execute_command(ag_node_copy_certs)


# Note this only works on Ubuntu currently. There is a future enhancement
# to enable this to work for any OS.
def trust_certs_on_node(bastion_node, ag_node):
    ag_node_update_certs = \
        'sudo cp domain.crt ' \
        '/usr/local/share/ca-certificates/domain.crt && ' \
        'sudo update-ca-certificates'
    run_command_on_airgap_node(bastion_node, ag_node,
                               ag_node_update_certs)


def add_tarball_to_node(bastion_node, ag_node, tar_file, cluster_type):
    ag_node_copy_tarball = \
        'scp -i "{0}.pem" -o StrictHostKeyChecking=no ./{3} ' \
        '{1}@{2}:~/{3}'.format(bastion_node.ssh_key_name, AWS_USER,
                               ag_node.private_ip_address, tar_file)
    bastion_node.execute_command(ag_node_copy_tarball)
    ag_node_add_tarball_to_dir = \
        'sudo mkdir -p /var/lib/rancher/{0}/agent/images/ && ' \
        'sudo cp ./{1} /var/lib/rancher/{0}/agent/images/'.format(cluster_type,
                                                                  tar_file)
    run_command_on_airgap_node(bastion_node, ag_node,
                               ag_node_add_tarball_to_dir)


def prepare_private_registry(bastion_node, version):
    if 'k3s' in version:
        # Get k3s files associated with the specified version
        k3s_binary = 'k3s'
        if ARCH == 'arm64':
            k3s_binary = 'k3s-arm64'

        get_images_command = \
            'wget -O k3s-images.txt https://github.com/k3s-io/k3s/' \
            'releases/download/{0}/k3s-images.txt && ' \
            'wget -O k3s-install.sh https://get.k3s.io/ && ' \
            'wget -O k3s https://github.com/k3s-io/k3s/' \
            'releases/download/{0}/{1}'.format(version, k3s_binary)
        bastion_node.execute_command(get_images_command)

        images = bastion_node.execute_command(
            'cat k3s-images.txt')[0].strip().split("\n")
    elif 'rke2' in version:
        stripped_version = re.search("v(\d+.\d+.\d+)", version).group(1)
        if compare_versions(stripped_version, "1.21.2") < 0:
            get_images_command = \
                'wget -O rke2-images.txt https://github.com/rancher/rke2/' \
                'releases/download/{0}/rke2-images.linux-amd64.txt && ' \
                'wget -O rke2 https://github.com/rancher/rke2/' \
                'releases/download/{0}/rke2.linux-amd64'.format(version)
        else:
            get_images_command = \
                'wget -O rke2-images.txt https://github.com/rancher/rke2/' \
                'releases/download/{0}/rke2-images-all.linux-amd64.txt && ' \
                'wget -O rke2 https://github.com/rancher/rke2/' \
                'releases/download/{0}/rke2.linux-amd64'.format(version)
        bastion_node.execute_command(get_images_command)

        images = bastion_node.execute_command(
            'cat rke2-images.txt')[0].strip().split("\n")
    else:
        images = False
        pytest.fail("{} is not valid. Please provide a valid "
                    "k3s or rke2 version.".format(version))

    assert images
    failures = add_cleaned_images(bastion_node, images)
    assert failures == [], "Failed to add images: {}".format(failures)


def prepare_registries_mirror_on_node(bastion_node, ag_node, cluster_type):
    # Ensure registry file has correct data
    reg_file = readDataFile(RESOURCE_DIR, "airgap/registries.yaml")
    reg_file = reg_file.replace("$PRIVATE_REG", bastion_node.host_name)
    reg_file = reg_file.replace("$USERNAME", PRIVATE_REGISTRY_USERNAME)
    reg_file = reg_file.replace("$PASSWORD", PRIVATE_REGISTRY_PASSWORD)
    # Add registry file to node
    ag_node_create_dir = \
        'sudo mkdir -p /etc/rancher/{0} && ' \
        'sudo chown {1} /etc/rancher/{0}'.format(cluster_type, AWS_USER)
    run_command_on_airgap_node(bastion_node, ag_node,
                               ag_node_create_dir)
    write_reg_file_command = \
        "cat <<EOT >> /etc/rancher/{}/registries.yaml\n{}\nEOT".format(
            cluster_type, reg_file)
    run_command_on_airgap_node(bastion_node, ag_node,
                               write_reg_file_command)


def deploy_airgap_cluster(bastion_node, ag_nodes, k8s, server_ops, agent_ops):
    token = ""
    server_ip = ag_nodes[0].private_ip_address
    if not any(x == k8s for x in ["k3s", "rke2"]):
        raise ValueError("Please only use k3s or rke2, not {}".format(k8s))
    for num, ag_node in enumerate(ag_nodes):
        if num == 0:
            if k8s == "k3s":
                install_server = \
                    'INSTALL_K3S_SKIP_DOWNLOAD=true ./install.sh {} && sudo ' \
                    'chmod 644 /etc/rancher/k3s/k3s.yaml'.format(server_ops)
            else:
                install_server = \
                    'sudo rke2 server --write-kubeconfig-mode 644 {} ' \
                    '> /dev/null 2>&1 &'.format(server_ops)

            print("Install server command: {}".format(install_server))
            run_command_on_airgap_node(bastion_node, ag_node, install_server)
            time.sleep(30)
            token_command = 'sudo cat /var/lib/rancher/{}/server/node-token'.format(k8s)
            token = run_command_on_airgap_node(bastion_node, ag_node,
                                               token_command)[0].strip()
        else:
            if k8s == "k3s":
                install_worker = \
                    'INSTALL_K3S_SKIP_DOWNLOAD=true K3S_URL=https://{}:6443 ' \
                    'K3S_TOKEN={} ./install.sh {}'.format(server_ip, token,
                                                          agent_ops)
            else:
                install_worker = \
                    'sudo rke2 agent --server https://{}:9345 ' \
                    '--token {} {} > /dev/null 2>&1 &'.format(server_ip, token,
                                                              agent_ops)
            print("Install worker command: {}".format(install_worker))
            run_command_on_airgap_node(bastion_node, ag_node,
                                       install_worker)
            time.sleep(15)
    if k8s == "k3s":
        wait_for_airgap_pods_ready(bastion_node, ag_nodes)
    else:
        time.sleep(60)
        wait_for_airgap_pods_ready(bastion_node, ag_nodes,
                                   kubectl='/var/lib/rancher/rke2/bin/kubectl',
                                   kubeconfig='/etc/rancher/rke2/rke2.yaml')


def optionally_add_cluster_to_rancher(bastion_node, ag_nodes, prep="none"):
    if AGENT_REG_CMD:
        if prep == "k3s":
            for num, ag_node in enumerate(ag_nodes):
                prepare_registries_mirror_on_node(bastion_node, ag_node, 'k3s')
                restart_k3s = 'sudo systemctl restart k3s-agent'
                if num == 0:
                    restart_k3s = 'sudo systemctl restart k3s && ' \
                                  'sudo chmod 644 /etc/rancher/k3s/k3s.yaml'
                run_command_on_airgap_node(bastion_node, ag_node, restart_k3s)
        print("Adding to rancher server")
        result = run_command_on_airgap_node(bastion_node, ag_nodes[0],
                                            AGENT_REG_CMD)
        assert "deployment.apps/cattle-cluster-agent created" in result


def deploy_airgap_rancher(bastion_node):
    ag_node = prepare_airgap_node(bastion_node, 1)[0]
    privileged = "--privileged"
    if RANCHER_HA_CERT_OPTION == 'byo-valid':
        write_cert_command = "cat <<EOT >> fullchain.pem\n{}\nEOT".format(
            base64.b64decode(RANCHER_VALID_TLS_CERT).decode("utf-8"))
        run_command_on_airgap_node(bastion_node, ag_node,
                                   write_cert_command)
        write_key_command = "cat <<EOT >> privkey.pem\n{}\nEOT".format(
            base64.b64decode(RANCHER_VALID_TLS_KEY).decode("utf-8"))
        run_command_on_airgap_node(bastion_node, ag_node,
                                   write_key_command)
        deploy_rancher_command = \
            'sudo docker run -d {} --restart=unless-stopped ' \
            '-p 80:80 -p 443:443 ' \
            '-v ${{PWD}}/fullchain.pem:/etc/rancher/ssl/cert.pem ' \
            '-v ${{PWD}}/privkey.pem:/etc/rancher/ssl/key.pem ' \
            '-e CATTLE_SYSTEM_DEFAULT_REGISTRY={} ' \
            '-e CATTLE_SYSTEM_CATALOG=bundled ' \
            '-e CATTLE_BOOTSTRAP_PASSWORD=\\\"{}\\\" ' \
            '{}/rancher/rancher:{} --no-cacerts --trace'.format(
                privileged, bastion_node.host_name, ADMIN_PASSWORD,
                bastion_node.host_name, RANCHER_SERVER_VERSION)
    else:
        deploy_rancher_command = \
            'sudo docker run -d {} --restart=unless-stopped ' \
            '-p 80:80 -p 443:443 ' \
            '-e CATTLE_SYSTEM_DEFAULT_REGISTRY={} ' \
            '-e CATTLE_BOOTSTRAP_PASSWORD=\\\"{}\\\" ' \
            '-e CATTLE_SYSTEM_CATALOG=bundled {}/rancher/rancher:{} --trace'.format(
                privileged, bastion_node.host_name, 
                ADMIN_PASSWORD,
                bastion_node.host_name,
                RANCHER_SERVER_VERSION)
    deploy_result = run_command_on_airgap_node(bastion_node, ag_node,
                                               deploy_rancher_command,
                                               log_out=True)
    assert "Downloaded newer image for {}/rancher/rancher:{}".format(
        bastion_node.host_name, RANCHER_SERVER_VERSION) in deploy_result[1]
    return ag_node


def run_docker_command_on_airgap_node(bastion_node, ag_node, cmd,
                                      log_out=False):
    docker_login_command = "docker login {} -u \\\"{}\\\" -p \\\"{}\\\"".format(
        bastion_node.host_name,
        PRIVATE_REGISTRY_USERNAME, PRIVATE_REGISTRY_PASSWORD)
    if cmd.startswith("sudo"):
        docker_login_command = "sudo " + docker_login_command
    ag_command = \
        'ssh -i "{}.pem" -o StrictHostKeyChecking=no {}@{} ' \
        '"{} && {}"'.format(
            bastion_node.ssh_key_name, AWS_USER, ag_node.private_ip_address,
            docker_login_command, cmd)
    result = bastion_node.execute_command(ag_command)
    if log_out:
        print("Running command: {}".format(ag_command))
        print("Result: {}".format(result))
    return result


def run_command_on_airgap_node(bastion_node, ag_node, cmd, log_out=False):
    if cmd.startswith("docker") or cmd.startswith("sudo docker"):
        return run_docker_command_on_airgap_node(
            bastion_node, ag_node, cmd, log_out)
    ag_command = \
        'ssh -i "{}.pem" -o StrictHostKeyChecking=no {}@{} ' \
        '"{}"'.format(
            bastion_node.ssh_key_name, AWS_USER,
            ag_node.private_ip_address, cmd)
    result = bastion_node.execute_command(ag_command)
    if log_out:
        print("Running command: {}".format(ag_command))
        print("Result:\n{}".format("\n---\n".join(result)))
    return result


def wait_for_airgap_pods_ready(bastion_node, ag_nodes,
                               kubectl='kubectl', kubeconfig=None):
    if kubeconfig:
        node_cmd = "{} get nodes --kubeconfig {}".format(kubectl, kubeconfig)
        command = "{} get pods -A --kubeconfig {}".format(kubectl, kubeconfig)
    else:
        node_cmd = "{} get nodes".format(kubectl)
        command = "{} get pods -A".format(kubectl)
    start = time.time()
    wait_for_pods_to_be_ready = True
    while wait_for_pods_to_be_ready:
        unready_pods = []
        unready_nodes = []
        if time.time() - start > DEFAULT_CLUSTER_STATE_TIMEOUT:
            print_cluster_state(bastion_node, ag_nodes, kubectl, kubeconfig)
            raise AssertionError("Timed out waiting for cluster to be ready")
        time.sleep(10)
        nodes = run_command_on_airgap_node(bastion_node, ag_nodes[0], node_cmd)
        nodes_arr = nodes[0].strip().split("\n")[1:]
        missing_nodes = NUMBER_OF_INSTANCES - len(nodes_arr)
        if missing_nodes:
            print("Waiting for {} more node(s) to join the cluster.".format(
                missing_nodes))
            unready_nodes.append(missing_nodes)
        for node in nodes_arr:
            if "NotReady" in node:
                print("Waiting for node: {}".format(node))
                unready_nodes.append(node)
        if unready_nodes or not nodes_arr:
            continue
        pods = run_command_on_airgap_node(bastion_node, ag_nodes[0], command)
        pods_arr = pods[0].strip().split("\n")[1:]
        for pod in pods_arr:
            if "Completed" not in pod and "Running" not in pod:
                print("Waiting for pod: {}".format(pod))
                unready_pods.append(pod)
        if unready_pods or not pods_arr:
            wait_for_pods_to_be_ready = True
        else:
            wait_for_pods_to_be_ready = False
    print_cluster_state(bastion_node, ag_nodes, kubectl, kubeconfig)


def print_cluster_state(bastion_node, ag_nodes, kubectl='kubectl',
                        kubeconfig=None):
    if kubeconfig:
        cmd = "{} get nodes,pods -A -o wide --kubeconfig {}".format(kubectl,
                                                                    kubeconfig)
    else:
        cmd = "{} get nodes,pods -A -o wide".format(kubectl)
    run_command_on_airgap_node(bastion_node, ag_nodes[0], cmd, log_out=True)


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


def get_bastion_node(auth=True):
    if BASTION_ID:
        bastion_node = AmazonWebServices().get_node(BASTION_ID, ssh_access=True)
        print("Bastion Server Details:\nHOST NAME: {}\n"
              "INSTANCE ID: {}\n".format(bastion_node.host_name,
                                         bastion_node.provider_node_id))
    elif auth:
        bastion_node = deploy_bastion_server()
    else:
        bastion_node = deploy_noauth_bastion_server()
    if bastion_node is None:
        pytest.fail("Did not provide a valid Provider ID for the bastion node")
    return bastion_node


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


@pytest.fixture()
def check_hostname_length():
    print("Host Name: {}".format(AG_HOST_NAME))
    assert len(AG_HOST_NAME) < 17, "Provide hostname that is 16 chars or less"
