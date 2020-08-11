# -*- coding: utf-8 -*-
import requests
try:
    from urlparse import urljoin
except ImportError:
    from urllib.parse import urljoin

from .baseapi import BaseAPI


class Metadata(BaseAPI):
    """
        Metadata API: Provide useful information about the current Droplet.
        See: https://developers.digitalocean.com/metadata/#introduction
    """
    droplet_id = None
    end_point = "http://169.254.169.254/metadata/v1"

    def __init__(self, *args, **kwargs):
        super(Metadata, self).__init__(*args, **kwargs)
        self.end_point = "http://169.254.169.254/metadata/v1"

    def get_data(self, url, headers=dict(), params=dict(), render_json=True):
        """
            Customized version of get_data to directly get the data without
            using the authentication method.
        """
        url = urljoin(self.end_point, url)

        response = requests.get(url, headers=headers, params=params,
                                timeout=self.get_timeout())

        if render_json:
            return response.json()
        return response.content

    def load(self):
        metadata = self.get_data("v1.json")

        for attr in metadata.keys():
            setattr(self, attr, metadata[attr])

        return self

    def __str__(self):
        return "<Metadata: %s>" % (self.droplet_id)
