# -*- coding: utf-8 -*-
from .baseapi import BaseAPI, POST, DELETE, PUT, NotFoundError

class Image(BaseAPI):
    def __init__(self, *args, **kwargs):
        self.id = None
        self.name = None
        self.distribution = None
        self.slug = None
        self.min_disk_size = None
        self.public = None
        self.regions = []
        self.created_at = None
        self.size_gigabytes = None

        super(Image, self).__init__(*args, **kwargs)

    @classmethod
    def get_object(cls, api_token, image_id_or_slug):
        """
            Class method that will return an Image object by ID or slug.

            This method is used to validate the type of the image. If it is a
            number, it will be considered as an Image ID, instead if it is a
            string, it will considered as slug.
        """
        if cls._is_string(image_id_or_slug):
            image = cls(token=api_token, slug=image_id_or_slug)
            image.load(use_slug=True)
        else:
            image = cls(token=api_token, id=image_id_or_slug)
            image.load()
        return image

    @staticmethod
    def _is_string(value):
        """
            Checks if the value provided is a string (True) or not integer
            (False) or something else (None).
        """
        if type(value) in [type(u''), type('')]:
            return True
        elif type(value) in [int, type(2 ** 64)]:
            return False
        else:
            return None

    def load(self, use_slug=False):
        """
            Load slug.

            Loads by id, or by slug if id is not present or use slug is True.
        """
        identifier = None
        if use_slug or not self.id:
            identifier = self.slug
        else:
            identifier = self.id
        if not identifier:
            raise NotFoundError("One of self.id or self.slug must be set.")
        data = self.get_data("images/%s" % identifier)
        image_dict = data['image']

        # Setting the attribute values
        for attr in image_dict.keys():
            setattr(self, attr, image_dict[attr])

        return self

    def destroy(self):
        """
            Destroy the image
        """
        return self.get_data("images/%s/" % self.id, type=DELETE)

    def transfer(self, new_region_slug):
        """
            Transfer the image
        """
        return self.get_data(
            "images/%s/actions/" % self.id,
            type=POST,
            params={"type": "transfer", "region": new_region_slug}
        )

    def rename(self, new_name):
        """
            Rename an image
        """
        return self.get_data(
            "images/%s" % self.id,
            type=PUT,
            params={"name": new_name}
        )

    def __str__(self):
        return "<Image: %s %s %s>" % (self.id, self.distribution, self.name)
