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

import time
import datetime
import uuid
import base64
import binascii
import re

import jwt

from .constants import Jwt
from .log import Logger
from .adal_error import AdalError

def _get_date_now():
    return datetime.datetime.now()

def _get_new_jwt_id():
    return str(uuid.uuid4())

def _create_x5t_value(thumbprint):
    hex_val = binascii.a2b_hex(thumbprint)
    return base64.urlsafe_b64encode(hex_val).decode()

def _sign_jwt(header, payload, certificate):
    try:
        encoded_jwt = _encode_jwt(payload, certificate, header)
    except Exception as exp:
        raise AdalError("Error:Invalid Certificate: Expected Start of Certificate to be '-----BEGIN RSA PRIVATE KEY-----'", exp)
    _raise_on_invalid_jwt_signature(encoded_jwt)
    return encoded_jwt

def _encode_jwt(payload, certificate, header):
    return jwt.encode(payload, certificate, algorithm='RS256', headers=header).decode()

def _raise_on_invalid_jwt_signature(encoded_jwt):
    segments = encoded_jwt.split('.')
    if len(segments) < 3 or not segments[2]:    
        raise AdalError('Failed to sign JWT. This is most likely due to an invalid certificate.')

def _extract_certs(public_cert_content):
    # Parses raw public certificate file contents and returns a list of strings
    # Usage: headers = {"x5c": extract_certs(open("my_cert.pem").read())}
    public_certificates = re.findall(
        r'-----BEGIN CERTIFICATE-----(?P<cert_value>[^-]+)-----END CERTIFICATE-----',
        public_cert_content, re.I)
    if public_certificates:
        return [cert.strip() for cert in public_certificates]
    # The public cert tags are not found in the input,
    # let's make best effort to exclude a private key pem file.
    if "PRIVATE KEY" in public_cert_content:
        raise ValueError(
            "We expect your public key but detect a private key instead")
    return [public_cert_content.strip()]

class SelfSignedJwt(object):

    NumCharIn128BitHexString = 128/8*2
    numCharIn160BitHexString = 160/8*2
    ThumbprintRegEx = r"^[a-f\d]*$"

    def __init__(self, call_context, authority, client_id):
        self._log = Logger('SelfSignedJwt', call_context['log_context'])
        self._call_context = call_context

        self._authortiy = authority
        self._token_endpoint = authority.token_endpoint
        self._client_id = client_id

    def _create_header(self, thumbprint, public_certificate):
        x5t = _create_x5t_value(thumbprint)
        header = {'typ':'JWT', 'alg':'RS256', 'x5t':x5t}
        if public_certificate:
            header['x5c'] = _extract_certs(public_certificate)
        self._log.debug("Creating self signed JWT header. x5t: %(x5t)s, x5c: %(x5c)s",
                        {"x5t": x5t, "x5c": public_certificate})

        return header

    def _create_payload(self):
        now = _get_date_now()
        minutes = datetime.timedelta(0, 0, 0, 0, Jwt.SELF_SIGNED_JWT_LIFETIME)
        expires = now + minutes

        self._log.debug(
            'Creating self signed JWT payload. Expires: %(expires)s NotBefore: %(nbf)s',
            {"expires": expires, "nbf": now})

        jwt_payload = {}
        jwt_payload[Jwt.AUDIENCE] = self._token_endpoint
        jwt_payload[Jwt.ISSUER] = self._client_id
        jwt_payload[Jwt.SUBJECT] = self._client_id
        jwt_payload[Jwt.NOT_BEFORE] = int(time.mktime(now.timetuple()))
        jwt_payload[Jwt.EXPIRES_ON] = int(time.mktime(expires.timetuple()))
        jwt_payload[Jwt.JWT_ID] = _get_new_jwt_id()

        return jwt_payload

    def _raise_on_invalid_thumbprint(self, thumbprint):
        thumbprint_sizes = [self.NumCharIn128BitHexString, self.numCharIn160BitHexString]
        size_ok = len(thumbprint) in thumbprint_sizes
        if not size_ok or not re.search(self.ThumbprintRegEx, thumbprint):
            raise AdalError("The thumbprint does not match a known format")

    def _reduce_thumbprint(self, thumbprint):
        canonical = thumbprint.lower().replace(' ', '').replace(':', '')
        self._raise_on_invalid_thumbprint(canonical)
        return canonical

    def create(self, certificate, thumbprint, public_certificate):
        thumbprint = self._reduce_thumbprint(thumbprint)

        header = self._create_header(thumbprint, public_certificate)
        payload = self._create_payload()
        return _sign_jwt(header, payload, certificate)
