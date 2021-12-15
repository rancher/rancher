import os
from lib.aws import AWS_USER
from .common import AmazonWebServices
from .test_airgap import (AG_HOST_NAME, ARCH, NUMBER_OF_INSTANCES,
                          TARBALL_TYPE,
                          get_bastion_node, prepare_registries_mirror_on_node,
                          run_command_on_airgap_node, prepare_private_registry,
                          copy_certs_to_node, deploy_airgap_cluster,
                          trust_certs_on_node, add_tarball_to_node,
                          optionally_add_cluster_to_rancher)

RANCHER_K3S_VERSION = os.environ.get("RANCHER_K3S_VERSION", "")
K3S_SERVER_OPTIONS = os.environ.get("K3S_SERVER_OPTIONS", "")
K3S_AGENT_OPTIONS = os.environ.get("K3S_AGENT_OPTIONS", "")


def test_deploy_airgap_k3s_system_default_registry():
    bastion_node = get_bastion_node(auth=False)
    prepare_private_registry(bastion_node, RANCHER_K3S_VERSION)
    ag_nodes = prepare_airgap_k3s(bastion_node, NUMBER_OF_INSTANCES,
                                  'system_default_registry')
    server_ops = K3S_SERVER_OPTIONS + " --system-default-registry={}".format(
        bastion_node.host_name)
    agent_ops = K3S_AGENT_OPTIONS
    deploy_airgap_cluster(bastion_node, ag_nodes, "k3s", server_ops, agent_ops)


def test_deploy_airgap_k3s_private_registry():
    bastion_node = get_bastion_node(auth=True)
    prepare_private_registry(bastion_node, RANCHER_K3S_VERSION)
    ag_nodes = prepare_airgap_k3s(bastion_node, NUMBER_OF_INSTANCES,
                                  'private_registry')
    deploy_airgap_cluster(bastion_node, ag_nodes, "k3s",
                          K3S_SERVER_OPTIONS, K3S_AGENT_OPTIONS)
    optionally_add_cluster_to_rancher(bastion_node, ag_nodes)


def test_deploy_airgap_k3s_tarball():
    bastion_node = get_bastion_node()
    add_k3s_tarball_to_bastion(bastion_node, RANCHER_K3S_VERSION)
    ag_nodes = prepare_airgap_k3s(bastion_node, NUMBER_OF_INSTANCES, 'tarball')
    deploy_airgap_cluster(bastion_node, ag_nodes, "k3s",
                          K3S_SERVER_OPTIONS, K3S_AGENT_OPTIONS)
    optionally_add_cluster_to_rancher(bastion_node, ag_nodes, prep="k3s")


def add_k3s_tarball_to_bastion(bastion_node, k3s_version):
    # Get k3s files associated with the specified version
    k3s_binary = 'k3s'
    if ARCH == 'arm64':
        k3s_binary = 'k3s-arm64'

    get_tarball_command = \
        'wget -O k3s-airgap-images-{1}.{3} https://github.com/k3s-io/k3s/' \
        'releases/download/{0}/k3s-airgap-images-{1}.{3} && ' \
        'wget -O k3s-install.sh https://get.k3s.io/ && ' \
        'wget -O k3s https://github.com/k3s-io/k3s/' \
        'releases/download/{0}/{2}'.format(k3s_version, ARCH, k3s_binary,
                                           TARBALL_TYPE)
    bastion_node.execute_command(get_tarball_command)


def prepare_airgap_k3s(bastion_node, number_of_nodes, method):
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
            '{1}@{2}:~/'.format(bastion_node.ssh_key_name, AWS_USER,
                                ag_node.private_ip_address)
        bastion_node.execute_command(ag_node_copy_files)

        ag_node_make_executable = \
            'sudo mv ./k3s /usr/local/bin/k3s && ' \
            'sudo chmod +x /usr/local/bin/k3s && sudo chmod +x install.sh'
        run_command_on_airgap_node(bastion_node, ag_node,
                                   ag_node_make_executable)

        if method == 'private_registry':
            prepare_registries_mirror_on_node(bastion_node, ag_node, 'k3s')
        elif method == 'system_default_registry':
            copy_certs_to_node(bastion_node, ag_node)
            trust_certs_on_node(bastion_node, ag_node)
        elif method == 'tarball':
            add_tarball_to_node(bastion_node, ag_node,
                                'k3s-airgap-images-{0}.{1}'.format(
                                    ARCH, TARBALL_TYPE), 'k3s')

        print("Airgapped K3S Instance Details:\nNAME: {}-{}\nPRIVATE IP: {}\n"
              "".format(node_name, num, ag_node.private_ip_address))

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
    return ag_nodes
