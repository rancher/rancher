# -*- coding: utf-8 -*-
from .baseapi import BaseAPI


class Size(BaseAPI):
    def __init__(self, *args, **kwargs):
        self.slug = None
        self.memory = None
        self.vcpus = None
        self.disk = None
        self.transfer = None
        self.price_monthly = None
        self.price_hourly = None
        self.regions = []

        super(Size, self).__init__(*args, **kwargs)

    def __str__(self):
        return "%s" % (self.slug)
