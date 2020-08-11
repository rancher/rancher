# Copyright 2013 Donald Stufft and individual contributors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

from __future__ import absolute_import, division, print_function

import base64
import binascii


class RawEncoder(object):

    @staticmethod
    def encode(data):
        return data

    @staticmethod
    def decode(data):
        return data


class HexEncoder(object):

    @staticmethod
    def encode(data):
        return binascii.hexlify(data)

    @staticmethod
    def decode(data):
        return binascii.unhexlify(data)


class Base16Encoder(object):

    @staticmethod
    def encode(data):
        return base64.b16encode(data)

    @staticmethod
    def decode(data):
        return base64.b16decode(data)


class Base32Encoder(object):

    @staticmethod
    def encode(data):
        return base64.b32encode(data)

    @staticmethod
    def decode(data):
        return base64.b32decode(data)


class Base64Encoder(object):

    @staticmethod
    def encode(data):
        return base64.b64encode(data)

    @staticmethod
    def decode(data):
        return base64.b64decode(data)


class URLSafeBase64Encoder(object):

    @staticmethod
    def encode(data):
        return base64.urlsafe_b64encode(data)

    @staticmethod
    def decode(data):
        return base64.urlsafe_b64decode(data)


class Encodable(object):

    def encode(self, encoder=RawEncoder):
        return encoder.encode(bytes(self))
