from base64 import b64decode
from lib.aws import AWS_USER
import os
import pytest
from time import sleep

from .common import AmazonWebServices
from .common import random_test_name
from .common import run_command
from .common import wait_until_active

from .test_airgap import RESOURCE_DIR
from .test_airgap import SSH_KEY_DIR
from .test_airgap import add_rancher_images_to_private_registry
from .test_airgap import deploy_airgap_rancher
from .test_airgap import prepare_airgap_node
from .test_airgap import setup_ssh_key

from .test_create_ha import RANCHER_VALID_TLS_CERT
from .test_create_ha import RANCHER_VALID_TLS_KEY

from .test_custom_host_reg import RANCHER_SERVER_VERSION

HOST_NAME = os.environ.get('RANCHER_HOST_NAME', "testsa")
RA_HOST_NAME = random_test_name(HOST_NAME)
RANCHER_HOSTNAME = RA_HOST_NAME + ".qa.rancher.space"
REG_HOST_NAME = RA_HOST_NAME + "-registry"
REGISTRY_HOSTNAME = REG_HOST_NAME + ".qa.rancher.space"


def test_private_registry_no_auth():
    node_name = REG_HOST_NAME
    # Create Registry Server in AWS
    registry_node = AmazonWebServices().create_node(node_name)
    setup_ssh_key(registry_node)
    # update a record if it exists
    AmazonWebServices().upsert_route_53_record_a(
            REGISTRY_HOSTNAME, registry_node.get_public_ip())
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
    add_rancher_images_to_private_registry(
                                    registry_node,
                                    noauth_reg_name=REGISTRY_HOSTNAME)
    assert registry_node.execute_command("docker pull {}/{}:{}".format(
        REGISTRY_HOSTNAME,
        "/rancher/rancher-agent",
        RANCHER_SERVER_VERSION))[0].find("not found") < 0
    deploy_airgap_rancher(registry_node)


# "privateRegistries":[{"isDefault":true,"type":"privateRegistry","url":"thisisatest.com","user":null}]}
def deploy_rancher_server():
    if "v2.5" in RANCHER_SERVER_VERSION or "master" in RANCHER_SERVER_VERSION:
        RANCHER_SERVER_CMD = \
            'sudo docker run -d --privileged --name="rancher-server" ' \
            '--restart=unless-stopped -p 80:80 -p 443:443  ' \
            '-e CATTLE_SYSTEM_DEFAULT_REGISTRY={} ' \
            'rancher/rancher'.format(REGISTRY_HOSTNAME)
    else:
        RANCHER_SERVER_CMD = \
            'sudo docker run -d --name="rancher-server" ' \
            '--restart=unless-stopped -p 80:80 -p 443:443  ' \
            '-e CATTLE_SYSTEM_DEFAULT_REGISTRY={} ' \
            'rancher/rancher'.format(REGISTRY_HOSTNAME)
    RANCHER_SERVER_CMD += ":" + RANCHER_SERVER_VERSION + " --trace"
    print(RANCHER_SERVER_CMD)
    aws_nodes = AmazonWebServices().create_multiple_nodes(
        1, random_test_name(HOST_NAME))
    aws_nodes[0].execute_command(RANCHER_SERVER_CMD)
    sleep(120)
    RANCHER_SERVER_URL = "https://" + aws_nodes[0].public_ip_address
    print(RANCHER_SERVER_URL)
    wait_until_active(RANCHER_SERVER_URL, timeout=300)

    RANCHER_SET_DEBUG_CMD = \
        "sudo docker exec rancher-server loglevel --set debug"
    aws_nodes[0].execute_command(RANCHER_SET_DEBUG_CMD)


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
        '-o UserKnownHostsFile=/dev/null -r {}/airgap/basic-registry/ ' \
        '{}@{}:~/basic-registry/'.format(
            SSH_KEY_DIR, external_node.ssh_key_name, RESOURCE_DIR,
            AWS_USER, external_node.host_name)
    run_command(get_resources_command, log_out=False)


def run_docker_registry(external_node):
    docker_compose_command = \
        'cd ~/basic-registry && ' \
        'sudo curl -L "https://github.com/docker/compose/releases/' \
        'download/1.24.1/docker-compose-$(uname -s)-$(uname -m)" ' \
        '-o /usr/local/bin/docker-compose && ' \
        'sudo chmod +x /usr/local/bin/docker-compose && ' \
        'sudo docker-compose up -d'
    external_node.execute_command(docker_compose_command)
    sleep(5)
