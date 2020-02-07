import json
import logging
import os
import paramiko
import time


logging.getLogger("paramiko").setLevel(logging.CRITICAL)
DOCKER_INSTALLED = os.environ.get("DOCKER_INSTALLED", "true")
DOCKER_INSTALL_CMD = (
    "curl https://releases.rancher.com/install-docker/{0}.sh | sh")
DOCKER_INSTALL_CMD_RHEL8 = (
    "sudo dnf install docker-ce-{0} --nobest -y")


class Node(object):
    def __init__(
        self, provider_node_id=None, host_name=None, node_name=None,
        public_ip_address=None, private_ip_address=None, state=None,
        labels=None, host_name_override=None, ssh_key=None,
        ssh_key_name=None, ssh_key_path=None, ssh_user=None,
            os_version=None, docker_version=None):

        self.provider_node_id = provider_node_id
        # node name giving to k8s node, hostname override
        self.node_name = node_name
        # Depending on the RKE config, this can be updated to be
        # either the internal IP, external IP address or FQDN name
        self.node_address = None
        self.host_name = host_name
        self.host_name_override = host_name_override
        self.public_ip_address = public_ip_address
        self.private_ip_address = private_ip_address
        self.ssh_user = ssh_user
        self.ssh_key = ssh_key
        self.ssh_key_name = ssh_key_name
        self.ssh_key_path = ssh_key_path
        self.os_version = os_version
        self.docker_version = docker_version
        self._roles = []
        self.labels = labels or {}
        self.state = state
        self._ssh_client = paramiko.SSHClient()
        self._ssh_client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
        self.ssh_port = '22'

    @property
    def roles(self):
        return self._roles

    @roles.setter
    def roles(self, r):
        self._roles = r

    def wait_for_ssh_ready(self):
        command = 'whoami'
        start_time = int(time.time())
        logs_while_waiting = ''
        while int(time.time()) - start_time < 100:
            try:
                self._ssh_client.connect(
                    self.public_ip_address, username=self.ssh_user,
                    key_filename=self.ssh_key_path, port=self.ssh_port)
                result = self._ssh_client.exec_command(command)
                if result and len(result) == 3 and result[1].readable():
                    result = [result[1].read(), result[2].read()]
                    self._ssh_client.close()
                    print("Successfully connected to node '{}' by SSH".format(self.public_ip_address))
                    return True
            except Exception as e:
                self._ssh_client.close()
                time.sleep(3)
                logs_while_waiting += str(e) + '\n'
                continue
        raise Exception(
            "Unable to connect to node '{0}' by SSH: {1}".format(
                self.public_ip_address, logs_while_waiting))

    def execute_command(self, command):
        result = None
        try:
            self._ssh_client.connect(
                self.public_ip_address, username=self.ssh_user,
                key_filename=self.ssh_key_path, port=self.ssh_port)
            result = self._ssh_client.exec_command(command)
            if result and len(result) == 3 and result[1].readable():
                result = [str(result[1].read(), 'utf-8'),
                          str(result[2].read(), 'utf-8')]
        finally:
            self._ssh_client.close()
        return result

    def install_docker(self):
        print("Installing Docker version {}".format(self.docker_version))
        # TODO: Fix to install native on RHEL 7.4
        command = (
            "{} && sudo usermod -aG docker {} && sudo systemctl enable docker"
            .format(
                DOCKER_INSTALL_CMD.format(self.docker_version),
                self.ssh_user))
        return self.execute_command(command)

    def install_docker_rhel8(self):
        print("RHEL8 detected, installing Docker version {}".format(self.docker_version))
        command = (
            "sudo dnf config-manager --add-repo=https://download.docker.com/linux/centos/docker-ce.repo &&" +
            "sudo sed -i '/^gpgkey=https:\/\/download.docker.com\/linux\/centos\/gpg/a module_hotfixes=True' /etc/yum.repos.d/docker-ce.repo &&")
        command += ("{} &&".format(DOCKER_INSTALL_CMD_RHEL8.format(self.docker_version)))
        command += ("sudo usermod -aG docker {} &&".format(self.ssh_user))
        command += (
            "sudo systemctl --now enable docker &&" +
            "sudo iptables -P FORWARD ACCEPT &&" +
            "echo 'net.ipv4.ip_forward = 1' | sudo tee -a /etc/sysctl.d/50-docker-forward.conf &&" +
            "for mod in ip_tables ip_vs_sh ip_vs ip_vs_rr ip_vs_wrr; do sudo modprobe $mod; echo $mod | sudo tee -a /etc/modules-load.d/iptables.conf; done &&" +
            "sudo dnf -y install network-scripts &&" +
            "sudo systemctl enable network &&" +
            "sudo systemctl disable NetworkManager")
        return self.execute_command(command)

    def ready_node(self):
        self.wait_for_ssh_ready()
        if DOCKER_INSTALLED.lower() == 'false':
            if self.os_version.startswith('rhel-8'):
                self.install_docker_rhel8()
            else:
                self.install_docker()

    def docker_ps(self, all=False, includeall=False):
        result = self.execute_command(
            'docker ps --format "{{.Names}}\t{{.Image}}"')
        if includeall:
            print("Docker ps including all containers")
            result = self.execute_command(
                'docker ps -a --format "{{.Names}}\t{{.Image}}"')
        if result[1] != '':
            raise Exception(
                "Error:'docker ps' command received this stderr output: "
                "{0}".format(result[1]))
        parse_out = result[0].strip('\n').split('\n')
        ret_dict = {}
        if parse_out == '':
            return ret_dict
        for item in parse_out:
            item0, item1 = item.split('\t')
            ret_dict[item0] = item1
        return ret_dict

    def docker_inspect(self, container_name, output_format=None):
        if output_format:
            command = 'docker inspect --format \'{0}\' {1}'.format(
                output_format, container_name)
        else:
            command = 'docker inspect {0}'.format(container_name)
        result = self.execute_command(command)
        if result[1] != '':
            raise Exception(
                "Error:'docker inspect' command received this stderr output: "
                "{0}".format(result[1]))
        result = json.loads(result[0])
        return result

    def docker_exec(self, container_name, cmd):
        command = 'docker exec {0} {1}'.format(container_name, cmd)
        result = self.execute_command(command)
        print(result)
        if result[1] != '':
            raise Exception(
                "Error:'docker exec' command received this stderr output: "
                "{0}".format(result[1]))
        return result[0]

    def get_public_ip(self):
        return self.public_ip_address
