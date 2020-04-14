import json
import logging
import paramiko
import time


logging.getLogger("paramiko").setLevel(logging.CRITICAL)
DOCKER_INSTALL_CMD = (
    "curl https://releases.rancher.com/install-docker/{0}.sh | sh")


class Node(object):
    def __init__(self, provider_node_id=None, host_name=None, node_name=None,
                 public_ip_address=None, private_ip_address=None, state=None,
                 labels=None, host_name_override=None, ssh_key=None,
                 ssh_key_name=None, ssh_key_path=None, ssh_user=None,
                 docker_version=None, docker_installed="false"):

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
        self.docker_version = docker_version
        self.docker_installed = docker_installed
        self._roles = []
        self.labels = labels or {}
        self.state = state
        self._ssh_client = paramiko.SSHClient()
        self._ssh_client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
        self.ssh_port = '22'
        self._ssh_password = None

    @property
    def ssh_password(self):
        return self._ssh_password

    @ssh_password.setter
    def ssh_password(self, password):
        self._ssh_password = password

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
                    key_filename=self.ssh_key_path, port=int(self.ssh_port))
                result = self._ssh_client.exec_command(command)
                if result and len(result) == 3 and result[1].readable():
                    result = [result[1].read(), result[2].read()]
                    self._ssh_client.close()
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
            if self.ssh_password is not None:
                self._ssh_client.connect(
                    self.public_ip_address, username=self.ssh_user,
                    password=self.ssh_password, port=int(self.ssh_port))
            else:
                self._ssh_client.connect(
                    self.public_ip_address, username=self.ssh_user,
                    key_filename=self.ssh_key_path, port=int(self.ssh_port))

            result = self._ssh_client.exec_command(command)
            if result and len(result) == 3 and result[1].readable():
                result = [str(result[1].read(), 'utf-8'),
                          str(result[2].read(), 'utf-8')]
        finally:
            self._ssh_client.close()
        return result

    def install_docker(self):
        # TODO: Fix to install native on RHEL 7.4
        command = (
            "{} && sudo usermod -aG docker {} && sudo systemctl enable docker"
            .format(
                DOCKER_INSTALL_CMD.format(self.docker_version),
                self.ssh_user))
        return self.execute_command(command)

    def ready_node(self):
        # ignore Windows node
        if self.ssh_user == "Administrator":
            return

        self.wait_for_ssh_ready()
        if self.docker_installed.lower() == 'false':
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
