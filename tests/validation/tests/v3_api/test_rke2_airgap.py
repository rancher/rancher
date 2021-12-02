import os
from lib.aws import AWS_USER
from .common import AmazonWebServices
from .test_airgap import (AG_HOST_NAME, NUMBER_OF_INSTANCES, TARBALL_TYPE,
                          get_bastion_node, prepare_registries_mirror_on_node,
                          run_command_on_airgap_node, deploy_airgap_cluster,
                          prepare_private_registry, copy_certs_to_node,
                          trust_certs_on_node, add_tarball_to_node,
                          optionally_add_cluster_to_rancher)

RANCHER_RKE2_VERSION = os.environ.get("RANCHER_RKE2_VERSION", "")
RKE2_SERVER_OPTIONS = os.environ.get("RKE2_SERVER_OPTIONS", "")
RKE2_AGENT_OPTIONS = os.environ.get("RKE2_AGENT_OPTIONS", "")


def test_deploy_airgap_rke2_all():
    reg_bastion_node = get_bastion_node(auth=True)
    noauth_bastion_node = get_bastion_node(auth=False)
    add_rke2_tarball_to_bastion(noauth_bastion_node, RANCHER_RKE2_VERSION)
    prepare_private_registry(noauth_bastion_node, RANCHER_RKE2_VERSION)
    prepare_private_registry(reg_bastion_node, RANCHER_RKE2_VERSION)

    ag_nodes = prepare_airgap_rke2(noauth_bastion_node, NUMBER_OF_INSTANCES,
                                   'tarball+mirror+system-default')
    for ag_node in ag_nodes:
        copy_certs_to_node(reg_bastion_node, ag_node)
        prepare_registries_mirror_on_node(reg_bastion_node, ag_node, 'rke2')

    server_ops = RKE2_SERVER_OPTIONS + " --system-default-registry={}".format(
        noauth_bastion_node.host_name)
    agent_ops = RKE2_AGENT_OPTIONS + " --system-default-registry={}".format(
        noauth_bastion_node.host_name)
    deploy_airgap_cluster(noauth_bastion_node, ag_nodes, "rke2",
                          server_ops, agent_ops)
    optionally_add_cluster_to_rancher(noauth_bastion_node, ag_nodes)


def test_deploy_airgap_rke2_system_default_registry():
    bastion_node = get_bastion_node(auth=False)
    prepare_private_registry(bastion_node, RANCHER_RKE2_VERSION)
    ag_nodes = prepare_airgap_rke2(bastion_node, NUMBER_OF_INSTANCES,
                                   'system_default_registry')
    server_ops = RKE2_SERVER_OPTIONS + " --system-default-registry={}".format(
        bastion_node.host_name)
    agent_ops = RKE2_AGENT_OPTIONS + " --system-default-registry={}".format(
        bastion_node.host_name)
    deploy_airgap_cluster(bastion_node, ag_nodes, "rke2",
                          server_ops, agent_ops)


def test_deploy_airgap_rke2_private_registry():
    bastion_node = get_bastion_node(auth=True)
    prepare_private_registry(bastion_node, RANCHER_RKE2_VERSION)
    ag_nodes = prepare_airgap_rke2(bastion_node, NUMBER_OF_INSTANCES,
                                   'private_registry')
    deploy_airgap_cluster(bastion_node, ag_nodes, "rke2",
                          RKE2_SERVER_OPTIONS, RKE2_AGENT_OPTIONS)
    optionally_add_cluster_to_rancher(bastion_node, ag_nodes)


def test_deploy_airgap_rke2_tarball():
    bastion_node = get_bastion_node()
    add_rke2_tarball_to_bastion(bastion_node, RANCHER_RKE2_VERSION)
    ag_nodes = prepare_airgap_rke2(
        bastion_node, NUMBER_OF_INSTANCES, 'tarball')
    deploy_airgap_cluster(bastion_node, ag_nodes, "rke2",
                          RKE2_SERVER_OPTIONS, RKE2_AGENT_OPTIONS)


def add_rke2_tarball_to_bastion(bastion_node, rke2_version):
    get_tarball_command = \
        'wget -O rke2-airgap-images.{1} https://github.com/rancher/rke2/' \
        'releases/download/{0}/rke2-images.linux-amd64.{1} && ' \
        'wget -O rke2 https://github.com/rancher/rke2/releases/' \
        'download/{0}/rke2.linux-amd64'.format(rke2_version, TARBALL_TYPE)
    bastion_node.execute_command(get_tarball_command)
    if '--cni=calico' in RKE2_SERVER_OPTIONS:
        get_calico_tarball_command = \
            'wget -O rke2-airgap-images-calico.{1} ' \
            'https://github.com/rancher/rke2/releases/download/{0}/' \
            'rke2-images-calico.linux-amd64.{1}'.format(rke2_version,
                                                        TARBALL_TYPE)
        bastion_node.execute_command(get_calico_tarball_command)
    elif '--cni=cilium' in RKE2_SERVER_OPTIONS:
        get_cilium_tarball_command = \
            'wget -O rke2-airgap-images-cilium.{1} ' \
            'https://github.com/rancher/rke2/releases/download/{0}/' \
            'rke2-images-cilium.linux-amd64.{1}'.format(rke2_version,
                                                        TARBALL_TYPE)
        bastion_node.execute_command(get_cilium_tarball_command)
    if 'multus' in RKE2_SERVER_OPTIONS:
        get_multus_tarball_command = \
            'wget -O rke2-airgap-images-multus.{1} ' \
            'https://github.com/rancher/rke2/releases/download/{0}/' \
            'rke2-images-multus.linux-amd64.{1}'.format(rke2_version,
                                                        TARBALL_TYPE)
        bastion_node.execute_command(get_multus_tarball_command)


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
            copy_certs_to_node(bastion_node, ag_node)
            prepare_registries_mirror_on_node(bastion_node, ag_node, 'rke2')
        elif method == 'system_default_registry':
            copy_certs_to_node(bastion_node, ag_node)
            trust_certs_on_node(bastion_node, ag_node)
        elif method == 'tarball+mirror':
            copy_certs_to_node(bastion_node, ag_node)
            prepare_registries_mirror_on_node(bastion_node, ag_node, 'rke2')
        else:
            copy_certs_to_node(bastion_node, ag_node)
            trust_certs_on_node(bastion_node, ag_node)
        if 'tarball' in method:
            add_tarball_to_node(bastion_node, ag_node,
                                'rke2-airgap-images.{}'.format(TARBALL_TYPE),
                                'rke2')
            if '--cni=calico' in RKE2_SERVER_OPTIONS:
                add_tarball_to_node(
                    bastion_node, ag_node,
                    'rke2-airgap-images-calico.{}'.format(TARBALL_TYPE),
                    'rke2')
            elif '--cni=cilium' in RKE2_SERVER_OPTIONS:
                add_tarball_to_node(
                    bastion_node, ag_node,
                    'rke2-airgap-images-cilium.{}'.format(TARBALL_TYPE),
                    'rke2')
            if 'multus' in RKE2_SERVER_OPTIONS:
                add_tarball_to_node(
                    bastion_node, ag_node,
                    'rke2-airgap-images-multus.{}'.format(TARBALL_TYPE),
                    'rke2')

        print("Airgapped RKE2 Instance Details:\nNAME: {}-{}\nPRIVATE IP: {}\n"
              "".format(node_name, num, ag_node.private_ip_address))

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
    return ag_nodes
