# Copyright 2012-2014 Amazon.com, Inc. or its affiliates. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License"). You
# may not use this file except in compliance with the License. A copy of
# the License is located at
#
# http://aws.amazon.com/apache2.0/
#
# or in the "license" file accompanying this file. This file is
# distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
# ANY KIND, either express or implied. See the License for the specific
# language governing permissions and limitations under the License.

import copy
import datetime
import sys
import inspect
import warnings
import hashlib
import logging
import shlex
from math import floor

from botocore.vendored import six
from botocore.exceptions import MD5UnavailableError
from urllib3 import exceptions

logger = logging.getLogger(__name__)


if six.PY3:
    from botocore.vendored.six.moves import http_client

    class HTTPHeaders(http_client.HTTPMessage):
        pass

    from urllib.parse import quote
    from urllib.parse import urlencode
    from urllib.parse import unquote
    from urllib.parse import unquote_plus
    from urllib.parse import urlparse
    from urllib.parse import urlsplit
    from urllib.parse import urlunsplit
    from urllib.parse import urljoin
    from urllib.parse import parse_qsl
    from urllib.parse import parse_qs
    from http.client import HTTPResponse
    from io import IOBase as _IOBase
    from base64 import encodebytes
    from email.utils import formatdate
    from itertools import zip_longest
    file_type = _IOBase
    zip = zip

    # In python3, unquote takes a str() object, url decodes it,
    # then takes the bytestring and decodes it to utf-8.
    # Python2 we'll have to do this ourself (see below).
    unquote_str = unquote_plus

    def set_socket_timeout(http_response, timeout):
        """Set the timeout of the socket from an HTTPResponse.

        :param http_response: An instance of ``httplib.HTTPResponse``

        """
        http_response._fp.fp.raw._sock.settimeout(timeout)

    def accepts_kwargs(func):
        # In python3.4.1, there's backwards incompatible
        # changes when using getargspec with functools.partials.
        return inspect.getfullargspec(func)[2]

    def ensure_unicode(s, encoding=None, errors=None):
        # NOOP in Python 3, because every string is already unicode
        return s

    def ensure_bytes(s, encoding='utf-8', errors='strict'):
        if isinstance(s, str):
            return s.encode(encoding, errors)
        if isinstance(s, bytes):
            return s
        raise ValueError("Expected str or bytes, received %s." % type(s))

else:
    from urllib import quote
    from urllib import urlencode
    from urllib import unquote
    from urllib import unquote_plus
    from urlparse import urlparse
    from urlparse import urlsplit
    from urlparse import urlunsplit
    from urlparse import urljoin
    from urlparse import parse_qsl
    from urlparse import parse_qs
    from email.message import Message
    from email.Utils import formatdate
    file_type = file
    from itertools import izip as zip
    from itertools import izip_longest as zip_longest
    from httplib import HTTPResponse
    from base64 import encodestring as encodebytes

    class HTTPHeaders(Message):

        # The __iter__ method is not available in python2.x, so we have
        # to port the py3 version.
        def __iter__(self):
            for field, value in self._headers:
                yield field

    def unquote_str(value, encoding='utf-8'):
        # In python2, unquote() gives us a string back that has the urldecoded
        # bits, but not the unicode parts.  We need to decode this manually.
        # unquote has special logic in which if it receives a unicode object it
        # will decode it to latin1.  This is hard coded.  To avoid this, we'll
        # encode the string with the passed in encoding before trying to
        # unquote it.
        byte_string = value.encode(encoding)
        return unquote_plus(byte_string).decode(encoding)

    def set_socket_timeout(http_response, timeout):
        """Set the timeout of the socket from an HTTPResponse.

        :param http_response: An instance of ``httplib.HTTPResponse``

        """
        http_response._fp.fp._sock.settimeout(timeout)

    def accepts_kwargs(func):
        return inspect.getargspec(func)[2]

    def ensure_unicode(s, encoding='utf-8', errors='strict'):
        if isinstance(s, six.text_type):
            return s
        return unicode(s, encoding, errors)

    def ensure_bytes(s, encoding='utf-8', errors='strict'):
        if isinstance(s, unicode):
            return s.encode(encoding, errors)
        if isinstance(s, str):
            return s
        raise ValueError("Expected str or unicode, received %s." % type(s))

try:
    from collections import OrderedDict
except ImportError:
    # Python2.6 we use the 3rd party back port.
    from ordereddict import OrderedDict


if sys.version_info[:2] == (2, 6):
    import simplejson as json
    # In py26, invalid xml parsed by element tree
    # will raise a plain old SyntaxError instead of
    # a real exception, so we need to abstract this change.
    XMLParseError = SyntaxError

    # Handle https://github.com/shazow/urllib3/issues/497 for py2.6.  In
    # python2.6, there is a known issue where sometimes we cannot read the SAN
    # from an SSL cert (http://bugs.python.org/issue13034).  However, newer
    # versions of urllib3 will warn you when there is no SAN.  While we could
    # just turn off this warning in urllib3 altogether, we _do_ want warnings
    # when they're legitimate warnings.  This method tries to scope the warning
    # filter to be as specific as possible.
    def filter_ssl_san_warnings():
        warnings.filterwarnings(
            'ignore',
            message="Certificate has no.*subjectAltName.*",
            category=exceptions.SecurityWarning,
            module=r".*urllib3\.connection")
else:
    import xml.etree.cElementTree
    XMLParseError = xml.etree.cElementTree.ParseError
    import json

    def filter_ssl_san_warnings():
        # Noop for non-py26 versions.  We will parse the SAN
        # appropriately.
        pass


def filter_ssl_warnings():
    # Ignore warnings related to SNI as it is not being used in validations.
    warnings.filterwarnings(
        'ignore',
        message="A true SSLContext object is not available.*",
        category=exceptions.InsecurePlatformWarning,
        module=r".*urllib3\.util\.ssl_")
    filter_ssl_san_warnings()


@classmethod
def from_dict(cls, d):
    new_instance = cls()
    for key, value in d.items():
        new_instance[key] = value
    return new_instance


@classmethod
def from_pairs(cls, pairs):
    new_instance = cls()
    for key, value in pairs:
        new_instance[key] = value
    return new_instance

HTTPHeaders.from_dict = from_dict
HTTPHeaders.from_pairs = from_pairs


def copy_kwargs(kwargs):
    """
    There is a bug in Python versions < 2.6.5 that prevents you
    from passing unicode keyword args (#4978).  This function
    takes a dictionary of kwargs and returns a copy.  If you are
    using Python < 2.6.5, it also encodes the keys to avoid this bug.
    Oh, and version_info wasn't a namedtuple back then, either!
    """
    vi = sys.version_info
    if vi[0] == 2 and vi[1] <= 6 and vi[3] < 5:
        copy_kwargs = {}
        for key in kwargs:
            copy_kwargs[key.encode('utf-8')] = kwargs[key]
    else:
        copy_kwargs = copy.copy(kwargs)
    return copy_kwargs


def total_seconds(delta):
    """
    Returns the total seconds in a ``datetime.timedelta``.

    Python 2.6 does not have ``timedelta.total_seconds()``, so we have
    to calculate this ourselves. On 2.7 or better, we'll take advantage of the
    built-in method.

    The math was pulled from the ``datetime`` docs
    (http://docs.python.org/2.7/library/datetime.html#datetime.timedelta.total_seconds).

    :param delta: The timedelta object
    :type delta: ``datetime.timedelta``
    """
    if sys.version_info[:2] != (2, 6):
        return delta.total_seconds()

    day_in_seconds = delta.days * 24 * 3600.0
    micro_in_seconds = delta.microseconds / 10.0**6
    return day_in_seconds + delta.seconds + micro_in_seconds


# Checks to see if md5 is available on this system. A given system might not
# have access to it for various reasons, such as FIPS mode being enabled.
try:
    hashlib.md5()
    MD5_AVAILABLE = True
except ValueError:
    MD5_AVAILABLE = False


def get_md5(*args, **kwargs):
    """
    Attempts to get an md5 hashing object.

    :param raise_error_if_unavailable: raise an error if md5 is unavailable on
        this system. If False, None will be returned if it is unavailable.
    :type raise_error_if_unavailable: bool
    :param args: Args to pass to the MD5 constructor
    :param kwargs: Key word arguments to pass to the MD5 constructor
    :return: An MD5 hashing object if available. If it is unavailable, None
        is returned if raise_error_if_unavailable is set to False.
    """
    if MD5_AVAILABLE:
        return hashlib.md5(*args, **kwargs)
    else:
        raise MD5UnavailableError()


def compat_shell_split(s, platform=None):
    if platform is None:
        platform = sys.platform

    if platform == "win32":
        return _windows_shell_split(s)
    else:
        return shlex.split(s)


def _windows_shell_split(s):
    """Splits up a windows command as the built-in command parser would.

    Windows has potentially bizarre rules depending on where you look. When
    spawning a process via the Windows C runtime (which is what python does
    when you call popen) the rules are as follows:

    https://docs.microsoft.com/en-us/cpp/cpp/parsing-cpp-command-line-arguments

    To summarize:

    * Only space and tab are valid delimiters
    * Double quotes are the only valid quotes
    * Backslash is interpreted literally unless it is part of a chain that
      leads up to a double quote. Then the backslashes escape the backslashes,
      and if there is an odd number the final backslash escapes the quote.

    :param s: The command string to split up into parts.
    :return: A list of command components.
    """
    if not s:
        return []

    components = []
    buff = []
    is_quoted = False
    num_backslashes = 0
    for character in s:
        if character == '\\':
            # We can't simply append backslashes because we don't know if
            # they are being used as escape characters or not. Instead we
            # keep track of how many we've encountered and handle them when
            # we encounter a different character.
            num_backslashes += 1
        elif character == '"':
            if num_backslashes > 0:
                # The backslashes are in a chain leading up to a double
                # quote, so they are escaping each other.
                buff.append('\\' * int(floor(num_backslashes / 2)))
                remainder = num_backslashes % 2
                num_backslashes = 0
                if remainder == 1:
                    # The number of backslashes is uneven, so they are also
                    # escaping the double quote, so it needs to be added to
                    # the current component buffer.
                    buff.append('"')
                    continue

            # We've encountered a double quote that is not escaped,
            # so we toggle is_quoted.
            is_quoted = not is_quoted

            # If there are quotes, then we may want an empty string. To be
            # safe, we add an empty string to the buffer so that we make
            # sure it sticks around if there's nothing else between quotes.
            # If there is other stuff between quotes, the empty string will
            # disappear during the joining process.
            buff.append('')
        elif character in [' ', '\t'] and not is_quoted:
            # Since the backslashes aren't leading up to a quote, we put in
            # the exact number of backslashes.
            if num_backslashes > 0:
                buff.append('\\' * num_backslashes)
                num_backslashes = 0

            # Excess whitespace is ignored, so only add the components list
            # if there is anything in the buffer.
            if buff:
                components.append(''.join(buff))
                buff = []
        else:
            # Since the backslashes aren't leading up to a quote, we put in
            # the exact number of backslashes.
            if num_backslashes > 0:
                buff.append('\\' * num_backslashes)
                num_backslashes = 0
            buff.append(character)

    # Quotes must be terminated.
    if is_quoted:
        raise ValueError('No closing quotation in string: %s' % s)

    # There may be some leftover backslashes, so we need to add them in.
    # There's no quote so we add the exact number.
    if num_backslashes > 0:
        buff.append('\\' * num_backslashes)

    # Add the final component in if there is anything in the buffer.
    if buff:
        components.append(''.join(buff))

    return components
