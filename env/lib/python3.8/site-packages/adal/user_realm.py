#------------------------------------------------------------------------------
#
# Copyright (c) Microsoft Corporation. 
# All rights reserved.
# 
# This code is licensed under the MIT License.
# 
# Permission is hereby granted, free of charge, to any person obtaining a copy
# of this software and associated documentation files(the "Software"), to deal
# in the Software without restriction, including without limitation the rights
# to use, copy, modify, merge, publish, distribute, sublicense, and / or sell
# copies of the Software, and to permit persons to whom the Software is
# furnished to do so, subject to the following conditions :
# 
# The above copyright notice and this permission notice shall be included in
# all copies or substantial portions of the Software.
# 
# THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
# IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
# FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.IN NO EVENT SHALL THE
# AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
# LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
# OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
# THE SOFTWARE.
#
#------------------------------------------------------------------------------
import json

try:
    from urllib.parse import quote, urlencode
    from urllib.parse import urlunparse
except ImportError:
    from urllib import quote, urlencode #pylint: disable=no-name-in-module
    from urlparse import urlunparse #pylint: disable=import-error

import requests

from . import constants
from . import log
from . import util
from .adal_error import AdalError 

USER_REALM_PATH_TEMPLATE = 'common/UserRealm/<user>'

ACCOUNT_TYPE = constants.UserRealm.account_type
FEDERATION_PROTOCOL_TYPE = constants.UserRealm.federation_protocol_type


class UserRealm(object):

    def __init__(self, call_context, user_principle, authority_url):

        self._log = log.Logger("UserRealm", call_context['log_context'])
        self._call_context = call_context
        self.api_version = '1.0'
        self.federation_protocol = None
        self.account_type = None
        self.federation_metadata_url = None
        self.federation_active_auth_url = None
        self._user_principle = user_principle
        self._authority_url = authority_url

    def _get_user_realm_url(self):

        url_components = list(util.copy_url(self._authority_url))
        url_encoded_user = quote(self._user_principle, safe='~()*!.\'')
        url_components[2] = '/' + USER_REALM_PATH_TEMPLATE.replace('<user>', url_encoded_user)

        user_realm_query = {'api-version':self.api_version}
        url_components[4] = urlencode(user_realm_query)
        return util.copy_url(urlunparse(url_components))

    @staticmethod
    def _validate_constant_value(value_dic, value, case_sensitive=False):

        if not value:
            return False

        if not case_sensitive:
            value = value.lower()

        return value if value in value_dic.values() else False

    @staticmethod
    def _validate_account_type(account_type):
        return UserRealm._validate_constant_value(ACCOUNT_TYPE, account_type)

    @staticmethod
    def _validate_federation_protocol(protocol):
        return UserRealm._validate_constant_value(FEDERATION_PROTOCOL_TYPE, protocol)

    def _log_parsed_response(self):

        self._log.debug(
            'UserRealm response:\n'
            ' AccountType: %(account_type)s\n'
            ' FederationProtocol: %(federation_protocol)s\n'
            ' FederationMetatdataUrl: %(federation_metadata_url)s\n'
            ' FederationActiveAuthUrl: %(federation_active_auth_url)s',
            {
                "account_type": self.account_type,
                "federation_protocol": self.federation_protocol,
                "federation_metadata_url": self.federation_metadata_url,
                "federation_active_auth_url": self.federation_active_auth_url,
            })

    def _parse_discovery_response(self, body):

        self._log.debug("Discovery response:\n %(discovery_response)s",
                        {"discovery_response": body})

        try:
            response = json.loads(body)
        except ValueError:
            self._log.info(
                "Parsing realm discovery response JSON failed for body: %(body)s",
                {"body": body})
            raise

        account_type = UserRealm._validate_account_type(response['account_type'])
        if not account_type:
            raise AdalError('Cannot parse account_type: {}'.format(account_type))
        self.account_type = account_type

        if self.account_type == ACCOUNT_TYPE['Federated']:
            protocol = UserRealm._validate_federation_protocol(response['federation_protocol'])

            if not protocol:
                raise AdalError('Cannot parse federation protocol: {}'.format(protocol))

            self.federation_protocol = protocol
            self.federation_metadata_url = response['federation_metadata_url']
            self.federation_active_auth_url = response['federation_active_auth_url']

        self._log_parsed_response()

    def discover(self):

        options = util.create_request_options(self, {'headers': {'Accept':'application/json'}})
        user_realm_url = self._get_user_realm_url()
        self._log.debug("Performing user realm discovery at: %(user_realm_url)s",
                        {"user_realm_url": user_realm_url.geturl()})

        operation = 'User Realm Discovery'
        resp = requests.get(user_realm_url.geturl(), headers=options['headers'],
                            proxies=self._call_context.get('proxies', None),
                            verify=self._call_context.get('verify_ssl', None))
        util.log_return_correlation_id(self._log, operation, resp)

        if resp.status_code == 429:
            resp.raise_for_status()  # Will raise requests.exceptions.HTTPError
        if not util.is_http_success(resp.status_code):
            return_error_string = u"{} request returned http error: {}".format(operation, 
                                                                               resp.status_code)
            error_response = ""
            if resp.text:
                return_error_string = u"{} and server response: {}".format(return_error_string, resp.text)
                try:
                    error_response = resp.json()
                except ValueError:
                    pass

            raise AdalError(return_error_string, error_response)

        else:
            self._parse_discovery_response(resp.text)
