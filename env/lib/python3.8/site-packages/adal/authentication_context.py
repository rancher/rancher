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
import os
import threading
import warnings

from .authority import Authority
from . import argument
from .code_request import CodeRequest
from .token_request import TokenRequest
from .token_cache import TokenCache
from . import log
from .constants import OAuth2DeviceCodeResponseParameters

GLOBAL_ADAL_OPTIONS = {}

class AuthenticationContext(object):
    '''Retrieves authentication tokens from Azure Active Directory.

    For usages, check out the "sample" folder at:
        https://github.com/AzureAD/azure-activedirectory-library-for-python
    '''

    def __init__(
            self, authority, validate_authority=None, cache=None,
            api_version=None, timeout=None, enable_pii=False, verify_ssl=None, proxies=None):
        '''Creates a new AuthenticationContext object.

        By default the authority will be checked against a list of known Azure
        Active Directory authorities. If the authority is not recognized as 
        one of these well known authorities then token acquisition will fail.
        This behavior can be turned off via the validate_authority parameter
        below.

        :param str authority: A URL that identifies a token authority. It should be of the
            format https://login.microsoftonline.com/your_tenant
        :param bool validate_authority: (optional) Turns authority validation 
            on or off. This parameter default to true.
        :param TokenCache cache: (optional) Sets the token cache used by this 
            AuthenticationContext instance. If this parameter is not set, then
            a default is used. Cache instances is only used by that instance of
            the AuthenticationContext and are not shared unless it has been
            manually passed during the construction of other
            AuthenticationContexts.
        :param api_version: (optional) Specifies API version using on the wire.
            Historically it has a hardcoded default value as "1.0".
            Developers have been encouraged to set it as None explicitly,
            which means the underlying API version will be automatically chosen.
            Starting from ADAL Python 1.0, this default value becomes None.
        :param timeout: (optional) requests timeout. How long to wait for the server to send
            data before giving up, as a float, or a `(connect timeout,
            read timeout) <timeouts>` tuple.
        :param enable_pii: (optional) Unless this is set to True,
            there will be no Personally Identifiable Information (PII) written in log.
        :param verify_ssl: (optional) requests verify. Either a boolean, in which case it 
            controls whether we verify the server's TLS certificate, or a string, in which 
            case it must be a path to a CA bundle to use. If this value is not provided, and 
            ADAL_PYTHON_SSL_NO_VERIFY env varaible is set, behavior is equivalent to 
            verify_ssl=False.
        :param proxies: (optional) requests proxies. Dictionary mapping protocol to the URL 
            of the proxy. See http://docs.python-requests.org/en/master/user/advanced/#proxies
            for details.
        '''
        self.authority = Authority(authority, validate_authority is None or validate_authority)
        self._oauth2client = None
        self.correlation_id = None
        env_verify = 'ADAL_PYTHON_SSL_NO_VERIFY' not in os.environ
        verify = verify_ssl if verify_ssl is not None else env_verify
        if api_version is not None:
            warnings.warn(
                """The default behavior of including api-version=1.0 on the wire
                is now deprecated.
                Future version of ADAL will change the default value to None.

                To ensure a smooth transition, you are recommended to explicitly
                set it to None in your code now, and test out the new behavior.

                    context = AuthenticationContext(..., api_version=None)
                """, DeprecationWarning)
        self._call_context = {
            'options': GLOBAL_ADAL_OPTIONS,
            'api_version': api_version,
            'verify_ssl': verify,
            'proxies':proxies,
            'timeout':timeout,
            "enable_pii": enable_pii,
            }
        self._token_requests_with_user_code = {}
        self.cache = cache or TokenCache()
        self._lock = threading.RLock()

    @property
    def options(self):
        return self._call_context['options']

    @options.setter
    def options(self, val):
        self._call_context['options'] = val

    def _acquire_token(self, token_func, correlation_id=None):
        self._call_context['log_context'] = log.create_log_context(
            correlation_id or self.correlation_id, self._call_context.get('enable_pii', False))
        self.authority.validate(self._call_context)
        return token_func(self)

    def acquire_token(self, resource, user_id, client_id):
        '''Gets a token for a given resource via cached tokens.

        :param str resource: A URI that identifies the resource for which the
            token is valid.
        :param str user_id: The username of the user on behalf this application
            is authenticating.
        :param str client_id: The OAuth client id of the calling application.
        :returns: dic with several keys, include "accessToken" and
            "refreshToken".
        '''
        def token_func(self):
            token_request = TokenRequest(self._call_context, self, client_id, resource)
            return token_request.get_token_from_cache_with_refresh(user_id)

        return self._acquire_token(token_func)       

    def acquire_token_with_username_password(self, resource, username, password, client_id):
        '''Gets a token for a given resource via user credentails.
        
        :param str resource: A URI that identifies the resource for which the 
            token is valid.
        :param str username: The username of the user on behalf this
            application is authenticating.
        :param str password: The password of the user named in the username
            parameter.
        :param str client_id: The OAuth client id of the calling application.
        :returns: dict with several keys, include "accessToken" and
            "refreshToken".
        '''
        def token_func(self):
            token_request = TokenRequest(self._call_context, self, client_id, resource)
            return token_request.get_token_with_username_password(username, password)

        return self._acquire_token(token_func)

    def acquire_token_with_client_credentials(self, resource, client_id, client_secret):
        '''Gets a token for a given resource via client credentials.

        :param str resource: A URI that identifies the resource for which the 
            token is valid.
        :param str client_id: The OAuth client id of the calling application.
        :param str client_secret: The OAuth client secret of the calling application.
        :returns: dict with several keys, include "accessToken".
        '''
        def token_func(self):
            token_request = TokenRequest(self._call_context, self, client_id, resource)
            return token_request.get_token_with_client_credentials(client_secret)

        return self._acquire_token(token_func)

    def acquire_token_with_authorization_code(self, authorization_code, 
                                              redirect_uri, resource, 
                                              client_id, client_secret=None, code_verifier=None):
        '''Gets a token for a given resource via authorization code for a
        server app.
        
        :param str authorization_code: An authorization code returned from a
            client.
        :param str redirect_uri: the redirect uri that was used in the
            authorize call.
        :param str resource: A URI that identifies the resource for which the
            token is valid.
        :param str client_id: The OAuth client id of the calling application.
        :param str client_secret: (only for confidential clients)The OAuth
            client secret of the calling application. This parameter if not set,
            defaults to None
        :param str code_verifier: (optional)The code verifier that was used to
            obtain authorization code if PKCE was used in the authorization
            code grant request.(usually used by public clients) This parameter if not set,
            defaults to None
        :returns: dict with several keys, include "accessToken" and
            "refreshToken".
        '''
        def token_func(self):
            token_request = TokenRequest(
                self._call_context, 
                self, 
                client_id, 
                resource, 
                redirect_uri)
            return token_request.get_token_with_authorization_code(
                authorization_code, 
                client_secret, code_verifier)

        return self._acquire_token(token_func)

    def acquire_token_with_refresh_token(self, refresh_token, client_id,
                                         resource, client_secret=None):
        '''Gets a token for a given resource via refresh tokens
        
        :param str refresh_token: A refresh token returned in a tokne response
            from a previous invocation of acquireToken.
        :param str client_id: The OAuth client id of the calling application.
        :param str resource: A URI that identifies the resource for which the
            token is valid.
        :param str client_secret: (optional)The OAuth client secret of the
            calling application.                 
        :returns: dict with several keys, include "accessToken" and
            "refreshToken".
        '''
        def token_func(self):
            token_request = TokenRequest(self._call_context, self, client_id, resource)
            return token_request.get_token_with_refresh_token(refresh_token, client_secret)

        return self._acquire_token(token_func)

    def acquire_token_with_client_certificate(self, resource, client_id, 
                                              certificate, thumbprint, public_certificate=None):
        '''Gets a token for a given resource via certificate credentials

        :param str resource: A URI that identifies the resource for which the
            token is valid.
        :param str client_id: The OAuth client id of the calling application.
        :param str certificate: A PEM encoded certificate private key.
        :param str thumbprint: hex encoded thumbprint of the certificate.
        :param str public_certificate(optional): if not None, it will be sent to the service for subject name
            and issuer based authentication, which is to support cert auto rolls. The value must match the
            certificate private key parameter.

            Per `specs <https://tools.ietf.org/html/rfc7515#section-4.1.6>`_,
            "the certificate containing
            the public key corresponding to the key used to digitally sign the
            JWS MUST be the first certificate.  This MAY be followed by
            additional certificates, with each subsequent certificate being the
            one used to certify the previous one."
            However, your certificate's issuer may use a different order.
            So, if your attempt ends up with an error AADSTS700027 -
            "The provided signature value did not match the expected signature value",
            you may try use only the leaf cert (in PEM/str format) instead.

        :returns: dict with several keys, include "accessToken".
        '''
        def token_func(self):
            token_request = TokenRequest(self._call_context, self, client_id, resource)
            return token_request.get_token_with_certificate(certificate, thumbprint, public_certificate)

        return self._acquire_token(token_func)

    def acquire_user_code(self, resource, client_id, language=None):
        '''Gets the user code info which contains user_code, device_code for
        authenticating user on device.
        
        :param str resource: A URI that identifies the resource for which the 
            device_code and user_code is valid for.
        :param str client_id: The OAuth client id of the calling application.
        :param str language: The language code specifying how the message
            should be localized to.
        :returns: dict contains code and uri for users to login through browser.
        '''
        self._call_context['log_context'] = log.create_log_context(
            self.correlation_id, self._call_context.get('enable_pii', False))
        self.authority.validate(self._call_context)
        code_request = CodeRequest(self._call_context, self, client_id, resource)
        return code_request.get_user_code_info(language)

    def acquire_token_with_device_code(self, resource, user_code_info, client_id):
        '''Gets a new access token using via a device code. 
        
        :param str resource: A URI that identifies the resource for which the
            token is valid.
        :param dict user_code_info: The code info from the invocation of
            "acquire_user_code"
        :param str client_id: The OAuth client id of the calling application.
        :returns: dict with several keys, include "accessToken" and
            "refreshToken".
        '''
        def token_func(self):
            token_request = TokenRequest(self._call_context, self, client_id, resource)

            key = user_code_info[OAuth2DeviceCodeResponseParameters.DEVICE_CODE]
            with self._lock:
                self._token_requests_with_user_code[key] = token_request

            token = token_request.get_token_with_device_code(user_code_info)
            
            with self._lock:
                self._token_requests_with_user_code.pop(key, None)
            
            return token

        return self._acquire_token(token_func, user_code_info.get('correlation_id', None))

    def cancel_request_to_get_token_with_device_code(self, user_code_info):
        '''Cancels the polling request to get token with device code. 

        :param dict user_code_info: The code info from the invocation of
            "acquire_user_code"
        :returns: None
        '''
        argument.validate_user_code_info(user_code_info)
        
        key = user_code_info[OAuth2DeviceCodeResponseParameters.DEVICE_CODE]
        with self._lock:
            request = self._token_requests_with_user_code.get(key)

            if not request:
                raise ValueError('No acquire_token_with_device_code existed to be cancelled')

            request.cancel_token_request_with_device_code()
            self._token_requests_with_user_code.pop(key, None)
