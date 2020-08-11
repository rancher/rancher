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
# pylint: disable=too-few-public-methods,old-style-class,no-init

class Errors: 
    # Constants
    ERROR_VALUE_NONE = '{} should not be None.'
    ERROR_VALUE_EMPTY_STRING = '{} should not be "".'
    ERROR_RESPONSE_MALFORMED_XML = 'The provided response string is not well formed XML.'

class OAuth2Parameters(object):

    GRANT_TYPE = 'grant_type'
    CLIENT_ASSERTION = 'client_assertion'
    CLIENT_ASSERTION_TYPE = 'client_assertion_type'
    CLIENT_ID = 'client_id'
    CLIENT_SECRET = 'client_secret'
    REDIRECT_URI = 'redirect_uri'
    RESOURCE = 'resource'
    CODE = 'code'
    CODE_VERIFIER = 'code_verifier'
    SCOPE = 'scope'
    ASSERTION = 'assertion'
    AAD_API_VERSION = 'api-version'
    USERNAME = 'username'
    PASSWORD = 'password'
    REFRESH_TOKEN = 'refresh_token'
    LANGUAGE = 'mkt'
    DEVICE_CODE = 'device_code'

class OAuth2GrantType(object):

    AUTHORIZATION_CODE = 'authorization_code'
    REFRESH_TOKEN = 'refresh_token'
    CLIENT_CREDENTIALS = 'client_credentials'
    JWT_BEARER = 'urn:ietf:params:oauth:client-assertion-type:jwt-bearer'
    PASSWORD = 'password'
    SAML1 = 'urn:ietf:params:oauth:grant-type:saml1_1-bearer'
    SAML2 = 'urn:ietf:params:oauth:grant-type:saml2-bearer'
    DEVICE_CODE = 'device_code'


class OAuth2ResponseParameters(object):

    CODE = 'code'
    TOKEN_TYPE = 'token_type'
    ACCESS_TOKEN = 'access_token'
    ID_TOKEN = 'id_token'
    REFRESH_TOKEN = 'refresh_token'
    CREATED_ON = 'created_on'
    EXPIRES_ON = 'expires_on'
    EXPIRES_IN = 'expires_in'
    RESOURCE = 'resource'
    ERROR = 'error'
    ERROR_DESCRIPTION = 'error_description'

class OAuth2DeviceCodeResponseParameters:
    USER_CODE = 'user_code'
    DEVICE_CODE = 'device_code'
    VERIFICATION_URL = 'verification_url'
    EXPIRES_IN = 'expires_in'
    INTERVAL = 'interval'
    MESSAGE = 'message'
    ERROR = 'error'
    ERROR_DESCRIPTION = 'error_description'

class OAuth2Scope(object):

    OPENID = 'openid'


class OAuth2(object):

    Parameters = OAuth2Parameters()
    GrantType = OAuth2GrantType()
    ResponseParameters = OAuth2ResponseParameters()
    DeviceCodeResponseParameters = OAuth2DeviceCodeResponseParameters()
    Scope = OAuth2Scope()
    IdTokenMap = {
        'tid' : 'tenantId',
        'given_name' : 'givenName',
        'family_name' : 'familyName',
        'idp' : 'identityProvider',
        'oid' : 'oid'
        }


class TokenResponseFields(object):

    TOKEN_TYPE = 'tokenType'
    ACCESS_TOKEN = 'accessToken'
    REFRESH_TOKEN = 'refreshToken'
    CREATED_ON = 'createdOn'
    EXPIRES_ON = 'expiresOn'
    EXPIRES_IN = 'expiresIn'
    RESOURCE = 'resource'
    USER_ID = 'userId'
    ERROR = 'error'
    ERROR_DESCRIPTION = 'errorDescription'
    
    # not from the wire, but amends for token cache
    _AUTHORITY = '_authority'
    _CLIENT_ID = '_clientId'
    IS_MRRT = 'isMRRT'


class IdTokenFields(object):

    USER_ID = 'userId'
    IS_USER_ID_DISPLAYABLE = 'isUserIdDisplayable'
    TENANT_ID = 'tenantId'
    GIVE_NAME = 'givenName'
    FAMILY_NAME = 'familyName'
    IDENTITY_PROVIDER = 'identityProvider'

class Misc(object):

    MAX_DATE = 0xffffffff
    CLOCK_BUFFER = 5 # In minutes.


class Jwt(object):

    SELF_SIGNED_JWT_LIFETIME = 10 # 10 mins in mins
    AUDIENCE = 'aud'
    ISSUER = 'iss'
    SUBJECT = 'sub'
    NOT_BEFORE = 'nbf'
    EXPIRES_ON = 'exp'
    JWT_ID = 'jti'


class UserRealm(object):

    federation_protocol_type = {
        'WSFederation' : 'wstrust',
        'SAML2' : 'saml20',
        'Unknown' : 'unknown'
    }

    account_type = {
        'Federated' : 'federated',
        'Managed' : 'managed',
        'Unknown' : 'unknown'
    }


class Saml(object):

    TokenTypeV1 = 'urn:oasis:names:tc:SAML:1.0:assertion'
    TokenTypeV2 = 'urn:oasis:names:tc:SAML:2.0:assertion'
    OasisWssSaml11TokenProfile11 = "http://docs.oasis-open.org/wss/oasis-wss-saml-token-profile-1.1#SAMLV1.1"
    OasisWssSaml2TokenProfile2 = "http://docs.oasis-open.org/wss/oasis-wss-saml-token-profile-1.1#SAMLV2.0"


class XmlNamespaces(object):
    namespaces = {
        'wsdl'   :'http://schemas.xmlsoap.org/wsdl/',
        'sp'     :'http://docs.oasis-open.org/ws-sx/ws-securitypolicy/200702',
        'sp2005' :'http://schemas.xmlsoap.org/ws/2005/07/securitypolicy',
        'wsu'    :'http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-utility-1.0.xsd',
        'wsa10'  :'http://www.w3.org/2005/08/addressing',
        'http'   :'http://schemas.microsoft.com/ws/06/2004/policy/http',
        'soap12' :'http://schemas.xmlsoap.org/wsdl/soap12/',
        'wsp'    :'http://schemas.xmlsoap.org/ws/2004/09/policy',
        's'      :'http://www.w3.org/2003/05/soap-envelope',
        'wsa'    :'http://www.w3.org/2005/08/addressing',
        'wst'    :'http://docs.oasis-open.org/ws-sx/ws-trust/200512',
        'trust'  : "http://docs.oasis-open.org/ws-sx/ws-trust/200512",
        'saml'   : "urn:oasis:names:tc:SAML:1.0:assertion",
        't'      : 'http://schemas.xmlsoap.org/ws/2005/02/trust'
    }


class Cache(object):

    HASH_ALGORITHM = 'sha256'


class HttpError(object):

    UNAUTHORIZED = 401


class AADConstants(object):

    WORLD_WIDE_AUTHORITY = 'login.microsoftonline.com'
    WELL_KNOWN_AUTHORITY_HOSTS = [
        'login.windows.net',
        'login.microsoftonline.com',
        'login.chinacloudapi.cn',
        'login.microsoftonline.us',
        'login.microsoftonline.de',
        ]
    INSTANCE_DISCOVERY_ENDPOINT_TEMPLATE = 'https://{authorize_host}/common/discovery/instance?authorization_endpoint={authorize_endpoint}&api-version=1.0' # pylint: disable=invalid-name
    AUTHORIZE_ENDPOINT_PATH = '/oauth2/authorize'
    TOKEN_ENDPOINT_PATH = '/oauth2/token'
    DEVICE_ENDPOINT_PATH = '/oauth2/devicecode'


class AdalIdParameters(object):

    SKU = 'x-client-SKU'
    VERSION = 'x-client-Ver'
    OS = 'x-client-OS'  # pylint: disable=invalid-name
    CPU = 'x-client-CPU'
    PYTHON_SKU = 'Python'

class WSTrustVersion(object):
    UNDEFINED = 'undefined'
    WSTRUST13 = 'wstrust13'
    WSTRUST2005 = 'wstrust2005'
 
