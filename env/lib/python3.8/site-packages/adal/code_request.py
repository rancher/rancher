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

from . import constants
from . import log
from . import oauth2_client

OAUTH2_PARAMETERS = constants.OAuth2.Parameters

class CodeRequest(object):
    def __init__(self, call_context, authentication_context, client_id, 
                 resource):
        self._log = log.Logger("CodeRequest", call_context['log_context'])
        self._call_context = call_context
        self._authentication_context = authentication_context
        self._client_id = client_id
        self._resource = resource

    def _get_user_code_info(self, oauth_parameters):
        client = self._create_oauth2_client()
        return client.get_user_code_info(oauth_parameters)

    def _create_oauth2_client(self):
        return oauth2_client.OAuth2Client(
            self._call_context,
            self._authentication_context.authority)

    def _create_oauth_parameters(self):
        return {
            OAUTH2_PARAMETERS.CLIENT_ID: self._client_id,
            OAUTH2_PARAMETERS.RESOURCE: self._resource
        }

    def get_user_code_info(self, language):
        self._log.info('Getting user code info.')

        oauth_parameters = self._create_oauth_parameters()
        if language:
            oauth_parameters[OAUTH2_PARAMETERS.LANGUAGE] = language

        return self._get_user_code_info(oauth_parameters)
