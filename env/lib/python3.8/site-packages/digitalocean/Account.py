# -*- coding: utf-8 -*-
from .baseapi import BaseAPI


class Account(BaseAPI):
    def __init__(self, *args, **kwargs):
        self.droplet_limit = None
        self.floating_ip_limit = None
        self.email = None
        self.uuid = None
        self.email_verified = None
        self.status = None
        self.status_message = None

        super(Account, self).__init__(*args, **kwargs)

    @classmethod
    def get_object(cls, api_token):
        """
            Class method that will return an Account object.
        """
        acct = cls(token=api_token)
        acct.load()
        return acct

    def load(self):
        # URL https://api.digitalocean.com/v2/account
        data = self.get_data("account/")
        account = data['account']

        for attr in account.keys():
            setattr(self, attr, account[attr])

    def __str__(self):
        return "%s" % (self.email)
