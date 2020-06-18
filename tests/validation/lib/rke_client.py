import os
import jinja2
import logging
import tempfile
import time
import subprocess
from yaml import load


logging.getLogger('invoke').setLevel(logging.WARNING)
DEBUG = os.environ.get('DEBUG', 'false')

DEFAULT_CONFIG_NAME = 'cluster.yml'
DEFAULT_NETWORK_PLUGIN = os.environ.get('DEFAULT_NETWORK_PLUGIN', 'canal')


class RKEClient(object):
    """
    Wrapper to interact with the RKE cli
    """
    def __init__(self, master_ssh_key_path, template_path):
        self.master_ssh_key_path = master_ssh_key_path
        self.template_path = template_path
        self._working_dir = tempfile.mkdtemp()
        self._hide = False if DEBUG.lower() == 'true' else True

    def _run(self, command):
        print('Running command: {}'.format(command))
        start_time = time.time()
        result = self.run_command('cd {0} && {1}'.format(self._working_dir,
                                                         command))
        end_time = time.time()
        print('Run time for command {0}: {1} seconds'.format(
            command, end_time - start_time))
        return result

    def up(self, config_yml, config=None):
        yml_name = config if config else DEFAULT_CONFIG_NAME
        self._save_cluster_yml(yml_name, config_yml)
        cli_args = '' if config is None else ' --config {0}'.format(config)
        result = self._run("/usr/local/bin/rke up {0}".format(cli_args))
        print(
            "RKE kube_config:\n{0}".format(self.get_kube_config_for_config()))
        return result

    def remove(self, config=None):
        result = self._run("/usr/local/bin/rke remove --force")
        return result

    def build_rke_template(self, template, nodes, **kwargs):
        """
            This method builds RKE cluster.yml from a template,
            and updates the list of nodes in update_nodes
        """
        print(template)
        render_dict = {
            'master_ssh_key_path': self.master_ssh_key_path,
            'network_plugin': DEFAULT_NETWORK_PLUGIN}
        render_dict.update(kwargs)  # will up master_key if passed in
        node_index = 0
        for node in nodes:
            node_dict = {
                'ssh_user_{}'.format(node_index): node.ssh_user,
                'ip_address_{}'.format(node_index): node.public_ip_address,
                'dns_hostname_{}'.format(node_index): node.host_name,
                'ssh_key_path_{}'.format(node_index): node.ssh_key_path,
                'ssh_key_{}'.format(node_index): node.ssh_key,
                'internal_address_{}'.format(node_index):
                    node.private_ip_address,
                'hostname_override_{}'.format(node_index):
                    node.node_name
            }
            render_dict.update(node_dict)
            node_index += 1
        yml_contents = jinja2.Environment(
            loader=jinja2.FileSystemLoader(self.template_path)
        ).get_template(template).render(render_dict)
        print("Generated cluster.yml contents:\n", yml_contents)
        nodes = self.update_nodes(yml_contents, nodes)
        return yml_contents, nodes

    @staticmethod
    def convert_to_dict(yml_contents):
        return load(yml_contents)

    def update_nodes(self, yml_contents, nodes):
        """
        This maps some rke logic for how the k8s nodes is configured to
        the nodes created by the cloud provider, so that the nodes list
        is the source of truth to validated against kubectl calls
        """
        yml_dict = self.convert_to_dict(yml_contents)
        for dict_node in yml_dict['nodes']:
            for node in nodes:
                if node.public_ip_address == dict_node['address'] or \
                        node.host_name == dict_node['address']:
                    # dep
                    node.host_name = dict_node['address']
                    if dict_node.get('hostname_override'):
                        node.node_name = dict_node['hostname_override']
                    else:
                        node.node_name = node.host_name
                    node.roles = dict_node['role']

                    # if internal_address is given, used to communicate
                    # this is the expected ip/value in nginx.conf
                    node.node_address = node.host_name
                    if dict_node.get('internal_address'):
                        node.node_address = dict_node['internal_address']
                    break
        return nodes

    def _save_cluster_yml(self, yml_name, yml_contents):
        file_path = "{}/{}".format(self._working_dir, yml_name)
        with open(file_path, 'w') as f:
            f.write(yml_contents)

    def get_kube_config_for_config(self, yml_name=DEFAULT_CONFIG_NAME):
        file_path = "{}/kube_config_{}".format(self._working_dir, yml_name)
        with open(file_path, 'r') as f:
            kube_config = f.read()
        return kube_config

    def kube_config_path(self, yml_name=DEFAULT_CONFIG_NAME):
        return os.path.abspath(
            "{}/kube_config_{}".format(self._working_dir, yml_name))

    def save_kube_config_locally(self, yml_name=DEFAULT_CONFIG_NAME):
        file_name = 'kube_config_{}'.format(yml_name)
        contents = self.get_kube_config_for_config(yml_name)
        with open(file_name, 'w') as f:
            f.write(contents)

    def run_command(self, command):
        return subprocess.check_output(command, shell=True, text=True)

    def run_command_with_stderr(self, command):
        try:
            output = subprocess.check_output(command, shell=True,
                                             stderr=subprocess.PIPE)
            returncode = 0
        except subprocess.CalledProcessError as e:
            output = e.output
            returncode = e.returncode
        print(returncode)
