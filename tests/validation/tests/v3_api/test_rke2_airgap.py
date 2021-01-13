import os
import time
from lib.aws import AWS_USER
from .common import AmazonWebServices
from .test_airgap import (AG_HOST_NAME, BASTION_ID, NUMBER_OF_INSTANCES,
                          add_cleaned_images, get_bastion_node,
                          run_command_on_airgap_node, setup_ssh_key,
                          wait_for_airgap_pods_ready)

RANCHER_RKE2_VERSION = os.environ.get("RANCHER_RKE2_VERSION", "")
RKE2_SERVER_OPTIONS = os.environ.get("RANCHER_RKE2_SERVER_OPTIONS", "")
RKE2_AGENT_OPTIONS = os.environ.get("RANCHER_RKE2_AGENT_OPTIONS", "")


def test_deploy_airgap_rke2_private_registry():
    bastion_node = deploy_noauth_bastion_server()

    failures = add_rke2_images_to_private_registry(bastion_node,
                                                   RANCHER_RKE2_VERSION)
    assert failures == [], "Failed to add images: {}".format(failures)
    ag_nodes = prepare_airgap_rke2(bastion_node, NUMBER_OF_INSTANCES,
                                   'private_registry')
    assert len(ag_nodes) == NUMBER_OF_INSTANCES

    print(
        '{} airgapped rke2 instance(s) created.\n'
        'Connect to these and run commands by connecting to bastion node, '
        'then connecting to these:\n'
        'ssh -i {}.pem {}@NODE_PRIVATE_IP'.format(
            NUMBER_OF_INSTANCES, bastion_node.ssh_key_name, AWS_USER))
    for ag_node in ag_nodes:
        assert ag_node.private_ip_address is not None
        assert ag_node.public_ip_address is None

    server_ops = RKE2_SERVER_OPTIONS + " --system-default-registry={}".format(
        bastion_node.host_name)
    agent_ops = RKE2_AGENT_OPTIONS + " --system-default-registry={}".format(
        bastion_node.host_name)

    deploy_airgap_rke2_cluster(bastion_node, ag_nodes, server_ops, agent_ops)

    wait_for_airgap_pods_ready(bastion_node, ag_nodes,
                               kubectl='/var/lib/rancher/rke2/bin/kubectl',
                               kubeconfig='/etc/rancher/rke2/rke2.yaml')


def test_deploy_airgap_rke2_tarball():
    bastion_node = get_bastion_node(BASTION_ID)
    add_rke2_tarball_to_bastion(bastion_node, RANCHER_RKE2_VERSION)

    ag_nodes = prepare_airgap_rke2(
        bastion_node, NUMBER_OF_INSTANCES, 'tarball')
    assert len(ag_nodes) == NUMBER_OF_INSTANCES

    print(
        '{} airgapped rke2 instance(s) created.\n'
        'Connect to these and run commands by connecting to bastion node, '
        'then connecting to these:\n'
        'ssh -i {}.pem {}@NODE_PRIVATE_IP'.format(
            NUMBER_OF_INSTANCES, bastion_node.ssh_key_name, AWS_USER))
    for ag_node in ag_nodes:
        assert ag_node.private_ip_address is not None
        assert ag_node.public_ip_address is None

    deploy_airgap_rke2_cluster(bastion_node, ag_nodes,
                               RKE2_SERVER_OPTIONS, RKE2_AGENT_OPTIONS)

    wait_for_airgap_pods_ready(bastion_node, ag_nodes,
                               kubectl='/var/lib/rancher/rke2/bin/kubectl',
                               kubeconfig='/etc/rancher/rke2/rke2.yaml')


def deploy_noauth_bastion_server():
    node_name = AG_HOST_NAME + "-noauthbastion"
    # Create Bastion Server in AWS
    bastion_node = AmazonWebServices().create_node(node_name)
    setup_ssh_key(bastion_node)

    # Generate self signed certs
    generate_certs_command = \
        'mkdir -p certs && sudo openssl req -newkey rsa:4096 -nodes -sha256 ' \
        '-keyout certs/domain.key -x509 -days 365 -out certs/domain.crt ' \
        '-subj "/C=US/ST=AZ/O=Rancher QA/CN={}"'.format(bastion_node.host_name)
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


def add_rke2_tarball_to_bastion(bastion_node, rke2_version):
    get_tarball_command = \
        'wget -O rke2-airgap-images.tar.gz https://github.com/rancher/rke2/' \
        'releases/download/{0}/rke2-images.linux-amd64.tar.gz && ' \
        'wget -O rke2 https://github.com/rancher/rke2/' \
        'releases/download/{0}/rke2.linux-amd64'.format(rke2_version)
    bastion_node.execute_command(get_tarball_command)


def add_rke2_images_to_private_registry(bastion_node, rke2_version):
    get_images_command = \
        'wget -O rke2-images.txt https://github.com/rancher/rke2/' \
        'releases/download/{0}/rke2-images.linux-amd64.txt && ' \
        'wget -O rke2 https://github.com/rancher/rke2/' \
        'releases/download/{0}/rke2.linux-amd64'.format(rke2_version)
    bastion_node.execute_command(get_images_command)

    images = bastion_node.execute_command(
        'cat rke2-images.txt')[0].strip().split("\n")
    assert images
    return add_cleaned_images(bastion_node, images)


def prepare_airgap_rke2(bastion_node, number_of_nodes, method):
    node_name = AG_HOST_NAME + "-rke2-airgap"
    # Create Airgap Node in AWS
    ag_nodes = AmazonWebServices().create_multiple_nodes(
        number_of_nodes, node_name, public_ip=False)

    for num, ag_node in enumerate(ag_nodes):
        # Copy relevant rke2 files to airgapped node
        ag_node_copy_files = \
            'scp -i "{0}.pem" -o StrictHostKeyChecking=no ./rke2 ' \
            '{1}@{2}:~/rke2'.format(bastion_node.ssh_key_name, AWS_USER,
                                    ag_node.private_ip_address)
        bastion_node.execute_command(ag_node_copy_files)

        ag_node_make_executable = \
            'sudo mv ./rke2 /usr/local/bin/rke2 && ' \
            'sudo chmod +x /usr/local/bin/rke2'
        run_command_on_airgap_node(bastion_node, ag_node,
                                   ag_node_make_executable)

        if method == 'private_registry':
            ag_node_copy_certs = \
                'scp -i "{0}.pem" -o StrictHostKeyChecking=no certs/* ' \
                '{1}@{2}:~/'.format(bastion_node.ssh_key_name, AWS_USER,
                                    ag_node.private_ip_address)
            bastion_node.execute_command(ag_node_copy_certs)
            ag_node_update_certs = \
                'sudo cp domain.crt ' \
                '/usr/local/share/ca-certificates/domain.crt && ' \
                'sudo update-ca-certificates'
            run_command_on_airgap_node(bastion_node, ag_node,
                                       ag_node_update_certs)
        elif method == 'tarball':
            ag_node_copy_tarball = \
                'scp -i "{0}.pem" -o StrictHostKeyChecking=no ' \
                './rke2-airgap-images.tar.gz ' \
                '{1}@{2}:~/rke2-airgap-images.tar.gz'.format(
                    bastion_node.ssh_key_name, AWS_USER,
                    ag_node.private_ip_address)
            bastion_node.execute_command(ag_node_copy_tarball)
            ag_node_add_tarball_to_dir = \
                'sudo mkdir -p /var/lib/rancher/rke2/agent/images/ && ' \
                'sudo cp ./rke2-airgap-images.tar.gz ' \
                '/var/lib/rancher/rke2/agent/images/ && sudo gunzip ' \
                '/var/lib/rancher/rke2/agent/images/rke2-airgap-images.tar.gz'
            run_command_on_airgap_node(bastion_node, ag_node,
                                       ag_node_add_tarball_to_dir)

        print("Airgapped RKE2 Instance Details:\nNAME: {}-{}\nPRIVATE IP: {}\n"
              "".format(node_name, num, ag_node.private_ip_address))
    return ag_nodes


def deploy_airgap_rke2_cluster(bastion_node, ag_nodes, server_ops, agent_ops):
    token = ""
    server_ip = ag_nodes[0].private_ip_address
    for num, ag_node in enumerate(ag_nodes):
        if num == 0:
            # Install rke2 server
            install_rke2_server = \
                'sudo rke2 server --write-kubeconfig-mode 644 {} ' \
                '> /dev/null 2>&1 &'.format(server_ops)
            run_command_on_airgap_node(bastion_node, ag_node,
                                       install_rke2_server)
            time.sleep(30)
            token_command = 'sudo cat /var/lib/rancher/rke2/server/node-token'
            token = run_command_on_airgap_node(bastion_node, ag_node,
                                               token_command)[0].strip()
        else:
            install_rke2_worker = \
                'sudo rke2 agent --server https://{}:9345 ' \
                '--token {} {} > /dev/null 2>&1 &'.format(
                    server_ip, token, agent_ops)
            run_command_on_airgap_node(bastion_node, ag_node,
                                       install_rke2_worker)
            time.sleep(15)
