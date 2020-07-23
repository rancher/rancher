import base64
import os
import pytest
import re
import time
from lib.aws import AWS_USER
from .common import (
    AmazonWebServices, run_command, wait_for_status_code,
    TEST_IMAGE, TEST_IMAGE_NGINX, TEST_IMAGE_OS_BASE, readDataFile,
    DEFAULT_CLUSTER_STATE_TIMEOUT
)
from .test_custom_host_reg import (
    random_test_name, RANCHER_SERVER_VERSION, HOST_NAME, AGENT_REG_CMD
)
from .test_create_ha import (
    set_url_and_password,
    RANCHER_HA_CERT_OPTION, RANCHER_VALID_TLS_CERT, RANCHER_VALID_TLS_KEY
)
from .test_import_k3s_cluster import (RANCHER_K3S_VERSION)

PRIVATE_REGISTRY_USERNAME = os.environ.get("RANCHER_BASTION_USERNAME")
PRIVATE_REGISTRY_PASSWORD = os.environ.get("RANCHER_BASTION_PASSWORD")
BASTION_ID = os.environ.get("RANCHER_BASTION_ID", "")
NUMBER_OF_INSTANCES = int(os.environ.get("RANCHER_AIRGAP_INSTANCE_COUNT", "1"))
IMAGE_LIST = os.environ.get("RANCHER_IMAGE_LIST", ",".join(
    [TEST_IMAGE, TEST_IMAGE_NGINX, TEST_IMAGE_OS_BASE])).split(",")

AG_HOST_NAME = random_test_name(HOST_NAME)
print("Host Name: {}".format(AG_HOST_NAME))
RANCHER_AG_INTERNAL_HOSTNAME = AG_HOST_NAME + "-internal.qa.rancher.space"
RANCHER_AG_HOSTNAME = AG_HOST_NAME + ".qa.rancher.space"
RESOURCE_DIR = os.path.join(os.path.dirname(os.path.realpath(__file__)),
                            'resource')
SSH_KEY_DIR = os.path.join(os.path.dirname(os.path.realpath(__file__)),
                           '.ssh')


def test_deploy_bastion():
    node = deploy_bastion_server()
    assert node.public_ip_address is not None


def test_deploy_airgap_rancher():
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
    bastion_node = get_bastion_node(BASTION_ID)
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
    bastion_node = get_bastion_node(BASTION_ID)
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

    results = add_cluster_to_rancher(bastion_node, ag_nodes)
    for result in results:
        assert "Downloaded newer image for {}/rancher/rancher-agent".format(
            bastion_node.host_name) in result[1]


def test_deploy_airgap_k3s():
    bastion_node = get_bastion_node(BASTION_ID)

    failures = add_k3s_images_to_private_registry(bastion_node,
                                                  RANCHER_K3S_VERSION)
    assert failures == [], "Failed to add images: {}".format(failures)
    ag_nodes = prepare_airgap_k3s(bastion_node, NUMBER_OF_INSTANCES)
    assert len(ag_nodes) == NUMBER_OF_INSTANCES

    print(
        '{} airgapped k3s instance(s) created.\n'
        'Connect to these and run commands by connecting to bastion node, '
        'then connecting to these:\n'
        'ssh -i {}.pem {}@NODE_PRIVATE_IP'.format(
            NUMBER_OF_INSTANCES, bastion_node.ssh_key_name, AWS_USER))
    for ag_node in ag_nodes:
        assert ag_node.private_ip_address is not None
        assert ag_node.public_ip_address is None

    deploy_airgap_k3s_cluster(bastion_node, ag_nodes)

    pods = run_command_on_airgap_node(
        bastion_node, ag_nodes[0], "kubectl get pods -A")
    pods_arr = pods[0].strip().split("\n")[1:]
    start = time.time()
    while pods_arr:
        new_arr = []
        if time.time() - start > DEFAULT_CLUSTER_STATE_TIMEOUT:
            raise AssertionError(
                "Timed out waiting for k3s cluster to be ready")
        time.sleep(5)
        pods = run_command_on_airgap_node(
            bastion_node, ag_nodes[0], "kubectl get pods -A")
        pods_arr = pods[0].strip().split("\n")[1:]
        for pod in pods_arr:
            if "Completed" not in pod and "Running" not in pod:
                print("Problem pod: {}".format(pod))
                new_arr.append(pod)
        pods_arr = new_arr
        print(pods_arr)

    # Optionally add k3s cluster to Rancher server
    if AGENT_REG_CMD:
        results = add_cluster_to_rancher(bastion_node, [ag_nodes[0]])
        for result in results:
            assert "deployment.apps/cattle-cluster-agent created" in result[0]


def test_add_rancher_images_to_private_registry():
    bastion_node = get_bastion_node(BASTION_ID)
    save_res, load_res = add_rancher_images_to_private_registry(bastion_node)
    assert "Image pull success: rancher/rancher:{}".format(
        RANCHER_SERVER_VERSION) in save_res[0]
    assert "The push refers to repository [{}/rancher/rancher]".format(
        bastion_node.host_name) in load_res[0]


def test_add_images_to_private_registry():
    bastion_node = get_bastion_node(BASTION_ID)
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
    set_url_and_password(base_url, "https://" + RANCHER_AG_INTERNAL_HOSTNAME)


def deploy_bastion_server():
    node_name = AG_HOST_NAME + "-bastion"
    # Create Bastion Server in AWS
    bastion_node = AmazonWebServices().create_node(node_name)

    # Copy SSH Key to Bastion and local dir and give it proper permissions
    write_key_command = "cat <<EOT >> {}.pem\n{}\nEOT".format(
        bastion_node.ssh_key_name,  bastion_node.ssh_key)
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

    # Get resources for private registry and generate self signed certs
    get_resources_command = \
        'scp -q -i {}/{}.pem -o StrictHostKeyChecking=no ' \
        '-o UserKnownHostsFile=/dev/null -r {}/airgap/basic-registry/ ' \
        '{}@{}:~/basic-registry/'.format(
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
        'docker run --rm melsayed/htpasswd {} {} >> ' \
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

    # Remove the "docker save" and "docker load" lines to save time
    edit_save_and_load_command = \
        "sudo sed -i '58d' rancher-save-images.sh && " \
        "sudo sed -i '76d' rancher-load-images.sh && " \
        "chmod +x rancher-save-images.sh && chmod +x rancher-load-images.sh"
    bastion_node.execute_command(edit_save_and_load_command)

    save_images_command = \
        "./rancher-save-images.sh --image-list ./rancher-images.txt"
    save_res = bastion_node.execute_command(save_images_command)

    if push_images:
        load_images_command = \
            "docker login {} -u {} -p {} && " \
            "./rancher-load-images.sh --image-list ./rancher-images.txt " \
            "--registry {}".format(
                bastion_node.host_name, PRIVATE_REGISTRY_USERNAME,
                PRIVATE_REGISTRY_PASSWORD, bastion_node.host_name)
        load_res = bastion_node.execute_command(load_images_command)
        print(load_res)
    else:
        load_res = None

    return save_res, load_res


def add_k3s_images_to_private_registry(bastion_node, k3s_version):
    failures = []
    # Get k3s files associated with the specified version
    get_images_command = \
        'wget -O k3s-images.txt https://github.com/rancher/k3s/' \
        'releases/download/{0}/k3s-images.txt && ' \
        'wget -O k3s-install.sh https://get.k3s.io/ && ' \
        'wget -O k3s https://github.com/rancher/k3s/' \
        'releases/download/{0}/k3s'.format(k3s_version)
    bastion_node.execute_command(get_images_command)

    images = bastion_node.execute_command(
        'cat k3s-images.txt')[0].strip().split("\n")
    assert images
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
        "docker login {} -u {} -p {} && docker push {}/{}".format(
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

        print("Airgapped Instance Details:\nNAME: {}-{}\nPRIVATE IP: {}\n"
              "".format(node_name, num, ag_node.private_ip_address))
    return ag_nodes


def prepare_airgap_k3s(bastion_node, number_of_nodes):
    node_name = AG_HOST_NAME + "-k3s-airgap"
    # Create Airgap Node in AWS
    ag_nodes = AmazonWebServices().create_multiple_nodes(
        number_of_nodes, node_name, public_ip=False)

    for num, ag_node in enumerate(ag_nodes):
        # Copy relevant k3s files to airgapped node
        ag_node_copy_files = \
            'scp -i "{0}.pem" -o StrictHostKeyChecking=no ./k3s-install.sh ' \
            '{1}@{2}:~/install.sh && ' \
            'scp -i "{0}.pem" -o StrictHostKeyChecking=no ./k3s ' \
            '{1}@{2}:~/k3s && ' \
            'scp -i "{0}.pem" -o StrictHostKeyChecking=no certs/* ' \
            '{1}@{2}:~/'.format(
                bastion_node.ssh_key_name, AWS_USER,
                ag_node.private_ip_address, bastion_node.host_name)
        bastion_node.execute_command(ag_node_copy_files)

        ag_node_make_executable = \
            'sudo mv ./k3s /usr/local/bin/k3s && ' \
            'sudo chmod +x /usr/local/bin/k3s && sudo chmod +x install.sh'
        run_command_on_airgap_node(bastion_node, ag_node,
                                   ag_node_make_executable)

        reg_file = readDataFile(RESOURCE_DIR, "airgap/registries.yaml")
        reg_file = reg_file.replace("$PRIVATE_REG", bastion_node.host_name)
        reg_file = reg_file.replace("$USERNAME", PRIVATE_REGISTRY_USERNAME)
        reg_file = reg_file.replace("$PASSWORD", PRIVATE_REGISTRY_PASSWORD)

        # Add registry file to node
        ag_node_create_dir = \
            'sudo mkdir -p /etc/rancher/k3s && ' \
            'sudo chown {} /etc/rancher/k3s'.format(AWS_USER)
        run_command_on_airgap_node(bastion_node, ag_node, ag_node_create_dir)

        write_reg_file_command = \
            "cat <<EOT >> /etc/rancher/k3s/registries.yaml\n{}\nEOT".format(
                reg_file)
        run_command_on_airgap_node(bastion_node, ag_node,
                                   write_reg_file_command)

        print("Airgapped K3S Instance Details:\nNAME: {}-{}\nPRIVATE IP: {}\n"
              "".format(node_name, num, ag_node.private_ip_address))
    return ag_nodes


def add_cluster_to_rancher(bastion_node, ag_nodes):
    results = []
    for ag_node in ag_nodes:
        deploy_result = run_command_on_airgap_node(bastion_node, ag_node,
                                                   AGENT_REG_CMD)
        results.append(deploy_result)
    return results


def deploy_airgap_k3s_cluster(bastion_node, ag_nodes):
    token = ""
    server_ip = ag_nodes[0].private_ip_address
    for num, ag_node in enumerate(ag_nodes):
        if num == 0:
            # Install k3s server
            install_k3s_server = \
                'INSTALL_K3S_SKIP_DOWNLOAD=true ./install.sh && ' \
                'sudo chmod 644 /etc/rancher/k3s/k3s.yaml'
            run_command_on_airgap_node(bastion_node, ag_node,
                                       install_k3s_server)
            token_command = 'sudo cat /var/lib/rancher/k3s/server/node-token'
            token = run_command_on_airgap_node(bastion_node, ag_node,
                                               token_command)[0].strip()
        else:
            install_k3s_worker = \
                'INSTALL_K3S_SKIP_DOWNLOAD=true K3S_URL=https://{}:6443 ' \
                'K3S_TOKEN={} ./install.sh'.format(server_ip, token)
            run_command_on_airgap_node(bastion_node, ag_node,
                                       install_k3s_worker)


def deploy_airgap_rancher(bastion_node):
    ag_node = prepare_airgap_node(bastion_node, 1)[0]
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
            'sudo docker run -d --restart=unless-stopped ' \
            '-p 80:80 -p 443:443 ' \
            '-v ${{PWD}}/fullchain.pem:/etc/rancher/ssl/cert.pem ' \
            '-v ${{PWD}}/privkey.pem:/etc/rancher/ssl/key.pem ' \
            '-e CATTLE_SYSTEM_DEFAULT_REGISTRY={} ' \
            '-e CATTLE_SYSTEM_CATALOG=bundled ' \
            '{}/rancher/rancher:{} --no-cacerts'.format(
                bastion_node.host_name, bastion_node.host_name,
                RANCHER_SERVER_VERSION)
    else:
        deploy_rancher_command = \
            'sudo docker run -d --restart=unless-stopped ' \
            '-p 80:80 -p 443:443 ' \
            '-e CATTLE_SYSTEM_DEFAULT_REGISTRY={} ' \
            '-e CATTLE_SYSTEM_CATALOG=bundled {}/rancher/rancher:{}'.format(
                bastion_node.host_name, bastion_node.host_name,
                RANCHER_SERVER_VERSION)
    deploy_result = run_command_on_airgap_node(bastion_node, ag_node,
                                               deploy_rancher_command,
                                               log_out=True)
    assert "Downloaded newer image for {}/rancher/rancher:{}".format(
        bastion_node.host_name, RANCHER_SERVER_VERSION) in deploy_result[1]
    return ag_node


def run_docker_command_on_airgap_node(bastion_node, ag_node, cmd,
                                      log_out=False):
    docker_login_command = "docker login {} -u {} -p {}".format(
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
        print("Result: {}".format(result))
    return result


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
        if health80 in ['initial', 'healthy']\
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


def get_bastion_node(provider_id):
    bastion_node = AmazonWebServices().get_node(provider_id, ssh_access=True)
    if bastion_node is None:
        pytest.fail("Did not provide a valid Provider ID for the bastion node")
    return bastion_node
