import abc
import os
from invoke import run


class CloudProviderBase(object, metaclass=abc.ABCMeta):
    DOCKER_VERSION = os.environ.get("DOCKER_VERSION", '19.03')
    DOCKER_INSTALLED = os.environ.get("DOCKER_INSTALLED", "false")

    @abc.abstractmethod
    def create_node(self, node_name, wait_for_ready=False):
        raise NotImplementedError

    @abc.abstractmethod
    def stop_node(self, node, wait_for_stop=False):
        raise NotImplementedError

    @abc.abstractmethod
    def delete_node(self, wait_for_delete=False):
        raise NotImplementedError

    @abc.abstractmethod
    def import_ssh_key(self, ssh_key_name, public_ssh_key):
        raise NotImplementedError

    @abc.abstractmethod
    def delete_ssh_key(self, ssh_key_name):
        raise NotImplementedError

    def save_master_key(self, ssh_key_name, ssh_key):
        if not os.path.isfile('.ssh/{}'.format(ssh_key_name)):
            run('mkdir -p .ssh')
            with open('.ssh/{}'.format(ssh_key_name), 'w') as f:
                f.write(ssh_key)
            run("chmod 0600 .ssh/{0}".format(ssh_key_name))
            run("cat .ssh/{}".format(ssh_key_name))

    def generate_ssh_key(self, ssh_key_name, ssh_key_passphrase=''):
        try:
            if not os.path.isfile('.ssh/{}'.format(ssh_key_name)):
                run('mkdir -p .ssh && rm -rf .ssh/{}'.format(ssh_key_name))
                run("ssh-keygen -N '{1}' -C '{0}' -f .ssh/{0}".format(
                    ssh_key_name, ssh_key_passphrase))
                run("chmod 0600 .ssh/{0}".format(ssh_key_name))

            public_ssh_key = self.get_ssh_key(ssh_key_name + '.pub')
        except Exception as e:
            raise Exception("Failed to generate ssh key: {0}".format(e))
        return public_ssh_key

    def get_ssh_key(self, ssh_key_name):
        with open(self.get_ssh_key_path(ssh_key_name), 'r') as f:
            ssh_key = f.read()
        return ssh_key

    def get_ssh_key_path(self, ssh_key_name):
        key_path = os.path.abspath('.ssh/{}'.format(ssh_key_name))
        return key_path
