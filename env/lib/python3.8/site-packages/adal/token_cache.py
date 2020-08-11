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
import threading

from .constants import TokenResponseFields

def _string_cmp(str1, str2):
    '''Case insensitive comparison. Return true if both are None'''
    str1 = str1 if str1 is not None else ''
    str2 = str2 if str2 is not None else ''
    return str1.lower() == str2.lower()

class TokenCacheKey(object): # pylint: disable=too-few-public-methods
    def __init__(self, authority, resource, client_id, user_id):
        self.authority = authority
        self.resource = resource
        self.client_id = client_id
        self.user_id = user_id

    def __hash__(self):
        return hash((self.authority, self.resource, self.client_id, self.user_id))

    def __eq__(self, other):
        return _string_cmp(self.authority, other.authority) and \
               _string_cmp(self.resource, other.resource) and \
               _string_cmp(self.client_id, other.client_id) and \
               _string_cmp(self.user_id, other.user_id)

    def __ne__(self, other):
        return not self == other

# pylint: disable=protected-access

def _get_cache_key(entry):
    return TokenCacheKey(
        entry.get(TokenResponseFields._AUTHORITY), 
        entry.get(TokenResponseFields.RESOURCE), 
        entry.get(TokenResponseFields._CLIENT_ID), 
        entry.get(TokenResponseFields.USER_ID))


class TokenCache(object):
    def __init__(self, state=None):
        self._cache = {}
        self._lock = threading.RLock()
        if state:
            self.deserialize(state)
        self.has_state_changed = False

    def find(self, query):
        with self._lock:
            return self._query_cache(
                query.get(TokenResponseFields.IS_MRRT), 
                query.get(TokenResponseFields.USER_ID), 
                query.get(TokenResponseFields._CLIENT_ID))

    def remove(self, entries):
        with self._lock:
            for e in entries:
                key = _get_cache_key(e)
                removed = self._cache.pop(key, None)
                if removed is not None:
                    self.has_state_changed = True

    def add(self, entries):
        with self._lock:
            for e in entries:
                key = _get_cache_key(e)
                self._cache[key] = e
            self.has_state_changed = True

    def serialize(self):
        with self._lock:
            return json.dumps(list(self._cache.values()))

    def deserialize(self, state):
        with self._lock:
            self._cache.clear()
            if state:
                tokens = json.loads(state)
                for t in tokens:
                    key = _get_cache_key(t)
                    self._cache[key] = t

    def read_items(self):
        '''output list of tuples in (key, authentication-result)'''
        with self._lock:
            return self._cache.items()

    def _query_cache(self, is_mrrt, user_id, client_id):
        matches = []
        for k in self._cache:
            v = self._cache[k]
            #None value will be taken as wildcard match
            #pylint: disable=too-many-boolean-expressions
            if ((is_mrrt is None or is_mrrt == v.get(TokenResponseFields.IS_MRRT)) and 
                    (user_id is None or _string_cmp(user_id, v.get(TokenResponseFields.USER_ID))) and 
                    (client_id is None or _string_cmp(client_id, v.get(TokenResponseFields._CLIENT_ID)))):
                matches.append(v)
        return matches
