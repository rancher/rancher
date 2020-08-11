# -*- coding: utf-8 -*-
from .baseapi import BaseAPI, GET, POST, DELETE, PUT


class SSHKey(BaseAPI):
    def __init__(self, *args, **kwargs):
        self.id = ""
        self.name = None
        self.public_key = None
        self.fingerprint = None

        super(SSHKey, self).__init__(*args, **kwargs)

    @classmethod
    def get_object(cls, api_token, ssh_key_id):
        """
            Class method that will return a SSHKey object by ID.
        """
        ssh_key = cls(token=api_token, id=ssh_key_id)
        ssh_key.load()
        return ssh_key

    def load(self):
        """
            Load the SSHKey object from DigitalOcean.

            Requires either self.id or self.fingerprint to be set.
        """
        identifier = None
        if self.id:
            identifier = self.id
        elif self.fingerprint is not None:
            identifier = self.fingerprint

        data = self.get_data("account/keys/%s" % identifier, type=GET)

        ssh_key = data['ssh_key']

        # Setting the attribute values
        for attr in ssh_key.keys():
            setattr(self, attr, ssh_key[attr])
        self.id = ssh_key['id']

    def load_by_pub_key(self, public_key):
        """
            This method will laod a SSHKey object from DigitalOcean
            from a public_key. This method will avoid problem like
            uploading the same public_key twice.
        """

        data = self.get_data("account/keys/")
        for jsoned in data['ssh_keys']:
            if jsoned.get('public_key', "") == public_key:
                self.id = jsoned['id']
                self.load()
                return self
        return None

    def create(self):
        """
            Create the SSH Key
        """
        input_params = {
            "name": self.name,
            "public_key": self.public_key,
        }

        data = self.get_data("account/keys/", type=POST, params=input_params)

        if data:
            self.id = data['ssh_key']['id']

    def edit(self):
        """
            Edit the SSH Key
        """
        input_params = {
            "name": self.name,
            "public_key": self.public_key,
        }

        data = self.get_data(
            "account/keys/%s" % self.id,
            type=PUT,
            params=input_params
        )

        if data:
            self.id = data['ssh_key']['id']

    def destroy(self):
        """
            Destroy the SSH Key
        """
        return self.get_data("account/keys/%s" % self.id, type=DELETE)

    def __str__(self):
        return "<SSHKey: %s %s>" % (self.id, self.name)
