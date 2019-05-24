import digitalocean
import os
import time

from .cloud_provider import CloudProviderBase
from .node import Node


PRIVATE_IMAGES = {
    "ubuntu-16.04-docker-1.12.6": {
        'image': 30447985, 'ssh_user': 'ubuntu'},
    "ubuntu-16.04-docker-17.03": {
        'image': 30473722, 'ssh_user': 'ubuntu'},
    "ubuntu-16.04-docker-1.13.1": {
        'image': 30473815, 'ssh_user': 'ubuntu'}}

DO_TOKEN = os.environ.get("DO_TOKEN")
DO_SSH_KEY_ID = os.environ.get("DO_SSH_KEY_ID")
DO_SSH_KEY_NAME = os.environ.get("DO_SSH_KEY_NAME")


class DigitalOcean(CloudProviderBase):
    DROPLET_STATE_MAP = {
        'running': 'create',
        'stopped': 'shutdown',
        'terminated': 'destroy'
    }

    def __init__(self):
        self._manager = digitalocean.Manager(token=DO_TOKEN)
        self._token = DO_TOKEN

        if DO_SSH_KEY_NAME:
            self.master_ssh_private_key = self.get_ssh_key(DO_SSH_KEY_NAME)
            self.master_ssh_public_key = self.get_ssh_key(
                DO_SSH_KEY_NAME + '.pub')
            self.master_ssh_private_key_path = self.get_ssh_key_path(
                DO_SSH_KEY_NAME)

    def _select_ami(self, os_version=None, docker_version=None):
        os_version = os_version or self.OS_VERSION
        docker_version = docker_version or self.DOCKER_VERSION
        image = PRIVATE_IMAGES[
            "{}-docker-{}".format(os_version, docker_version)]
        return image['image'], image['ssh_user']

    def create_node(
        self, node_name, key_name=None, os_version=None, docker_version=None,
            wait_for_ready=True):

        os_version = os_version or self.OS_VERSION
        docker_version = docker_version or self.DOCKER_VERSION
        image, ssh_user = self._select_ami(os_version, docker_version)

        if key_name:
            # get private key
            ssh_private_key_name = key_name.replace('.pub', '')
            ssh_private_key = self.get_ssh_key(ssh_private_key_name)
            ssh_private_key_path = self.get_ssh_key_path(ssh_private_key_name)

        ssh_key_id = self._get_ssh_key_id(key_name)

        droplet = digitalocean.Droplet(
            token=self._token,
            name=node_name,
            region='sfo1',
            image=image,
            size_slug='2gb',
            ssh_keys=[ssh_key_id],
            backups=False)
        droplet.create()

        node = Node(
            provider_node_id=droplet.id,
            state=droplet.status,
            ssh_user=ssh_user,
            ssh_key_name=ssh_private_key_name,
            ssh_key_path=ssh_private_key_path,
            ssh_key=ssh_private_key,
            os_version=os_version,
            docker_version=docker_version)

        if wait_for_ready:
            self.wait_for_node_state(node)
            node.wait_for_ssh_ready()
        return node

    def create_multiple_nodes(
        self, number_of_nodes, node_name_prefix, os_version=None,
            docker_version=None, key_name=None, wait_for_ready=True):

        nodes = []
        for i in range(number_of_nodes):
            node_name = "{}_{}".format(node_name_prefix, i)
            nodes.append(self.create_node(
                node_name, key_name=key_name, os_version=os_version,
                docker_version=docker_version, wait_for_ready=False))

        if wait_for_ready:
            nodes = self.wait_for_nodes_state(nodes)
            for node in nodes:
                node.wait_for_ssh_ready()
        return nodes

    def get_node(self, provider_id):
        droplet = self._manager.get_droplet(provider_id)
        node = Node(
            provider_node_id=droplet.id,
            node_name=droplet.name,
            ip_address=droplet.ip_address,
            state=droplet.status,
            labels=droplet.tags)
        return node

    def stop_node(self, node, wait_for_stopped=False):
        droplet = self._manager.get_droplet(node.provider_node_id)
        droplet.shutdown()

        if wait_for_stopped:
            self.wait_for_node_state(node, 'stopped')

    def delete_node(self, node, wait_for_deleted=False):
        droplet = self._manager.get_droplet(node.provider_node_id)
        droplet.destroy()

        if wait_for_deleted:
            self.wait_for_node_state(node, 'terminated')

    def wait_for_node_state(self, node, state='running'):
        action_type = self.DROPLET_STATE_MAP[state]
        droplet = self._manager.get_droplet(node.provider_node_id)
        actions = droplet.get_actions()

        action = None
        for item in actions:
            if item.type == action_type:
                action = item
                break
        else:
            raise Exception(
                "Unable to find action for {0}: {1}".format(
                    state, action_type))

        timeout = 300
        start_time = start_time = time.time()
        while action.status != "completed":
            if time.time() - start_time > timeout:
                raise Exception("{0} node timed out".format(state))
            action.load()

        if action_type == "create":
            droplet.load()
            node.host_name = droplet.name
            node.ip_address = droplet.ip_address
            node.labels = droplet.tags
        node.state = action_type

    def _get_ssh_key_id(self, key_name):
        pass

