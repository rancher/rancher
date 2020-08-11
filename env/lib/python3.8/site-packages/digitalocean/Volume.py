# -*- coding: utf-8 -*-
from .baseapi import BaseAPI, POST, DELETE
from .Snapshot import Snapshot

class Volume(BaseAPI):
    def __init__(self, *args, **kwargs):
        self.id = None
        self.name = None
        self.droplet_ids = []
        self.region = None
        self.description = None
        self.size_gigabytes = None
        self.created_at = None

        super(Volume, self).__init__(*args, **kwargs)

    @classmethod
    def get_object(cls, api_token, volume_id):
        """
        Class method that will return an Volume object by ID.
        """
        volume = cls(token=api_token, id=volume_id)
        volume.load()
        return volume

    def load(self):
        data = self.get_data("volumes/%s" % self.id)
        volume_dict = data['volume']

        # Setting the attribute values
        for attr in volume_dict.keys():
            setattr(self, attr, volume_dict[attr])

        return self

    def create(self, *args, **kwargs):
        """
        Creates a Block Storage volume

        Note: Every argument and parameter given to this method will be
        assigned to the object.

        Args:
            name: string - a name for the volume
            region: string - slug identifier for the region
            size_gigabytes: int - size of the Block Storage volume in GiB

        Optional Args:
            description: string - text field to describe a volume
        """
        data = self.get_data('volumes/',
                             type=POST,
                             params={'name': self.name,
                                     'region': self.region,
                                     'size_gigabytes': self.size_gigabytes,
                                     'description': self.description})

        if data:
            self.id = data['volume']['id']
            self.created_at = data['volume']['created_at']

        return self

    def destroy(self):
        """
            Destroy a volume
        """
        return self.get_data("volumes/%s/" % self.id, type=DELETE)

    def attach(self, droplet_id, region):
        """
        Attach a Volume to a Droplet.

        Args:
            droplet_id: int - droplet id
            region: string - slug identifier for the region
        """
        return self.get_data(
            "volumes/%s/actions/" % self.id,
            type=POST,
            params={"type": "attach",
                    "droplet_id": droplet_id,
                    "region": region}
        )

    def detach(self, droplet_id, region):
        """
        Detach a Volume to a Droplet.

        Args:
            droplet_id: int - droplet id
            region: string - slug identifier for the region
        """
        return self.get_data(
            "volumes/%s/actions/" % self.id,
            type=POST,
            params={"type": "detach",
                    "droplet_id": droplet_id,
                    "region": region}
        )

    def resize(self, size_gigabytes, region):
        """
        Detach a Volume to a Droplet.

        Args:
            size_gigabytes: int - size of the Block Storage volume in GiB
            region: string - slug identifier for the region
        """
        return self.get_data(
            "volumes/%s/actions/" % self.id,
            type=POST,
            params={"type": "resize",
                    "size_gigabytes": size_gigabytes,
                    "region": region}
        )

    def snapshot(self, name):
        """
        Create a snapshot of the volume.

        Args:
            name: string - a human-readable name for the snapshot
        """
        return self.get_data(
            "volumes/%s/snapshots/" % self.id,
            type=POST,
            params={"name": name}
        )

    def get_snapshots(self):
        """
        Retrieve the list of snapshots that have been created from a volume.

        Args:
        """
        data = self.get_data("volumes/%s/snapshots/" % self.id)
        snapshots = list()
        for jsond in data[u'snapshots']:
            snapshot = Snapshot(**jsond)
            snapshot.token = self.token
            snapshots.append(snapshot)

        return snapshots


    def __str__(self):
        return "<Volume: %s %s %s>" % (self.id, self.name, self.size_gigabytes)
