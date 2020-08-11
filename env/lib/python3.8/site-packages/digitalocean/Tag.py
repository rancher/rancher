from .baseapi import BaseAPI
from .Droplet import Droplet

class Tag(BaseAPI):
    def __init__(self, *args, **kwargs):
        self.name = ""
        self.resources = {}
        super(Tag, self).__init__(*args, **kwargs)


    @classmethod
    def get_object(cls, api_token, tag_name):
        tag = cls(token=api_token, name=tag_name)
        tag.load()
        return tag


    def load(self):
        """
           Fetch data about tag
        """
        tags = self.get_data("tags/%s" % self.name)
        tag = tags['tag']

        for attr in tag.keys():
            setattr(self, attr, tag[attr])

        return self


    def create(self, **kwargs):
        """
            Create the tag.
        """
        for attr in kwargs.keys():
            setattr(self, attr, kwargs[attr])

        params = {"name": self.name}

        output = self.get_data("tags/", type="POST", params=params)
        if output:
            self.name = output['tag']['name']
            self.resources = output['tag']['resources']


    def delete(self):
        return self.get_data("tags/%s" % self.name, type="DELETE")


    def __get_resources(self, resources, method):

        """ Method used to talk directly to the API (TAGs' Resources) """
        tagged = self.get_data(
            'tags/%s/resources' % self.name, params={
                "resources": resources
            },
            type=method,
        )
        return tagged


    def __add_resources(self, resources):
        """
            Add to the resources to this tag.

            Attributes accepted at creation time:
                resources: array - See API.
        """
        return self.__get_resources(resources, method='POST')


    def __remove_resources(self, resources):
        """
            Remove resources from this tag.

            Attributes accepted at creation time:
                resources: array - See API.
        """
        return self.__get_resources(resources, method='DELETE')


    def __extract_resources_from_droplets(self, data):
        """
            Private method to extract from a value, the resources.
            It will check the type of object in the array provided and build
            the right structure for the API.
        """
        resources = []
        if not isinstance(data, list): return data
        for a_droplet in data:
            res = {}

            if isinstance(a_droplet, str) or isinstance(a_droplet, unicode) or isinstance(a_droplet, int):
                res = {"resource_id": a_droplet, "resource_type": "droplet"}
            elif isinstance(a_droplet, Droplet):
                res = {"resource_id": a_droplet.id, "resource_type": "droplet"}

            if len(res) > 0:
                resources.append(res)

        return resources


    def add_droplets(self, droplet):
        """
            Add the Tag to a Droplet.

            Attributes accepted at creation time:
                droplet: array of string or array of int, or array of Droplets.
        """
        droplets = droplet
        if not isinstance(droplets, list):
            droplets = [droplet]

        # Extracting data from the Droplet object
        resources = self.__extract_resources_from_droplets(droplets)
        if len(resources) > 0:
            return self.__add_resources(resources)

        return False


    def remove_droplets(self, droplet):
        """
            Remove the Tag from the Droplet.

            Attributes accepted at creation time:
                droplet: array of string or array of int, or array of Droplets.
        """
        droplets = droplet
        if not isinstance(droplets, list):
            droplets = [droplet]

        # Extracting data from the Droplet object
        resources = self.__extract_resources_from_droplets(droplets)
        if len(resources) > 0:
            return self.__remove_resources(resources)

        return False
