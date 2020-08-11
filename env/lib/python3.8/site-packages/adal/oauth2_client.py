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

from datetime import datetime, timedelta
import math
import re
import json
import time
import uuid

try:
    from urllib.parse import urlencode, urlparse
except ImportError:
    from urllib import urlencode # pylint: disable=no-name-in-module
    from urlparse import urlparse # pylint: disable=import-error,ungrouped-imports

import requests

from . import log
from . import util
from .constants import OAuth2, TokenResponseFields, IdTokenFields
from .adal_error import AdalError

TOKEN_RESPONSE_MAP = {
    OAuth2.ResponseParameters.TOKEN_TYPE : TokenResponseFields.TOKEN_TYPE,
    OAuth2.ResponseParameters.ACCESS_TOKEN : TokenResponseFields.ACCESS_TOKEN,
    OAuth2.ResponseParameters.REFRESH_TOKEN : TokenResponseFields.REFRESH_TOKEN,
    OAuth2.ResponseParameters.CREATED_ON : TokenResponseFields.CREATED_ON,
    OAuth2.ResponseParameters.EXPIRES_ON : TokenResponseFields.EXPIRES_ON,
    OAuth2.ResponseParameters.EXPIRES_IN : TokenResponseFields.EXPIRES_IN,
    OAuth2.ResponseParameters.RESOURCE : TokenResponseFields.RESOURCE,
    OAuth2.ResponseParameters.ERROR : TokenResponseFields.ERROR,
    OAuth2.ResponseParameters.ERROR_DESCRIPTION : TokenResponseFields.ERROR_DESCRIPTION,
}

_REQ_OPTION = {'headers' : {'content-type': 'application/x-www-form-urlencoded'}}
_ERROR_TEMPLATE = u"{} request returned http error: {}"


def map_fields(in_obj, map_to):
    return dict((map_to[k], v) for k, v in in_obj.items() if k in map_to)

def _get_user_id(id_token):
    user_id = None
    is_displayable = False

    if id_token.get('upn'):
        user_id = id_token['upn']
        is_displayable = True
    elif id_token.get('email'):
        user_id = id_token['email']
        is_displayable = True
    elif id_token.get('sub'):
        user_id = id_token['sub']

    if not user_id:
        user_id = str(uuid.uuid4())

    user_id_vals = {}
    user_id_vals[IdTokenFields.USER_ID] = user_id

    if is_displayable:
        user_id_vals[IdTokenFields.IS_USER_ID_DISPLAYABLE] = True

    return user_id_vals

def _extract_token_values(id_token):
    extracted_values = {}
    extracted_values = map_fields(id_token, OAuth2.IdTokenMap)
    extracted_values.update(_get_user_id(id_token))
    return extracted_values

class OAuth2Client(object):

    def __init__(self, call_context, authority):
        self._token_endpoint = authority.token_endpoint
        self._device_code_endpoint = authority.device_code_endpoint
        self._log = log.Logger("OAuth2Client", call_context['log_context'])
        self._call_context = call_context
        self._cancel_polling_request = False

    def _create_token_url(self):
        parameters = {}
        if self._call_context.get('api_version'):
            parameters[OAuth2.Parameters.AAD_API_VERSION] = self._call_context[
                'api_version']

        return urlparse('{}?{}'.format(self._token_endpoint, urlencode(parameters)))

    def _create_device_code_url(self):
        parameters = {}
        parameters[OAuth2.Parameters.AAD_API_VERSION] = '1.0'
        return urlparse('{}?{}'.format(self._device_code_endpoint, urlencode(parameters)))

    def _parse_optional_ints(self, obj, keys):
        for key in keys:
            try:
                obj[key] = int(obj[key])
            except ValueError:
                self._log.exception("%(key)s could not be parsed as an int", {"key": key})
                raise
            except KeyError:
                # if the key isn't present we can just continue
                pass  

    def _parse_id_token(self, encoded_token):

        cracked_token = self._open_jwt(encoded_token)
        if not cracked_token:
            return

        try:
            b64_id_token = cracked_token['JWSPayload']
            b64_decoded = util.base64_urlsafe_decode(b64_id_token)
            if not b64_decoded:
                self._log.warn('The returned id_token could not be base64 url safe decoded.')
                return

            id_token = json.loads(b64_decoded.decode('utf-8'))
        except ValueError:
            self._log.exception(
                "The returned id_token could not be decoded: %(id_token)s",
                {"id_token": encoded_token})
            raise

        return _extract_token_values(id_token)

    def _open_jwt(self, jwt_token):
        id_token_parts_reg = r"^([^\.\s]*)\.([^\.\s]+)\.([^\.\s]*)$"
        matches = re.search(id_token_parts_reg, jwt_token)
        if not matches or len(matches.groups()) < 3:
            self._log.warn('The token was not parsable.')
            return {}

        return {
            'header': matches.group(1),
            'JWSPayload': matches.group(2),
            'JWSSig': matches.group(3)
            }

    def _validate_token_response(self, body):

        try:
            wire_response = json.loads(body)
        except ValueError:
            self._log.exception(
                'The token response from the server is unparseable as JSON: %(token_response)s',
                {"token_response": body})
            raise

        int_keys = [
            OAuth2.ResponseParameters.EXPIRES_ON,
            OAuth2.ResponseParameters.EXPIRES_IN,
            OAuth2.ResponseParameters.CREATED_ON
        ]

        self._parse_optional_ints(wire_response, int_keys)

        expires_in = wire_response.get(OAuth2.ResponseParameters.EXPIRES_IN)
        if expires_in:
            now = datetime.now()
            soon = timedelta(seconds=expires_in)
            wire_response[OAuth2.ResponseParameters.EXPIRES_ON] = str(now + soon)

        created_on = wire_response.get(OAuth2.ResponseParameters.CREATED_ON)
        if created_on:
            temp_date = datetime.fromtimestamp(created_on)
            wire_response[OAuth2.ResponseParameters.CREATED_ON] = str(temp_date)

        if not wire_response.get(OAuth2.ResponseParameters.TOKEN_TYPE):
            raise AdalError('wire_response is missing token_type', wire_response)

        if not wire_response.get(OAuth2.ResponseParameters.ACCESS_TOKEN):
            raise AdalError('wire_response is missing access_token', wire_response)

        token_response = map_fields(wire_response, TOKEN_RESPONSE_MAP)

        if wire_response.get(OAuth2.ResponseParameters.ID_TOKEN):
            id_token = self._parse_id_token(wire_response[OAuth2.ResponseParameters.ID_TOKEN])
            if id_token:
                token_response.update(id_token)

        return token_response

    def _validate_device_code_response(self, body):

        try:
            wire_response = json.loads(body)
        except ValueError:
            self._log.info('The device code response returned from the server is unparseable as JSON:')
            raise

        int_keys = [
            OAuth2.DeviceCodeResponseParameters.EXPIRES_IN,
            OAuth2.DeviceCodeResponseParameters.INTERVAL
        ]

        self._parse_optional_ints(wire_response, int_keys)

        if not wire_response.get(OAuth2.DeviceCodeResponseParameters.EXPIRES_IN):
            raise AdalError('wire_response is missing expires_in', wire_response)

        if not wire_response.get(OAuth2.DeviceCodeResponseParameters.DEVICE_CODE):
            raise AdalError('wire_response is missing device_code', wire_response)

        if not wire_response.get(OAuth2.DeviceCodeResponseParameters.USER_CODE):
            raise AdalError('wire_response is missing user_code', wire_response)

        #skip field naming tweak, becasue names from wire are python style already
        return wire_response

    def _handle_get_token_response(self, body):
        try:
            return self._validate_token_response(body)
        except Exception:
            self._log.exception(
                "Error validating get token response: %(token_response)s",
                {"token_response": body})
            raise

    def _handle_get_device_code_response(self, body):

        try:
            return self._validate_device_code_response(body)
        except Exception:
            self._log.exception(
                "Error validating get user code response: %(token_response)s",
                {"token_response": body})
            raise

    def get_token(self, oauth_parameters):
        token_url = self._create_token_url()
        url_encoded_token_request = urlencode(oauth_parameters)
        post_options = util.create_request_options(self, _REQ_OPTION)

        operation = "Get Token"

        try:
            resp = requests.post(token_url.geturl(), 
                                 data=url_encoded_token_request, 
                                 headers=post_options['headers'],
                                 verify=self._call_context.get('verify_ssl', None),
                                 proxies=self._call_context.get('proxies', None),
                                 timeout=self._call_context.get('timeout', None))

            util.log_return_correlation_id(self._log, operation, resp)
        except Exception:
            self._log.exception("%(operation)s request failed", {"operation": operation})
            raise

        if util.is_http_success(resp.status_code):
            return self._handle_get_token_response(resp.text)
        else:
            if resp.status_code == 429:
                resp.raise_for_status()  # Will raise requests.exceptions.HTTPError
            return_error_string = _ERROR_TEMPLATE.format(operation, resp.status_code)
            error_response = ""
            if resp.text:
                return_error_string = u"{} and server response: {}".format(return_error_string,
                                                                           resp.text)
                try:
                    error_response = resp.json()
                except ValueError:
                    pass
            raise AdalError(return_error_string, error_response)

    def get_user_code_info(self, oauth_parameters):
        device_code_url = self._create_device_code_url()
        url_encoded_code_request = urlencode(oauth_parameters)

        post_options = util.create_request_options(self, _REQ_OPTION)
        operation = "Get Device Code"
        try:
            resp = requests.post(device_code_url.geturl(), 
                                 data=url_encoded_code_request, 
                                 headers=post_options['headers'],
                                 verify=self._call_context.get('verify_ssl', None),
                                 proxies=self._call_context.get('proxies', None),
                                 timeout=self._call_context.get('timeout', None))
            util.log_return_correlation_id(self._log, operation, resp)
        except Exception:
            self._log.exception("%(operation)s request failed", {"operation": operation})
            raise

        if util.is_http_success(resp.status_code):
            user_code_info = self._handle_get_device_code_response(resp.text)
            user_code_info['correlation_id'] = resp.headers.get('client-request-id')
            return user_code_info
        else:
            if resp.status_code == 429:
                resp.raise_for_status()  # Will raise requests.exceptions.HTTPError
            return_error_string = _ERROR_TEMPLATE.format(operation, resp.status_code)
            error_response = ""
            if resp.text:
                return_error_string = u"{} and server response: {}".format(return_error_string,
                                                                           resp.text)
                try:
                    error_response = resp.json()
                except ValueError:
                    pass

            raise AdalError(return_error_string, error_response)

    def get_token_with_polling(self, oauth_parameters, refresh_internal, expires_in):
        token_url = self._create_token_url()
        url_encoded_code_request = urlencode(oauth_parameters)

        post_options = util.create_request_options(self, _REQ_OPTION)

        operation = "Get token with device code"

        max_times_for_retry = math.floor(expires_in/refresh_internal)
        for _ in range(int(max_times_for_retry)):
            if self._cancel_polling_request:
                raise AdalError('Polling_Request_Cancelled')

            resp = requests.post(
                token_url.geturl(), 
                data=url_encoded_code_request, headers=post_options['headers'],
                proxies=self._call_context.get('proxies', None),
                verify=self._call_context.get('verify_ssl', None))
            if resp.status_code == 429:
                resp.raise_for_status()  # Will raise requests.exceptions.HTTPError

            util.log_return_correlation_id(self._log, operation, resp)

            wire_response = {} 
            if not util.is_http_success(resp.status_code):
                # on error, the body should be json already 
                wire_response = json.loads(resp.text) 

            error = wire_response.get(OAuth2.DeviceCodeResponseParameters.ERROR)
            if error == 'authorization_pending':
                time.sleep(refresh_internal)
                continue
            elif error:
                raise AdalError('Unexpected polling state {}'.format(error),
                                wire_response)
            else:
                try:
                    return self._validate_token_response(resp.text)
                except Exception:
                    self._log.exception(
                        u"Error validating get token response %(access_token)s",
                        {"access_token": resp.text})
                    raise

        raise AdalError('Timeout from "get_token_with_polling"')

    def cancel_polling_request(self):
        self._cancel_polling_request = True

