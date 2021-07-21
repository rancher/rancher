import os
import time
import json
from lib.aws import AWS_USER
from .test_airgap import add_rancher_images_to_private_registry
from .common import (
    AmazonWebServices, run_command
)
from .test_airgap import get_bastion_node
from .test_custom_host_reg import (
    random_test_name, RANCHER_SERVER_VERSION, HOST_NAME, AGENT_REG_CMD
)

### Please set the following environment vars: RANCHER_SERVER_VERSION,
### RANCHER_BASTION_USERNAME, RANCHER_BASTION_PASSWORD, RANCHER_HOST_NAME,
### AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, AWS_SSH_KEY_NAME
### Optional vars: RANCHER_PR_PORT, AWS_INSTANCE_TYPE

PR_HOST_NAME = random_test_name(HOST_NAME)
PRIVATE_REGISTRY_USERNAME = os.environ.get("RANCHER_BASTION_USERNAME")
PRIVATE_REGISTRY_PASSWORD = \
    os.environ.get("RANCHER_BASTION_PASSWORD")
SSH_KEY_DIR = os.path.join(os.path.dirname(os.path.realpath(__file__)),
                           '.ssh')
RANCHER_PR_PORT = os.environ.get("RANCHER_PR_PORT", "3131")

def test_deploy_private_registry():
    node_name = PR_HOST_NAME + "-pvt-reg"
    private_registry_node = AmazonWebServices().create_node(node_name)

    # Copy SSH Key to pvt_rgstry + local dir, then give proper permissions
    write_key_command = "cat <<EOT >> {}.pem\n{}\nEOT".format(
        private_registry_node.ssh_key_name, private_registry_node.ssh_key)
    private_registry_node.execute_command(write_key_command)
    local_write_key_command = \
        "mkdir -p {} && cat <<EOT >> {}/{}.pem\n{}\nEOT".format(
            SSH_KEY_DIR, SSH_KEY_DIR,
            private_registry_node.ssh_key_name, private_registry_node.ssh_key)
    run_command(local_write_key_command, log_out=False)

    set_key_permissions_command = "chmod 600 {}.pem".format(
        private_registry_node.ssh_key_name)
    private_registry_node.execute_command(set_key_permissions_command)
    local_set_key_permissions_command = "chmod 600 {}/{}.pem".format(
        SSH_KEY_DIR, private_registry_node.ssh_key_name)
    run_command(local_set_key_permissions_command, log_out=False)

    # Write the private_registry config to the node and run the private_registry
    private_registry_node.execute_command(
    "sudo apt-get -y install apache2-utils")

    registry_json={"insecure-registries" : ["{}:{}".format(
    private_registry_node.public_ip_address,RANCHER_PR_PORT)]}
    bypass_insecure_cmd="sudo echo '{}' " \
    "| sudo tee /etc/docker/daemon.json && " \
    "sudo systemctl daemon-reload && sudo systemctl restart docker".format(
    json.dumps(registry_json)
    )
    private_registry_node.execute_command(bypass_insecure_cmd)

    pr_cmd="mkdir auth && htpasswd -Bbn {} " \
    "{} > auth/htpasswd && " \
    "docker run -d   -p {}:5000   --restart=always   --name registry2 " \
    "-v \"$(pwd)\"/auth:/auth   -e \"REGISTRY_AUTH=htpasswd\" " \
    "-e \"REGISTRY_AUTH_HTPASSWD_REALM=Registry Realm\"   -e " \
    "REGISTRY_AUTH_HTPASSWD_PATH=/auth/htpasswd   registry:2".format(
    PRIVATE_REGISTRY_USERNAME,PRIVATE_REGISTRY_PASSWORD,RANCHER_PR_PORT)
    private_registry_node.execute_command(pr_cmd)

    pr_login_cmd="docker login {}:{} -u {} -p {}".format(
    private_registry_node.public_ip_address,RANCHER_PR_PORT,
    PRIVATE_REGISTRY_USERNAME,PRIVATE_REGISTRY_PASSWORD)
    private_registry_node.execute_command(pr_login_cmd)

    #get save/load images and install to registry
    get_images_cmd = "cd ~/ && " \
        'wget -O rancher-images.txt https://github.com/rancher/rancher/' \
        'releases/download/{0}/rancher-images.txt && ' \
        'wget -O rancher-save-images.sh https://github.com/rancher/rancher/' \
        'releases/download/{0}/rancher-save-images.sh && ' \
        'wget -O rancher-load-images.sh https://github.com/rancher/rancher/' \
        'releases/download/{0}/rancher-load-images.sh'.format(
            RANCHER_SERVER_VERSION)
    private_registry_node.execute_command(get_images_cmd)
    check_images=private_registry_node.execute_command(
        "if grep -q rancher rancher-images.txt; then" \
        " grep rancher rancher-images.txt | head -1; fi")
    assert check_images[0].find("rancher") > -1

    apply_images_cmd="sudo sed -i '58d' rancher-save-images.sh && " \
    "sudo sed -i '76d' rancher-load-images.sh && " \
    "chmod +x rancher-save-images.sh && chmod +x rancher-load-images.sh && " \
    "./rancher-save-images.sh --image-list ./rancher-images.txt && " \
    "./rancher-load-images.sh --image-list ./rancher-images.txt --registry " \
    "{}:{}".format(private_registry_node.public_ip_address,RANCHER_PR_PORT)
    private_registry_node.execute_command(apply_images_cmd)

    check_image_pull = private_registry_node.execute_command("docker pull {}:{}/{}".format(
        private_registry_node.public_ip_address,RANCHER_PR_PORT, check_images[0]))
    assert check_image_pull[0].find("Image is up to date") > -1

    print("Private Registry Details:\nNAME: {}\nHOST NAME: {}\n"
          "INSTANCE ID: {}\n".format(node_name, private_registry_node.host_name,
                                     private_registry_node.provider_node_id),
                                      "\npswd: {}\nusr: {}".format(
                                      PRIVATE_REGISTRY_PASSWORD,
                                      PRIVATE_REGISTRY_USERNAME))
    print("public IP: {}:{}".format(private_registry_node.public_ip_address,
    RANCHER_PR_PORT))
    assert int(private_registry_node.ssh_port) == 22
    return private_registry_node
