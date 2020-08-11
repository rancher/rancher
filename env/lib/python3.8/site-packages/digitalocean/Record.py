# -*- coding: utf-8 -*-
from .baseapi import BaseAPI, POST, DELETE, PUT


class Record(BaseAPI):
    """
    An object representing an DigitalOcean Domain Record.

    Args:
        type (str): The type of the DNS record (e.g. A, CNAME, TXT).
        name (str): The host name, alias, or service being defined by the
            record.
        data (int): Variable data depending on record type.
        priority (int): The priority for SRV and MX records.
        port (int): The port for SRV records.
        ttl (int): The time to live for the record, in seconds.
        weight (int): The weight for SRV records.
        flags (int): An unsigned integer between 0-255 used for CAA records.
        tags (string): The parameter tag for CAA records. Valid values are
            "issue", "wildissue", or "iodef"
    """

    def __init__(self, domain_name=None, *args, **kwargs):
        self.domain = domain_name if domain_name else ""
        self.id = None
        self.type = None
        self.name = None
        self.data = None
        self.priority = None
        self.port = None
        self.ttl = None
        self.weight = None
        self.flags = None
        self.tags = None

        super(Record, self).__init__(*args, **kwargs)

    @classmethod
    def get_object(cls, api_token, domain, record_id):
        """
            Class method that will return a Record object by ID and the domain.
        """
        record = cls(token=api_token, domain=domain, id=record_id)
        record.load()
        return record

    def create(self):
        """
        Creates a new record for a domain.

        Args:
            type (str): The type of the DNS record (e.g. A, CNAME, TXT).
            name (str): The host name, alias, or service being defined by the
                record.
            data (int): Variable data depending on record type.
            priority (int): The priority for SRV and MX records.
            port (int): The port for SRV records.
            ttl (int): The time to live for the record, in seconds.
            weight (int): The weight for SRV records.
            flags (int): An unsigned integer between 0-255 used for CAA records.
            tags (string): The parameter tag for CAA records. Valid values are
                "issue", "wildissue", or "iodef"
        """
        input_params = {
            "type": self.type,
            "data": self.data,
            "name": self.name,
            "priority": self.priority,
            "port": self.port,
            "ttl": self.ttl,
            "weight": self.weight,
            "flags": self.flags,
            "tags": self.tags
        }

        data = self.get_data(
            "domains/%s/records" % (self.domain),
            type=POST,
            params=input_params,
        )

        if data:
            self.id = data['domain_record']['id']

    def destroy(self):
        """
            Destroy the record
        """
        return self.get_data(
            "domains/%s/records/%s" % (self.domain, self.id),
            type=DELETE,
        )

    def save(self):
        """
            Save existing record
        """
        data = {
            "type": self.type,
            "data": self.data,
            "name": self.name,
            "priority": self.priority,
            "port": self.port,
            "ttl": self.ttl,
            "weight": self.weight,
            "flags": self.flags,
            "tags": self.tags
        }
        return self.get_data(
            "domains/%s/records/%s" % (self.domain, self.id),
            type=PUT,
            params=data
        )

    def load(self):
        url = "domains/%s/records/%s" % (self.domain, self.id)
        record = self.get_data(url)
        if record:
            record = record[u'domain_record']

            # Setting the attribute values
            for attr in record.keys():
                setattr(self, attr, record[attr])

    def __str__(self):
        return "<Record: %s %s>" % (self.id, self.domain)
