# -*- coding: utf-8 -*-
from .baseapi import BaseAPI, GET, POST, DELETE


class FloatingIP(BaseAPI):
    def __init__(self, *args, **kwargs):
        self.ip = None
        self.droplet = []
        self.region = []

        super(FloatingIP, self).__init__(*args, **kwargs)

    @classmethod
    def get_object(cls, api_token, ip):
        """
            Class method that will return a FloatingIP object by its IP.

            Args:
                api_token: str - token
                ip: str - floating ip address
        """
        floating_ip = cls(token=api_token, ip=ip)
        floating_ip.load()
        return floating_ip

    def load(self):
        """
            Load the FloatingIP object from DigitalOcean.

            Requires self.ip to be set.
        """
        data = self.get_data('floating_ips/%s' % self.ip, type=GET)
        floating_ip = data['floating_ip']

        # Setting the attribute values
        for attr in floating_ip.keys():
            setattr(self, attr, floating_ip[attr])

        return self

    def create(self, *args, **kwargs):
        """
            Creates a FloatingIP and assigns it to a Droplet.

            Note: Every argument and parameter given to this method will be
            assigned to the object.

            Args:
                droplet_id: int - droplet id
        """
        data = self.get_data('floating_ips/',
                             type=POST,
                             params={'droplet_id': self.droplet_id})

        if data:
            self.ip = data['floating_ip']['ip']
            self.region = data['floating_ip']['region']

        return self

    def reserve(self, *args, **kwargs):
        """
            Creates a FloatingIP in a region without assigning
            it to a specific Droplet.

            Note: Every argument and parameter given to this method will be
            assigned to the object.

            Args:
                region_slug: str - region's slug (e.g. 'nyc3')
        """
        data = self.get_data('floating_ips/',
                             type=POST,
                             params={'region': self.region_slug})

        if data:
            self.ip = data['floating_ip']['ip']
            self.region = data['floating_ip']['region']

        return self

    def destroy(self):
        """
            Destroy the FloatingIP
        """
        return self.get_data('floating_ips/%s/' % self.ip, type=DELETE)

    def assign(self, droplet_id):
        """
            Assign a FloatingIP to a Droplet.

            Args:
                droplet_id: int - droplet id
        """
        return self.get_data(
            "floating_ips/%s/actions/" % self.ip,
            type=POST,
            params={"type": "assign", "droplet_id": droplet_id}
        )

    def unassign(self):
        """
            Unassign a FloatingIP.
        """
        return self.get_data(
            "floating_ips/%s/actions/" % self.ip,
            type=POST,
            params={"type": "unassign"}
        )

    def __str__(self):
        return "%s" % (self.ip)
