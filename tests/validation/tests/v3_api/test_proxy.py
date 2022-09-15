import os
import time
from lib.aws import AWS_USER
from .common import (
    ADMIN_PASSWORD, AmazonWebServices, run_command
)
from .test_airgap import get_bastion_node
from .test_custom_host_reg import (
    random_test_name, RANCHER_SERVER_VERSION, HOST_NAME, AGENT_REG_CMD
)
BASTION_ID = os.environ.get("RANCHER_BASTION_ID", "")
NUMBER_OF_INSTANCES = int(os.environ.get("RANCHER_AIRGAP_INSTANCE_COUNT", "1"))

PROXY_HOST_NAME = random_test_name(HOST_NAME)
RANCHER_PROXY_INTERNAL_HOSTNAME = \
    PROXY_HOST_NAME + "-internal.qa.rancher.space"
RANCHER_PROXY_HOSTNAME = PROXY_HOST_NAME + ".qa.rancher.space"

RESOURCE_DIR = os.path.join(os.path.dirname(os.path.realpath(__file__)),
                            'resource')
SSH_KEY_DIR = os.path.join(os.path.dirname(os.path.realpath(__file__)),
                           '.ssh')

RANCHER_PROXY_PORT = os.environ.get("RANCHER_PROXY_PORT", "3131")
RANCHER_CIDR_OVERRIDE = os.environ.get("RANCHER_CIDR_OVERRIDE", "")


def deploy_proxy_server():
    node_name = PROXY_HOST_NAME + "-proxy"
    proxy_node = AmazonWebServices().create_node(node_name)

    # Copy SSH Key to proxy and local dir and give it proper permissions
    write_key_command = "cat <<EOT >> {}.pem\n{}\nEOT".format(
        proxy_node.ssh_key_name, proxy_node.ssh_key)
    proxy_node.execute_command(write_key_command)
    local_write_key_command = \
        "mkdir -p {} && cat <<EOT >> {}/{}.pem\n{}\nEOT".format(
            SSH_KEY_DIR, SSH_KEY_DIR,
            proxy_node.ssh_key_name, proxy_node.ssh_key)
    run_command(local_write_key_command, log_out=False)

    set_key_permissions_command = "chmod 400 {}.pem".format(
        proxy_node.ssh_key_name)
    proxy_node.execute_command(set_key_permissions_command)
    local_set_key_permissions_command = "chmod 400 {}/{}.pem".format(
        SSH_KEY_DIR, proxy_node.ssh_key_name)
    run_command(local_set_key_permissions_command, log_out=False)

    # Write the proxy config to the node and run the proxy
    proxy_node.execute_command("mkdir -p /home/ubuntu/squid/")

    copy_cfg_command = \
        'scp -q -i {}/{}.pem -o StrictHostKeyChecking=no ' \
        '-o UserKnownHostsFile=/dev/null {}/squid/squid.conf ' \
        '{}@{}:~/squid/squid.conf'.format(
            SSH_KEY_DIR, proxy_node.ssh_key_name, RESOURCE_DIR,
            AWS_USER, proxy_node.host_name)
    run_command(copy_cfg_command, log_out=True)

    squid_cmd = "sudo docker run -d " \
                "-v /home/ubuntu/squid/squid.conf:/etc/squid/squid.conf " \
                "-p {}:3128 wernight/squid".format(RANCHER_PROXY_PORT)

    proxy_node.execute_command(squid_cmd)

    print("Proxy Server Details:\nNAME: {}\nHOST NAME: {}\n"
          "INSTANCE ID: {}\n".format(node_name, proxy_node.host_name,
                                     proxy_node.provider_node_id))
    return proxy_node


def run_command_on_proxy_node(bastion_node, ag_node, cmd, log_out=False):
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


def prepare_airgap_proxy_node(bastion_node, number_of_nodes):
    node_name = PROXY_HOST_NAME + "-agproxy"
    ag_nodes = AmazonWebServices().create_multiple_nodes(
        number_of_nodes, node_name, public_ip=False)

    for num, ag_node in enumerate(ag_nodes):
        ag_node_update_docker = \
            'ssh -i "{}.pem" -o StrictHostKeyChecking=no {}@{} ' \
            '"sudo usermod -aG docker {}"'.format(
                bastion_node.ssh_key_name, AWS_USER,
                ag_node.private_ip_address, AWS_USER)
        bastion_node.execute_command(ag_node_update_docker)

        proxy_url = bastion_node.host_name + ":" + RANCHER_PROXY_PORT
        proxy_info = '[Service]\nEnvironment=\"HTTP_PROXY={}\" ' \
                     '\"HTTPS_PROXY={}\" ' \
                     '\"NO_PROXY=localhost,127.0.0.1,0.0.0.0,10.0.0.0/8,' \
                     'cattle-system.svc,{}\"' \
                     .format(proxy_url, proxy_url, RANCHER_CIDR_OVERRIDE)

        bastion_node.execute_command('echo "{}" > http-proxy.conf'
                                     .format(proxy_info))

        ag_node_create_dir = \
            'ssh -i "{}.pem" -o StrictHostKeyChecking=no {}@{} ' \
            '"sudo mkdir -p /etc/systemd/system/docker.service.d"'.format(
                bastion_node.ssh_key_name, AWS_USER,
                ag_node.private_ip_address)
        bastion_node.execute_command(ag_node_create_dir)

        copy_conf_cmd = \
            'scp -q -i "{}".pem -o StrictHostKeyChecking=no -o ' \
            'UserKnownHostsFile=/dev/null ~/http-proxy.conf ' \
            '{}@{}:~/'.format(bastion_node.ssh_key_name, AWS_USER,
                              ag_node.private_ip_address)
        bastion_node.execute_command(copy_conf_cmd)

        ag_node_mv_conf = \
            'ssh -i "{}.pem" -o StrictHostKeyChecking=no ' \
            '-o UserKnownHostsFile=/dev/null {}@{} ' \
            '"sudo mv http-proxy.conf /etc/systemd/system/docker.service.d/ ' \
            '&& sudo systemctl daemon-reload && ' \
            'sudo systemctl restart docker"'.format(
                bastion_node.ssh_key_name, AWS_USER,
                ag_node.private_ip_address)
        bastion_node.execute_command(ag_node_mv_conf)

        print("Airgapped Proxy Instance Details:\n"
              "NAME: {}-{}\nPRIVATE IP: {}\n"
              "".format(node_name, num, ag_node.private_ip_address))
    return ag_nodes


def deploy_proxy_rancher(bastion_node):
    ag_node = prepare_airgap_proxy_node(bastion_node, 1)[0]
    proxy_url = bastion_node.host_name + ":" + RANCHER_PROXY_PORT
    deploy_rancher_command = \
        'sudo docker run -d --privileged --restart=unless-stopped ' \
        '-p 80:80 -p 443:443 ' \
        '-e HTTP_PROXY={} ' \
        '-e HTTPS_PROXY={} ' \
        '-e NO_PROXY="localhost,127.0.0.1,0.0.0.0,10.0.0.0/8,' \
        'cattle-system.svc,{}" ' \
        '-e CATTLE_BOOTSTRAP_PASSWORD=\\\"{}\\\" ' \
        'rancher/rancher:{} --trace'.format(
            proxy_url, proxy_url, RANCHER_CIDR_OVERRIDE,
            ADMIN_PASSWORD, RANCHER_SERVER_VERSION)

    deploy_result = run_command_on_proxy_node(bastion_node, ag_node,
                                              deploy_rancher_command,
                                              log_out=True)
    assert "Downloaded newer image for rancher/rancher:{}".format(
        RANCHER_SERVER_VERSION) in deploy_result[1]
    return ag_node


def register_cluster_nodes(bastion_node, ag_nodes):
    results = []
    for ag_node in ag_nodes:
        deploy_result = run_command_on_proxy_node(bastion_node, ag_node,
                                                  AGENT_REG_CMD)
        results.append(deploy_result)
    return results


def create_nlb_and_add_targets(aws_nodes):
    # Create internet-facing nlb and grab ARN & dns name
    lb = AmazonWebServices().create_network_lb(name=PROXY_HOST_NAME + "-nlb")
    lb_arn = lb["LoadBalancers"][0]["LoadBalancerArn"]
    public_dns = lb["LoadBalancers"][0]["DNSName"]
    # Create internal nlb and grab ARN & dns name
    internal_lb = AmazonWebServices().create_network_lb(
        name=PROXY_HOST_NAME + "-internal-nlb", scheme='internal')
    internal_lb_arn = internal_lb["LoadBalancers"][0]["LoadBalancerArn"]
    internal_lb_dns = internal_lb["LoadBalancers"][0]["DNSName"]

    # Upsert the route53 record -- if it exists, update, if not, insert
    AmazonWebServices().upsert_route_53_record_cname(
        RANCHER_PROXY_INTERNAL_HOSTNAME, internal_lb_dns)

    AmazonWebServices().upsert_route_53_record_cname(
        RANCHER_PROXY_HOSTNAME, public_dns)
    public_dns = RANCHER_PROXY_HOSTNAME

    # Create the target groups
    tg80 = AmazonWebServices(). \
        create_ha_target_group(80, PROXY_HOST_NAME + "-tg-80")
    tg443 = AmazonWebServices(). \
        create_ha_target_group(443, PROXY_HOST_NAME + "-tg-443")
    tg80_arn = tg80["TargetGroups"][0]["TargetGroupArn"]
    tg443_arn = tg443["TargetGroups"][0]["TargetGroupArn"]
    # Create the internal target groups
    internal_tg80 = AmazonWebServices(). \
        create_ha_target_group(80, PROXY_HOST_NAME + "-internal-tg-80")
    internal_tg443 = AmazonWebServices(). \
        create_ha_target_group(443, PROXY_HOST_NAME + "-internal-tg-443")
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


def test_deploy_proxied_rancher():
    proxy_node = deploy_proxy_server()
    proxy_rancher_node = deploy_proxy_rancher(proxy_node)
    public_dns = create_nlb_and_add_targets([proxy_rancher_node])
    print(
        "\nConnect to bastion node with:\nssh -i {}.pem {}@{}\n"
        "Connect to rancher node by connecting to bastion, then run:\n"
        "ssh -i {}.pem {}@{}\n\nOpen the Rancher UI with: https://{}\n"
        "".format(
            proxy_node.ssh_key_name, AWS_USER,
            proxy_node.host_name,
            proxy_node.ssh_key_name, AWS_USER,
            proxy_rancher_node.private_ip_address,
            public_dns))


def test_deploy_proxy_nodes():
    bastion_node = get_bastion_node(BASTION_ID)
    ag_nodes = prepare_airgap_proxy_node(bastion_node, NUMBER_OF_INSTANCES)
    assert len(ag_nodes) == NUMBER_OF_INSTANCES

    print(
        '{} airgapped instance(s) created.\n'
        'Connect to these and run commands by connecting to bastion node, '
        'then running the following command (with the quotes):\n'
        'ssh -i {}.pem {}@NODE_PRIVATE_IP '.format(
            NUMBER_OF_INSTANCES, bastion_node.ssh_key_name,
            AWS_USER))

    for ag_node in ag_nodes:
        assert ag_node.private_ip_address is not None
        assert ag_node.public_ip_address is None

    results = register_cluster_nodes(bastion_node, ag_nodes)
    for result in results:
        assert "Downloaded newer image for rancher/rancher-agent" in result[1]
