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

import uuid
from datetime import datetime, timedelta

import requests

from . import log
from . import util
from . import wstrust_response
from .adal_error import AdalError 
from .constants import WSTrustVersion

_USERNAME_PLACEHOLDER = '{UsernamePlaceHolder}'
_PASSWORD_PLACEHOLDER = '{PasswordPlaceHolder}' 

class WSTrustRequest(object):

    def __init__(self, call_context, watrust_endpoint_url, applies_to, wstrust_endpoint_version):
        self._log = log.Logger('WSTrustRequest', call_context['log_context'])
        self._call_context = call_context
        self._wstrust_endpoint_url = watrust_endpoint_url
        self._applies_to = applies_to
        self._wstrust_endpoint_version = wstrust_endpoint_version
        
    @staticmethod
    def _build_security_header():

        time_now = datetime.utcnow()
        expire_time = time_now + timedelta(minutes=10)

        time_now_str = time_now.isoformat()[:-3] + 'Z'
        expire_time_str = expire_time.isoformat()[:-3] + 'Z'

        security_header_xml = ("<wsse:Security s:mustUnderstand='1' xmlns:wsse='http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-secext-1.0.xsd'>"
                               "<wsu:Timestamp wsu:Id=\'_0\'>"
                               "<wsu:Created>" + time_now_str + "</wsu:Created>"
                               "<wsu:Expires>" + expire_time_str  + "</wsu:Expires>"
                               "</wsu:Timestamp>"
                               "<wsse:UsernameToken wsu:Id='ADALUsernameToken'>"
                               "<wsse:Username>" + _USERNAME_PLACEHOLDER + "</wsse:Username>"
                               "<wsse:Password>" + _PASSWORD_PLACEHOLDER + "</wsse:Password>"
                               "</wsse:UsernameToken>"
                               "</wsse:Security>")

        return security_header_xml

    @staticmethod
    def _populate_rst_username_password(template, username, password):
        password = WSTrustRequest._escape_password(password)
        return template.replace(_USERNAME_PLACEHOLDER, username).replace(_PASSWORD_PLACEHOLDER, password)

    @staticmethod
    def _escape_password(password):
        return password.replace('&', '&amp;').replace('"', '&quot;').replace("'", '&apos;').replace('<', '&lt;').replace('>', '&gt;')

    def _build_rst(self, username, password):
        message_id = str(uuid.uuid4())

        schema_location = 'http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-utility-1.0.xsd'
        soap_action = 'http://docs.oasis-open.org/ws-sx/ws-trust/200512/RST/Issue'
        rst_trust_namespace = 'http://docs.oasis-open.org/ws-sx/ws-trust/200512'
        key_type = 'http://docs.oasis-open.org/ws-sx/ws-trust/200512/Bearer'
        request_type = 'http://docs.oasis-open.org/ws-sx/ws-trust/200512/Issue'
 
        if self._wstrust_endpoint_version == WSTrustVersion.WSTRUST2005:
            soap_action = 'http://schemas.xmlsoap.org/ws/2005/02/trust/RST/Issue'
            rst_trust_namespace = 'http://schemas.xmlsoap.org/ws/2005/02/trust'
            key_type = 'http://schemas.xmlsoap.org/ws/2005/05/identity/NoProofKey'
            request_type = 'http://schemas.xmlsoap.org/ws/2005/02/trust/Issue'
   
        rst_template = ("<s:Envelope xmlns:s='http://www.w3.org/2003/05/soap-envelope' xmlns:wsa='http://www.w3.org/2005/08/addressing' xmlns:wsu='{}'>".format(schema_location) +
                        "<s:Header>" + 
                        "<wsa:Action s:mustUnderstand='1'>{}</wsa:Action>".format(soap_action) +
                        "<wsa:messageID>urn:uuid:{}</wsa:messageID>".format(message_id) +
                        "<wsa:ReplyTo>" +
                        "<wsa:Address>http://www.w3.org/2005/08/addressing/anonymous</wsa:Address>" +
                        "</wsa:ReplyTo>" +
                        "<wsa:To s:mustUnderstand='1'>{}</wsa:To>".format(self._wstrust_endpoint_url) +
                        WSTrustRequest._build_security_header() +
                        "</s:Header>" +
                        "<s:Body>" +
                        "<wst:RequestSecurityToken xmlns:wst='{}'>".format(rst_trust_namespace) +
                        "<wsp:AppliesTo xmlns:wsp='http://schemas.xmlsoap.org/ws/2004/09/policy'>" + 
                        "<wsa:EndpointReference>" +
                        "<wsa:Address>{}</wsa:Address>".format(self._applies_to) +
                        "</wsa:EndpointReference>" +
                        "</wsp:AppliesTo>" +
                        "<wst:KeyType>{}</wst:KeyType>".format(key_type) +
                        "<wst:RequestType>{}</wst:RequestType>".format(request_type) +
                        "</wst:RequestSecurityToken>" +
                        "</s:Body>" +
                        "</s:Envelope>")

        self._log.debug('Created RST: \n %(rst_template)s',
                        {"rst_template": rst_template})
        return WSTrustRequest._populate_rst_username_password(rst_template, username, password)

    def _handle_rstr(self, body):
        wstrust_resp = wstrust_response.WSTrustResponse(self._call_context, body, self._wstrust_endpoint_version)
        wstrust_resp.parse()
        return wstrust_resp

    def acquire_token(self, username, password):
        if self._wstrust_endpoint_version == WSTrustVersion.UNDEFINED:
            raise AdalError('Unsupported wstrust endpoint version. Current support version is wstrust2005 or wstrust13.')

        rst = self._build_rst(username, password)
        if self._wstrust_endpoint_version == WSTrustVersion.WSTRUST2005:
            soap_action = 'http://schemas.xmlsoap.org/ws/2005/02/trust/RST/Issue'
        else:
            soap_action = 'http://docs.oasis-open.org/ws-sx/ws-trust/200512/RST/Issue'

        headers = {'headers': {'Content-type':'application/soap+xml; charset=utf-8',
                               'SOAPAction': soap_action},
                   'body': rst}
        options = util.create_request_options(self, headers)
        self._log.debug("Sending RST to: %(wstrust_endpoint)s",
                        {"wstrust_endpoint": self._wstrust_endpoint_url})

        operation = "WS-Trust RST"
        resp = requests.post(self._wstrust_endpoint_url, headers=options['headers'], data=rst,
                             allow_redirects=True,
                             verify=self._call_context.get('verify_ssl', None),
                             proxies=self._call_context.get('proxies', None),
                             timeout=self._call_context.get('timeout', None))

        util.log_return_correlation_id(self._log, operation, resp)

        if resp.status_code == 429:
            resp.raise_for_status()  # Will raise requests.exceptions.HTTPError
        if not util.is_http_success(resp.status_code):
            return_error_string = u"{} request returned http error: {}".format(operation, resp.status_code)
            error_response = ""
            if resp.text:
                return_error_string = u"{} and server response: {}".format(return_error_string, resp.text)
                try:
                    error_response = resp.json()
                except ValueError:
                    pass

            raise AdalError(return_error_string, error_response)
        else:
            return self._handle_rstr(resp.text)
