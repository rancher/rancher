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

from base64 import b64encode

from . import constants
from . import log
from . import mex
from . import oauth2_client
from . import self_signed_jwt
from . import user_realm
from . import wstrust_request
from .adal_error import AdalError
from .cache_driver import CacheDriver
from .constants import WSTrustVersion

OAUTH2_PARAMETERS = constants.OAuth2.Parameters
TOKEN_RESPONSE_FIELDS = constants.TokenResponseFields
OAUTH2_GRANT_TYPE = constants.OAuth2.GrantType
OAUTH2_SCOPE = constants.OAuth2.Scope
OAUTH2_DEVICE_CODE_RESPONSE_PARAMETERS = constants.OAuth2.DeviceCodeResponseParameters 
SAML = constants.Saml
ACCOUNT_TYPE = constants.UserRealm.account_type
USER_ID = constants.TokenResponseFields.USER_ID
_CLIENT_ID = constants.TokenResponseFields._CLIENT_ID #pylint: disable=protected-access

def add_parameter_if_available(parameters, key, value):
    if value:
        parameters[key] = value

def _get_saml_grant_type(wstrust_response):
    token_type = wstrust_response.token_type
    if token_type == SAML.TokenTypeV1 or token_type == SAML.OasisWssSaml11TokenProfile11:
        return OAUTH2_GRANT_TYPE.SAML1

    elif token_type == SAML.TokenTypeV2 or token_type == SAML.OasisWssSaml2TokenProfile2:
        return OAUTH2_GRANT_TYPE.SAML2

    else:
        raise AdalError("RSTR returned unknown token type: {}".format(token_type))

class TokenRequest(object):

    def __init__(self, call_context, authentication_context, client_id, 
                 resource, redirect_uri=None):

        self._log = log.Logger("TokenRequest", call_context['log_context'])
        self._call_context = call_context

        self._authentication_context = authentication_context
        self._resource = resource
        self._client_id = client_id
        self._redirect_uri = redirect_uri

        self._cache_driver = None
        
        # should be set at the beginning of get_token
        # functions that have a user_id
        self._user_id = None
        self._user_realm = None

        # should be set when acquire token using device flow
        self._polling_client = None

    def _create_user_realm_request(self, username):
        return user_realm.UserRealm(self._call_context, 
                                    username, 
                                    self._authentication_context.authority.url)

    def _create_mex(self, mex_endpoint):
        return mex.Mex(self._call_context, mex_endpoint)

    def _create_wstrust_request(self, wstrust_endpoint, applies_to, wstrust_endpoint_version):
        return wstrust_request.WSTrustRequest(self._call_context, wstrust_endpoint,
                                              applies_to, wstrust_endpoint_version)

    def _create_oauth2_client(self):
        return oauth2_client.OAuth2Client(self._call_context, 
                                          self._authentication_context.authority)

    def _create_self_signed_jwt(self):
        return self_signed_jwt.SelfSignedJwt(self._call_context, 
                                             self._authentication_context.authority, 
                                             self._client_id)

    def _oauth_get_token(self, oauth_parameters):
        client = self._create_oauth2_client()
        return client.get_token(oauth_parameters)

    def _create_cache_driver(self):
        return CacheDriver(
            self._call_context,
            self._authentication_context.authority.url,
            self._resource,
            self._client_id,
            self._authentication_context.cache,
            self._get_token_with_token_response
        )

    def _find_token_from_cache(self):
        self._cache_driver = self._create_cache_driver()
        cache_query = self._create_cache_query()
        return self._cache_driver.find(cache_query)

    def _add_token_into_cache(self, token):
        cache_driver = self._create_cache_driver()
        self._log.debug('Storing retrieved token into cache')
        cache_driver.add(token)

    def _get_token_with_token_response(self, entry, resource):
        self._log.debug("called to refresh a token from the cache")
        refresh_token = entry[TOKEN_RESPONSE_FIELDS.REFRESH_TOKEN]
        return self._get_token_with_refresh_token(refresh_token, resource, None)

    def _create_cache_query(self):
        query = {_CLIENT_ID : self._client_id}
        if self._user_id:
            query[USER_ID] = self._user_id
        else:
            self._log.debug("No user_id passed for cache query")

        return query

    def _create_oauth_parameters(self, grant_type):

        oauth_parameters = {}
        oauth_parameters[OAUTH2_PARAMETERS.GRANT_TYPE] = grant_type

        if (OAUTH2_GRANT_TYPE.AUTHORIZATION_CODE != grant_type and
                OAUTH2_GRANT_TYPE.CLIENT_CREDENTIALS != grant_type and
                OAUTH2_GRANT_TYPE.REFRESH_TOKEN != grant_type and
                OAUTH2_GRANT_TYPE.DEVICE_CODE != grant_type):

            oauth_parameters[OAUTH2_PARAMETERS.SCOPE] = OAUTH2_SCOPE.OPENID

        add_parameter_if_available(oauth_parameters, OAUTH2_PARAMETERS.CLIENT_ID, 
                                   self._client_id)
        add_parameter_if_available(oauth_parameters, OAUTH2_PARAMETERS.RESOURCE, 
                                   self._resource)
        add_parameter_if_available(oauth_parameters, OAUTH2_PARAMETERS.REDIRECT_URI, 
                                   self._redirect_uri)

        return oauth_parameters

    def _get_token_username_password_managed(self, username, password):
        self._log.debug('Acquiring token with username password for managed user')

        oauth_parameters = self._create_oauth_parameters(OAUTH2_GRANT_TYPE.PASSWORD)

        oauth_parameters[OAUTH2_PARAMETERS.PASSWORD] = password
        oauth_parameters[OAUTH2_PARAMETERS.USERNAME] = username

        return self._oauth_get_token(oauth_parameters)

    def _perform_wstrust_assertion_oauth_exchange(self, wstrust_response):
        self._log.debug("Performing OAuth assertion grant type exchange.")

        oauth_parameters = {}
        grant_type = _get_saml_grant_type(wstrust_response)
        
        token_bytes = wstrust_response.token
        assertion = b64encode(token_bytes)

        oauth_parameters = self._create_oauth_parameters(grant_type)
        oauth_parameters[OAUTH2_PARAMETERS.ASSERTION] = assertion

        return self._oauth_get_token(oauth_parameters)

    def _perform_wstrust_exchange(self, wstrust_endpoint, wstrust_endpoint_version, username, password):

        wstrust = self._create_wstrust_request(wstrust_endpoint, "urn:federation:MicrosoftOnline",
                                               wstrust_endpoint_version)
        result = wstrust.acquire_token(username, password)

        if not result.token:
            err_template = "Unsuccessful RSTR.\n\terror code: {}\n\tfaultMessage: {}"
            error_msg = err_template.format(result.error_code, result.fault_message)
            self._log.info(error_msg)
            raise AdalError(error_msg)

        return result

    def _perform_username_password_for_access_token_exchange(self, wstrust_endpoint, wstrust_endpoint_version,
                                                             username, password):
        wstrust_response = self._perform_wstrust_exchange(wstrust_endpoint, wstrust_endpoint_version,
                                                          username, password)
        return self._perform_wstrust_assertion_oauth_exchange(wstrust_response)

    def _get_token_username_password_federated(self, username, password):
        self._log.debug("Acquiring token with username password for federated user")

        if not self._user_realm.federation_metadata_url:
            self._log.warn("Unable to retrieve federationMetadataUrl from AAD. "
                           "Attempting fallback to AAD supplied endpoint.")

            if not self._user_realm.federation_active_auth_url:
                raise AdalError('AAD did not return a WSTrust endpoint. Unable to proceed.')

            wstrust_version = TokenRequest._parse_wstrust_version_from_federation_active_authurl(
                self._user_realm.federation_active_auth_url)
            self._log.debug(
                'wstrust endpoint version is: %(wstrust_version)s',
                {"wstrust_version": wstrust_version})

            return self._perform_username_password_for_access_token_exchange(
                self._user_realm.federation_active_auth_url,
                wstrust_version, username, password)
        else:
            mex_endpoint = self._user_realm.federation_metadata_url
            self._log.debug(
                "Attempting mex at: %(mex_endpoint)s",
                {"mex_endpoint": mex_endpoint})
            mex_instance = self._create_mex(mex_endpoint)
            wstrust_version = WSTrustVersion.UNDEFINED

            try:
                mex_instance.discover()
                wstrust_endpoint = mex_instance.username_password_policy['url']
                wstrust_version = mex_instance.username_password_policy['version']
            except Exception: #pylint: disable=broad-except
                self._log.warn(
                    "MEX exchange failed for %(mex_endpoint)s. "
                    "Attempting fallback to AAD supplied endpoint.",
                    {"mex_endpoint": mex_endpoint})
                wstrust_endpoint = self._user_realm.federation_active_auth_url
                wstrust_version = TokenRequest._parse_wstrust_version_from_federation_active_authurl(
                    self._user_realm.federation_active_auth_url)
                if not wstrust_endpoint:
                    raise AdalError('AAD did not return a WSTrust endpoint. Unable to proceed.')

            return self._perform_username_password_for_access_token_exchange(wstrust_endpoint, wstrust_version,
                                                                             username, password)
    @staticmethod
    def _parse_wstrust_version_from_federation_active_authurl(federation_active_authurl):
        if '/trust/2005/usernamemixed' in federation_active_authurl:
            return WSTrustVersion.WSTRUST2005
        if '/trust/13/usernamemixed' in federation_active_authurl:
            return WSTrustVersion.WSTRUST13
        return WSTrustVersion.UNDEFINED

    def get_token_with_username_password(self, username, password):
        self._log.debug("Acquiring token with username password.")
        self._user_id = username
        try:
            token = self._find_token_from_cache()
            if token:
                return token
        except AdalError:
            self._log.exception('Attempt to look for token in cache resulted in Error')

        if not self._authentication_context.authority.is_adfs_authority:
            self._user_realm = self._create_user_realm_request(username)
            self._user_realm.discover()

            try:
                if self._user_realm.account_type == ACCOUNT_TYPE['Managed']:
                    token = self._get_token_username_password_managed(username, password)
                elif self._user_realm.account_type == ACCOUNT_TYPE['Federated']:
                    token = self._get_token_username_password_federated(username, password)
                else:
                    raise AdalError(
                        "Server returned an unknown AccountType: {}".format(self._user_realm.account_type))
                self._log.debug("Successfully retrieved token from authority.")
            except Exception:
                self._log.info("get_token_func returned with error")
                raise
        else:
            self._log.info('Skipping user realm discovery for ADFS authority')
            token = self._get_token_username_password_managed(username, password)
       
        self._cache_driver.add(token)
        return token

    def get_token_with_client_credentials(self, client_secret):
        self._log.debug("Getting token with client credentials.")
        try:
            token = self._find_token_from_cache()
            if token:
                return token
        except AdalError:
            self._log.exception('Attempt to look for token in cache resulted in Error')

        oauth_parameters = self._create_oauth_parameters(OAUTH2_GRANT_TYPE.CLIENT_CREDENTIALS)
        oauth_parameters[OAUTH2_PARAMETERS.CLIENT_SECRET] = client_secret

        token = self._oauth_get_token(oauth_parameters)
        self._cache_driver.add(token)
        return token

    def get_token_with_authorization_code(self, authorization_code, client_secret, code_verifier):

        self._log.info("Getting token with auth code.")
        oauth_parameters = self._create_oauth_parameters(OAUTH2_GRANT_TYPE.AUTHORIZATION_CODE)
        oauth_parameters[OAUTH2_PARAMETERS.CODE] = authorization_code
        if client_secret is not None:
            oauth_parameters[OAUTH2_PARAMETERS.CLIENT_SECRET] = client_secret
        if code_verifier is not None:
            oauth_parameters[OAUTH2_PARAMETERS.CODE_VERIFIER] = code_verifier
        token = self._oauth_get_token(oauth_parameters)
        self._add_token_into_cache(token)
        return token

    def _get_token_with_refresh_token(self, refresh_token, resource, client_secret):

        self._log.info("Getting a new token from a refresh token")

        oauth_parameters = self._create_oauth_parameters(OAUTH2_GRANT_TYPE.REFRESH_TOKEN)
        if resource:
            oauth_parameters[OAUTH2_PARAMETERS.RESOURCE] = resource

        if client_secret:
            oauth_parameters[OAUTH2_PARAMETERS.CLIENT_SECRET] = client_secret

        oauth_parameters[OAUTH2_PARAMETERS.REFRESH_TOKEN] = refresh_token
        return self._oauth_get_token(oauth_parameters)

    def get_token_with_refresh_token(self, refresh_token, client_secret):
        return self._get_token_with_refresh_token(refresh_token, None, client_secret)

    def get_token_from_cache_with_refresh(self, user_id):
        self._log.debug("Getting token from cache with refresh if necessary.")
        self._user_id = user_id
        return self._find_token_from_cache()

    def _create_jwt(self, certificate, thumbprint, public_certificate):

        ssj = self._create_self_signed_jwt()
        jwt = ssj.create(certificate, thumbprint, public_certificate)

        if not jwt:
            raise AdalError("Failed to create JWT.")
        return jwt

    def get_token_with_certificate(self, certificate, thumbprint, public_certificate):

        self._log.info("Getting a token via certificate.")

        jwt = self._create_jwt(certificate, thumbprint, public_certificate)

        oauth_parameters = self._create_oauth_parameters(OAUTH2_GRANT_TYPE.CLIENT_CREDENTIALS)
        oauth_parameters[OAUTH2_PARAMETERS.CLIENT_ASSERTION_TYPE] = OAUTH2_GRANT_TYPE.JWT_BEARER
        oauth_parameters[OAUTH2_PARAMETERS.CLIENT_ASSERTION] = jwt

        try:
            token = self._find_token_from_cache()
            if token:
                return token
        except AdalError:
            self._log.exception('Attempt to look for token in cache resulted in Error')

        return self._oauth_get_token(oauth_parameters)

    def get_token_with_device_code(self, user_code_info):
        self._log.info("Getting a token via device code")

        oauth_parameters = self._create_oauth_parameters(OAUTH2_GRANT_TYPE.DEVICE_CODE)
        oauth_parameters[OAUTH2_PARAMETERS.CODE] = user_code_info[OAUTH2_DEVICE_CODE_RESPONSE_PARAMETERS.DEVICE_CODE]

        interval = user_code_info[OAUTH2_DEVICE_CODE_RESPONSE_PARAMETERS.INTERVAL]
        expires_in = user_code_info[OAUTH2_DEVICE_CODE_RESPONSE_PARAMETERS.EXPIRES_IN]

        if interval <= 0:
            raise AdalError('invalid refresh interval')

        client = self._create_oauth2_client()
        self._polling_client = client

        token = client.get_token_with_polling(oauth_parameters, interval, expires_in)
        self._add_token_into_cache(token)

        return token

    def cancel_token_request_with_device_code(self):
        self._polling_client.cancel_polling_request()
