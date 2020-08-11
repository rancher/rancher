# -*- coding: utf-8 -*-
import re

from .Action import Action
from .Image import Image
from .Kernel import Kernel
from .baseapi import BaseAPI, Error, GET, POST, DELETE
from .SSHKey import SSHKey


class DropletError(Error):
    """Base exception class for this module"""
    pass


class BadKernelObject(DropletError):
    pass


class BadSSHKeyFormat(DropletError):
    pass


class Droplet(BaseAPI):
    """"Droplet management

    Attributes accepted at creation time:

    Args:
        name (str): name
        size_slug (str): droplet size
        image (str): image name to use to create droplet
        region (str): region
        ssh_keys: (:obj:`str`, optional): list of ssh keys
        backups (bool): True if backups enabled
        ipv6 (bool): True if ipv6 enabled
        private_networking (bool): True if private networking enabled
        user_data (str): arbitrary data to pass to droplet
        volumes (:obj:`str`, optional): list of blockstorage volumes
        monitoring: (bool) - True if installing the DigitalOcean monitoring agent

    Attributes returned by API:
        id (int): droplet id
        memory (str): memory size
        vcpus (int): number of vcpus
        disk (int): disk size in GB
        status (str): status
        locked (bool): True if locked
        created_at (str): creation date in format u'2014-11-06T10:42:09Z'
        status (str): status, e.g. 'new', 'active', etc
        networks (dict): details of connected networks
        kernel (dict): details of kernel
        backup_ids (:obj:`int`, optional): list of ids of backups of this droplet
        snapshot_ids (:obj:`int`, optional): list of ids of snapshots of this droplet
        action_ids (:obj:`int`, optional): list of ids of actions
        features (:obj:`str`, optional): list of enabled features. e.g.
                  [u'private_networking', u'virtio']
        image (dict): details of image used to create this droplet
        ip_address (str): public ip addresses
        private_ip_address (str): private ip address
        ip_v6_address (:obj:`str`, optional): list of ipv6 addresses assigned
        end_point (str): url of api endpoint used
        volume_ids (:obj:`str`, optional): list of blockstorage volumes
    """

    def __init__(self, *args, **kwargs):
        # Defining default values
        self.id = None
        self.name = None
        self.memory = None
        self.vcpus = None
        self.disk = None
        self.region = []
        self.status = None
        self.image = None
        self.size_slug = None
        self.locked = None
        self.created_at = None
        self.status = None
        self.networks = []
        self.kernel = None
        self.backup_ids = []
        self.snapshot_ids = []
        self.action_ids = []
        self.features = []
        self.ip_address = None
        self.private_ip_address = None
        self.ip_v6_address = None
        self.ssh_keys = []
        self.backups = None
        self.ipv6 = None
        self.private_networking = None
        self.user_data = None
        self.volumes = []
        self.tags = []
        self.monitoring = None

        # This will load also the values passed
        super(Droplet, self).__init__(*args, **kwargs)

    @classmethod
    def get_object(cls, api_token, droplet_id):
        """Class method that will return a Droplet object by ID.

        Args:
            api_token (str): token
            droplet_id (int): droplet id
        """
        droplet = cls(token=api_token, id=droplet_id)
        droplet.load()
        return droplet

    @classmethod
    def create_multiple(*args, **kwargs):
        api = BaseAPI(token=kwargs.get("token"))

        data = {
            "names": kwargs.get("names"),
            "size": kwargs.get("size_slug") or kwargs.get("size"),
            "image": kwargs.get("image"),
            "region": kwargs.get("region"),
            "backups": bool(kwargs.get("backups")),
            "ipv6": bool(kwargs.get("ipv6")),
            "private_networking": bool(kwargs.get("private_networking")),
            "tags": kwargs.get("tags"),
            "monitoring": bool(kwargs.get("monitoring")),
        }

        if kwargs.get("ssh_keys"):
            data["ssh_keys"] = Droplet.__get_ssh_keys_id_or_fingerprint(
                    kwargs["ssh_keys"], kwargs.get("token"),
                    kwargs["names"][0])

        if kwargs.get("user_data"):
            data["user_data"] = kwargs["user_data"]

        droplets = []

        data = api.get_data("droplets/", type=POST, params=data)

        if data:
            action_ids = [data["links"]["actions"][0]["id"]]
            for droplet_json in data["droplets"]:
                droplet_json["token"] = kwargs["token"]
                droplet = Droplet(**droplet_json)
                droplet.action_ids = action_ids
                droplets.append(droplet)

        return droplets

    def __check_actions_in_data(self, data):
        # reloading actions if actions is provided.
        if u"actions" in data:
            self.action_ids = []
            for action in data[u'actions']:
                self.action_ids.append(action[u'id'])

    def get_data(self, *args, **kwargs):
        """
            Customized version of get_data to perform __check_actions_in_data
        """
        data = super(Droplet, self).get_data(*args, **kwargs)
        if "type" in kwargs:
            if kwargs["type"] == POST:
                self.__check_actions_in_data(data)
        return data

    def load(self):
        """
           Fetch data about droplet - use this instead of get_data()
        """
        droplets = self.get_data("droplets/%s" % self.id)
        droplet = droplets['droplet']

        for attr in droplet.keys():
            setattr(self, attr, droplet[attr])

        for net in self.networks['v4']:
            if net['type'] == 'private':
                self.private_ip_address = net['ip_address']
            if net['type'] == 'public':
                self.ip_address = net['ip_address']
        if self.networks['v6']:
            self.ip_v6_address = self.networks['v6'][0]['ip_address']

            if "backups" in self.features:
                self.backups = True
            else:
                self.backups = False
            if "ipv6" in self.features:
                self.ipv6 = True
            else:
                self.ipv6 = False
            if "private_networking" in self.features:
                self.private_networking = True
            else:
                self.private_networking = False

        if "tags" in droplets:
            self.tags = droplets["tags"]

        return self

    def _perform_action(self, params, return_dict=True):
        """
            Perform a droplet action.

            Args:
                params (dict): parameters of the action

            Optional Args:
                return_dict (bool): Return a dict when True (default),
                    otherwise return an Action.

            Returns dict or Action
        """
        action = self.get_data(
            "droplets/%s/actions/" % self.id,
            type=POST,
            params=params
        )
        if return_dict:
            return action
        else:
            action = action[u'action']
            return_action = Action(token=self.token)
            # Loading attributes
            for attr in action.keys():
                setattr(return_action, attr, action[attr])
            return return_action

    def power_on(self, return_dict=True):
        """
            Boot up the droplet

            Optional Args:
                return_dict (bool): Return a dict when True (default),
                    otherwise return an Action.

            Returns dict or Action
        """
        return self._perform_action({'type': 'power_on'}, return_dict)

    def shutdown(self, return_dict=True):
        """
            shutdown the droplet

            Optional Args:
                return_dict (bool): Return a dict when True (default),
                    otherwise return an Action.

            Returns dict or Action
        """
        return self._perform_action({'type': 'shutdown'}, return_dict)

    def reboot(self, return_dict=True):
        """
            restart the droplet

            Optional Args:
                return_dict (bool): Return a dict when True (default),
                    otherwise return an Action.

            Returns dict or Action
        """
        return self._perform_action({'type': 'reboot'}, return_dict)

    def power_cycle(self, return_dict=True):
        """
            restart the droplet

            Optional Args:
                return_dict (bool): Return a dict when True (default),
                    otherwise return an Action.

            Returns dict or Action
        """
        return self._perform_action({'type': 'power_cycle'}, return_dict)

    def power_off(self, return_dict=True):
        """
            restart the droplet

            Optional Args:
                return_dict (bool): Return a dict when True (default),
                    otherwise return an Action.

            Returns dict or Action
        """
        return self._perform_action({'type': 'power_off'}, return_dict)

    def reset_root_password(self, return_dict=True):
        """
            reset the root password

            Optional Args:
                return_dict (bool): Return a dict when True (default),
                    otherwise return an Action.

            Returns dict or Action
        """
        return self._perform_action({'type': 'password_reset'}, return_dict)

    def resize(self, new_size_slug, return_dict=True, disk=True):
        """Resize the droplet to a new size slug.
        https://developers.digitalocean.com/documentation/v2/#resize-a-droplet

        Args:
            new_size_slug (str): name of new size

        Optional Args:
            return_dict (bool): Return a dict when True (default),
                                otherwise return an Action.
            disk (bool): If a permanent resize, with disk changes included.

        Returns dict or Action
        """
        options = {"type": "resize", "size": new_size_slug}
        if disk: options["disk"] = "true"

        return self._perform_action(options, return_dict)

    def take_snapshot(self, snapshot_name, return_dict=True, power_off=False):
        """Take a snapshot!

        Args:
            snapshot_name (str): name of snapshot

        Optional Args:
            return_dict (bool): Return a dict when True (default),
                otherwise return an Action.
            power_off (bool): Before taking the snapshot the droplet will be
                turned off with another API call. It will wait until the
                droplet will be powered off.

        Returns dict or Action
        """
        if power_off is True and self.status != "off":
            action = self.power_off(return_dict=False)
            action.wait()
            self.load()

        return self._perform_action(
            {"type": "snapshot", "name": snapshot_name},
            return_dict
        )

    def restore(self, image_id, return_dict=True):
        """Restore the droplet to an image ( snapshot or backup )

        Args:
            image_id (int): id of image

        Optional Args:
            return_dict (bool): Return a dict when True (default),
                otherwise return an Action.

        Returns dict or Action
        """
        return self._perform_action(
            {"type": "restore", "image": image_id},
            return_dict
        )

    def rebuild(self, image_id=None, return_dict=True):
        """Restore the droplet to an image ( snapshot or backup )

        Args:
            image_id (int): id of image

        Optional Args:
            return_dict (bool): Return a dict when True (default),
                otherwise return an Action.

        Returns dict or Action
        """
        if not image_id:
            image_id = self.image['id']

        return self._perform_action(
            {"type": "rebuild", "image": image_id},
            return_dict
        )

    def enable_backups(self, return_dict=True):
        """
            Enable automatic backups

            Optional Args:
                return_dict (bool): Return a dict when True (default),
                    otherwise return an Action.

            Returns dict or Action
        """
        return self._perform_action({'type': 'enable_backups'}, return_dict)

    def disable_backups(self, return_dict=True):
        """
            Disable automatic backups

            Optional Args:
                return_dict (bool): Return a dict when True (default),
                    otherwise return an Action.

            Returns dict or Action
        """
        return self._perform_action({'type': 'disable_backups'}, return_dict)

    def destroy(self):
        """
            Destroy the droplet

            Returns dict
        """
        return self.get_data("droplets/%s" % self.id, type=DELETE)

    def rename(self, name, return_dict=True):
        """Rename the droplet

        Args:
            name (str): new name

        Optional Args:
            return_dict (bool): Return a dict when True (default),
                otherwise return an Action.

        Returns dict or Action
        """
        return self._perform_action(
            {'type': 'rename', 'name': name},
            return_dict
        )

    def enable_private_networking(self, return_dict=True):
        """
           Enable private networking on an existing Droplet where available.

           Optional Args:
               return_dict (bool): Return a dict when True (default),
                   otherwise return an Action.

           Returns dict or Action
        """
        return self._perform_action(
            {'type': 'enable_private_networking'},
            return_dict
        )

    def enable_ipv6(self, return_dict=True):
        """
            Enable IPv6 on an existing Droplet where available.

            Optional Args:
                return_dict (bool): Return a dict when True (default),
                    otherwise return an Action.

            Returns dict or Action
        """
        return self._perform_action({'type': 'enable_ipv6'}, return_dict)

    def change_kernel(self, kernel, return_dict=True):
        """Change the kernel to a new one

        Args:
            kernel : instance of digitalocean.Kernel.Kernel

        Optional Args:
            return_dict (bool): Return a dict when True (default),
                otherwise return an Action.

        Returns dict or Action
        """
        if type(kernel) != Kernel:
            raise BadKernelObject("Use Kernel object")

        return self._perform_action(
            {'type': 'change_kernel', 'kernel': kernel.id},
            return_dict
        )

    @staticmethod
    def __get_ssh_keys_id_or_fingerprint(ssh_keys, token, name):
        """
            Check and return a list of SSH key IDs or fingerprints according
            to DigitalOcean's API. This method is used to check and create a
            droplet with the correct SSH keys.
        """
        ssh_keys_id = list()
        for ssh_key in ssh_keys:
            if type(ssh_key) in [int, type(2 ** 64)]:
                ssh_keys_id.append(int(ssh_key))

            elif type(ssh_key) == SSHKey:
                ssh_keys_id.append(ssh_key.id)

            elif type(ssh_key) in [type(u''), type('')]:
                # ssh_key could either be a fingerprint or a public key
                #
                # type(u'') and type('') is the same in python 3 but
                # different in 2. See:
                # https://github.com/koalalorenzo/python-digitalocean/issues/80
                regexp_of_fingerprint = '([0-9a-fA-F]{2}:){15}[0-9a-fA-F]'
                match = re.match(regexp_of_fingerprint, ssh_key)

                if match is not None and match.end() == len(ssh_key) - 1:
                    ssh_keys_id.append(ssh_key)

                else:
                    key = SSHKey()
                    key.token = token
                    results = key.load_by_pub_key(ssh_key)

                    if results is None:
                        key.public_key = ssh_key
                        key.name = "SSH Key %s" % name
                        key.create()
                    else:
                        key = results

                    ssh_keys_id.append(key.id)
            else:
                raise BadSSHKeyFormat(
                    "Droplet.ssh_keys should be a list of IDs, public keys"
                    + " or fingerprints."
                )

        return ssh_keys_id

    def create(self, *args, **kwargs):
        """
            Create the droplet with object properties.

            Note: Every argument and parameter given to this method will be
            assigned to the object.
        """
        for attr in kwargs.keys():
            setattr(self, attr, kwargs[attr])

        # Provide backwards compatibility
        if not self.size_slug and self.size:
            self.size_slug = self.size

        ssh_keys_id = Droplet.__get_ssh_keys_id_or_fingerprint(self.ssh_keys,
                                                               self.token,
                                                               self.name)

        data = {
            "name": self.name,
            "size": self.size_slug,
            "image": self.image,
            "region": self.region,
            "ssh_keys": ssh_keys_id,
            "backups": bool(self.backups),
            "ipv6": bool(self.ipv6),
            "private_networking": bool(self.private_networking),
            "volumes": self.volumes,
            "tags": self.tags,
            "monitoring": bool(self.monitoring),
        }

        if self.user_data:
            data["user_data"] = self.user_data

        data = self.get_data("droplets/", type=POST, params=data)

        if data:
            self.id = data['droplet']['id']
            action_id = data['links']['actions'][0]['id']
            self.action_ids = []
            self.action_ids.append(action_id)

    def get_events(self):
        """
            A helper function for backwards compatibility.
            Calls get_actions()
        """
        return self.get_actions()

    def get_actions(self):
        """
            Returns a list of Action objects
            This actions can be used to check the droplet's status
        """
        answer = self.get_data("droplets/%s/actions/" % self.id, type=GET)

        actions = []
        for action_dict in answer['actions']:
            action = Action(**action_dict)
            action.token = self.token
            action.droplet_id = self.id
            action.load()
            actions.append(action)
        return actions

    def get_action(self, action_id):
        """Returns a specific Action by its ID.

        Args:
            action_id (int): id of action
        """
        return Action.get_object(
            api_token=self.token,
            action_id=action_id
        )

    def get_snapshots(self):
        """
            This method will return the snapshots/images connected to that
            specific droplet.
        """
        snapshots = list()
        for id in self.snapshot_ids:
            snapshot = Image()
            snapshot.id = id
            snapshot.token = self.token
            snapshots.append(snapshot)
        return snapshots

    def get_kernel_available(self):
        """
            Get a list of kernels available
        """

        kernels = list()
        data = self.get_data("droplets/%s/kernels/" % self.id)
        while True:
            for jsond in data[u'kernels']:
                kernel = Kernel(**jsond)
                kernel.token = self.token
                kernels.append(kernel)
            try:
                url = data[u'links'][u'pages'].get(u'next')
                if not url:
                        break
                data = self.get_data(url)
            except KeyError:  # No links.
                break

        return kernels

    def __str__(self):
        return "<Droplet: %s %s>" % (self.id, self.name)
