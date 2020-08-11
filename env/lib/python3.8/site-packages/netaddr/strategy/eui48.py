#-----------------------------------------------------------------------------
#   Copyright (c) 2008 by David P. D. Moss. All rights reserved.
#
#   Released under the BSD license. See the LICENSE file for details.
#-----------------------------------------------------------------------------
"""
IEEE 48-bit EUI (MAC address) logic.

Supports numerous MAC string formats including Cisco's triple hextet as well
as bare MACs containing no delimiters.
"""
import struct as _struct
import re as _re

#   Check whether we need to use fallback code or not.
try:
    from socket import AF_LINK
except ImportError:
    AF_LINK = 48

from netaddr.core import AddrFormatError
from netaddr.compat import _is_str
from netaddr.strategy import (
    valid_words as _valid_words, int_to_words as _int_to_words,
    words_to_int as _words_to_int, valid_bits as _valid_bits,
    bits_to_int as _bits_to_int, int_to_bits as _int_to_bits,
    valid_bin as _valid_bin, int_to_bin as _int_to_bin,
    bin_to_int as _bin_to_int)

#: The width (in bits) of this address type.
width = 48

#: The AF_* constant value of this address type.
family = AF_LINK

#: A friendly string name address type.
family_name = 'MAC'

#: The version of this address type.
version = 48

#: The maximum integer value that can be represented by this address type.
max_int = 2 ** width - 1

#-----------------------------------------------------------------------------
#   Dialect classes.
#-----------------------------------------------------------------------------

class mac_eui48(object):
    """A standard IEEE EUI-48 dialect class."""
    #: The individual word size (in bits) of this address type.
    word_size = 8

    #: The number of words in this address type.
    num_words = width // word_size

    #: The maximum integer value for an individual word in this address type.
    max_word = 2 ** word_size - 1

    #: The separator character used between each word.
    word_sep = '-'

    #: The format string to be used when converting words to string values.
    word_fmt = '%.2X'

    #: The number base to be used when interpreting word values as integers.
    word_base = 16


class mac_unix(mac_eui48):
    """A UNIX-style MAC address dialect class."""
    word_size = 8
    num_words = width // word_size
    word_sep = ':'
    word_fmt = '%x'
    word_base = 16


class mac_unix_expanded(mac_unix):
    """A UNIX-style MAC address dialect class with leading zeroes."""
    word_fmt = '%.2x'


class mac_cisco(mac_eui48):
    """A Cisco 'triple hextet' MAC address dialect class."""
    word_size = 16
    num_words = width // word_size
    word_sep = '.'
    word_fmt = '%.4x'
    word_base = 16


class mac_bare(mac_eui48):
    """A bare (no delimiters) MAC address dialect class."""
    word_size = 48
    num_words = width // word_size
    word_sep = ''
    word_fmt = '%.12X'
    word_base = 16


class mac_pgsql(mac_eui48):
    """A PostgreSQL style (2 x 24-bit words) MAC address dialect class."""
    word_size = 24
    num_words = width // word_size
    word_sep = ':'
    word_fmt = '%.6x'
    word_base = 16

#: The default dialect to be used when not specified by the user.
DEFAULT_DIALECT = mac_eui48

#-----------------------------------------------------------------------------
#: Regular expressions to match all supported MAC address formats.
RE_MAC_FORMATS = (
    #   2 bytes x 6 (UNIX, Windows, EUI-48)
    '^' + ':'.join(['([0-9A-F]{1,2})'] * 6) + '$',
    '^' + '-'.join(['([0-9A-F]{1,2})'] * 6) + '$',

    #   4 bytes x 3 (Cisco)
    '^' + ':'.join(['([0-9A-F]{1,4})'] * 3) + '$',
    '^' + '-'.join(['([0-9A-F]{1,4})'] * 3) + '$',
    '^' + '\.'.join(['([0-9A-F]{1,4})'] * 3) + '$',

    #   6 bytes x 2 (PostgreSQL)
    '^' + '-'.join(['([0-9A-F]{5,6})'] * 2) + '$',
    '^' + ':'.join(['([0-9A-F]{5,6})'] * 2) + '$',

    #   12 bytes (bare, no delimiters)
    '^(' + ''.join(['[0-9A-F]'] * 12) + ')$',
    '^(' + ''.join(['[0-9A-F]'] * 11) + ')$',
)
#   For efficiency, each string regexp converted in place to its compiled
#   counterpart.
RE_MAC_FORMATS = [_re.compile(_, _re.IGNORECASE) for _ in RE_MAC_FORMATS]


def valid_str(addr):
    """
    :param addr: An IEEE EUI-48 (MAC) address in string form.

    :return: ``True`` if MAC address string is valid, ``False`` otherwise.
    """
    for regexp in RE_MAC_FORMATS:
        try:
            match_result = regexp.findall(addr)
            if len(match_result) != 0:
                return True
        except TypeError:
            pass

    return False


def str_to_int(addr):
    """
    :param addr: An IEEE EUI-48 (MAC) address in string form.

    :return: An unsigned integer that is equivalent to value represented
        by EUI-48/MAC string address formatted according to the dialect
        settings.
    """
    words = []
    if _is_str(addr):
        found_match = False
        for regexp in RE_MAC_FORMATS:
            match_result = regexp.findall(addr)
            if len(match_result) != 0:
                found_match = True
                if isinstance(match_result[0], tuple):
                    words = match_result[0]
                else:
                    words = (match_result[0],)
                break
        if not found_match:
            raise AddrFormatError('%r is not a supported MAC format!' % addr)
    else:
        raise TypeError('%r is not str() or unicode()!' % addr)

    int_val = None

    if len(words) == 6:
        #   2 bytes x 6 (UNIX, Windows, EUI-48)
        int_val = int(''.join(['%.2x' % int(w, 16) for w in words]), 16)
    elif len(words) == 3:
        #   4 bytes x 3 (Cisco)
        int_val = int(''.join(['%.4x' % int(w, 16) for w in words]), 16)
    elif len(words) == 2:
        #   6 bytes x 2 (PostgreSQL)
        int_val = int(''.join(['%.6x' % int(w, 16) for w in words]), 16)
    elif len(words) == 1:
        #   12 bytes (bare, no delimiters)
        int_val = int('%012x' % int(words[0], 16), 16)
    else:
        raise AddrFormatError('unexpected word count in MAC address %r!' % addr)

    return int_val


def int_to_str(int_val, dialect=None):
    """
    :param int_val: An unsigned integer.

    :param dialect: (optional) a Python class defining formatting options.

    :return: An IEEE EUI-48 (MAC) address string that is equivalent to
        unsigned integer formatted according to the dialect settings.
    """
    if dialect is None:
        dialect = mac_eui48

    words = int_to_words(int_val, dialect)
    tokens = [dialect.word_fmt % i for i in words]
    addr = dialect.word_sep.join(tokens)

    return addr


def int_to_packed(int_val):
    """
    :param int_val: the integer to be packed.

    :return: a packed string that is equivalent to value represented by an
    unsigned integer.
    """
    return _struct.pack(">HI", int_val >> 32, int_val & 0xffffffff)


def packed_to_int(packed_int):
    """
    :param packed_int: a packed string containing an unsigned integer.
        It is assumed that string is packed in network byte order.

    :return: An unsigned integer equivalent to value of network address
        represented by packed binary string.
    """
    words = list(_struct.unpack('>6B', packed_int))

    int_val = 0
    for i, num in enumerate(reversed(words)):
        word = num
        word = word << 8 * i
        int_val = int_val | word

    return int_val


def valid_words(words, dialect=None):
    if dialect is None:
        dialect = DEFAULT_DIALECT
    return _valid_words(words, dialect.word_size, dialect.num_words)


def int_to_words(int_val, dialect=None):
    if dialect is None:
        dialect = DEFAULT_DIALECT
    return _int_to_words(int_val, dialect.word_size, dialect.num_words)


def words_to_int(words, dialect=None):
    if dialect is None:
        dialect = DEFAULT_DIALECT
    return _words_to_int(words, dialect.word_size, dialect.num_words)


def valid_bits(bits, dialect=None):
    if dialect is None:
        dialect = DEFAULT_DIALECT
    return _valid_bits(bits, width, dialect.word_sep)


def bits_to_int(bits, dialect=None):
    if dialect is None:
        dialect = DEFAULT_DIALECT
    return _bits_to_int(bits, width, dialect.word_sep)


def int_to_bits(int_val, dialect=None):
    if dialect is None:
        dialect = DEFAULT_DIALECT
    return _int_to_bits(
        int_val, dialect.word_size, dialect.num_words, dialect.word_sep)


def valid_bin(bin_val, dialect=None):
    if dialect is None:
        dialect = DEFAULT_DIALECT
    return _valid_bin(bin_val, width)


def int_to_bin(int_val):
    return _int_to_bin(int_val, width)


def bin_to_int(bin_val):
    return _bin_to_int(bin_val, width)
