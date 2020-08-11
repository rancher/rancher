# -*- coding: utf-8 -*-
from .baseapi import BaseAPI, POST, DELETE, PUT


class Snapshot(BaseAPI):
    def __init__(self, *args, **kwargs):
        self.id = None
        self.name = None
        self.created_at = None
        self.regions = []
        self.resource_id = None
        self.resource_type = None
        self.min_disk_size = None
        self.size_gigabytes = None

        super(Snapshot, self).__init__(*args, **kwargs)

    @classmethod
    def get_object(cls, api_token, snapshot_id):
        """
            Class method that will return a Snapshot object by ID.
        """
        snapshot = cls(token=api_token, id=snapshot_id)
        snapshot.load()
        return snapshot

    def load(self):
        data = self.get_data("snapshots/%s" % self.id)
        snapshot_dict = data['snapshot']

        # Setting the attribute values
        for attr in snapshot_dict.keys():
            setattr(self, attr, snapshot_dict[attr])

        return self

    def destroy(self):
        """
            Destroy the image
        """
        return self.get_data("snapshots/%s/" % self.id, type=DELETE)

    def __str__(self):
        return "<Snapshot: %s %s>" % (self.id, self.name)
