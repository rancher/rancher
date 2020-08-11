#-----------------------------------------------------------------------------
#   Copyright (c) 2008 by David P. D. Moss. All rights reserved.
#
#   Released under the BSD license. See the LICENSE file for details.
#-----------------------------------------------------------------------------
"""
IEEE 64-bit EUI (Extended Unique Indentifier) logic.
"""
import struct as _struct
import re as _re

from netaddr.core import AddrFormatError
from netaddr.strategy import (
    valid_words as _valid_words, int_to_words as _int_to_words,
    words_to_int as _words_to_int, valid_bits as _valid_bits,
    bits_to_int as _bits_to_int, int_to_bits as _int_to_bits,
    valid_bin as _valid_bin, int_to_bin as _int_to_bin,
    bin_to_int as _bin_to_int)


#   This is a fake constant that doesn't really exist. Here for completeness.
AF_EUI64 = 64

#: The width (in bits) of this address type.
width = 64

#: The AF_* constant value of this address type.
family = AF_EUI64

#: A friendly string name address type.
family_name = 'EUI-64'

#: The version of this address type.
version = 64

#: The maximum integer value that can be represented by this address type.
max_int = 2 ** width - 1

#-----------------------------------------------------------------------------
#   Dialect classes.
#-----------------------------------------------------------------------------

class eui64_base(object):
    """A standard IEEE EUI-64 dialect class."""
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


class eui64_unix(eui64_base):
    """A UNIX-style MAC address dialect class."""
    word_size = 8
    num_words = width // word_size
    word_sep = ':'
    word_fmt = '%x'
    word_base = 16


class eui64_unix_expanded(eui64_unix):
    """A UNIX-style MAC address dialect class with leading zeroes."""
    word_fmt = '%.2x'


class eui64_cisco(eui64_base):
    """A Cisco 'triple hextet' MAC address dialect class."""
    word_size = 16
    num_words = width // word_size
    word_sep = '.'
    word_fmt = '%.4x'
    word_base = 16


class eui64_bare(eui64_base):
    """A bare (no delimiters) MAC address dialect class."""
    word_size = 64
    num_words = width // word_size
    word_sep = ''
    word_fmt = '%.16X'
    word_base = 16


#: The default dialect to be used when not specified by the user.

DEFAULT_EUI64_DIALECT = eui64_base

#-----------------------------------------------------------------------------
#: Regular expressions to match all supported MAC address formats.
RE_EUI64_FORMATS = (
    #   2 bytes x 8 (UNIX, Windows, EUI-64)
    '^' + ':'.join(['([0-9A-F]{1,2})'] * 8) + '$',
    '^' + '-'.join(['([0-9A-F]{1,2})'] * 8) + '$',

    #   4 bytes x 4 (Cisco like)
    '^' + ':'.join(['([0-9A-F]{1,4})'] * 4) + '$',
    '^' + '-'.join(['([0-9A-F]{1,4})'] * 4) + '$',
    '^' + '\.'.join(['([0-9A-F]{1,4})'] * 4) + '$',

    #   16 bytes (bare, no delimiters)
    '^(' + ''.join(['[0-9A-F]'] * 16) + ')$',
)
#   For efficiency, each string regexp converted in place to its compiled
#   counterpart.
RE_EUI64_FORMATS = [_re.compile(_, _re.IGNORECASE) for _ in RE_EUI64_FORMATS]


def _get_match_result(address, formats):
    for regexp in formats:
        match = regexp.findall(address)
        if match:
            return match[0]


def valid_str(addr):
    """
    :param addr: An IEEE EUI-64 indentifier in string form.

    :return: ``True`` if EUI-64 indentifier is valid, ``False`` otherwise.
    """
    try:
        if _get_match_result(addr, RE_EUI64_FORMATS):
            return True
    except TypeError:
        pass

    return False


def str_to_int(addr):
    """
    :param addr: An IEEE EUI-64 indentifier in string form.

    :return: An unsigned integer that is equivalent to value represented
        by EUI-64 string address formatted according to the dialect
    """
    words = []

    try:
        words = _get_match_result(addr, RE_EUI64_FORMATS)
        if not words:
            raise TypeError
    except TypeError:
        raise AddrFormatError('invalid IEEE EUI-64 identifier: %r!' % addr)

    if isinstance(words, tuple):
        pass
    else:
        words = (words,)

    if len(words) == 8:
        #   2 bytes x 8 (UNIX, Windows, EUI-48)
        int_val = int(''.join(['%.2x' % int(w, 16) for w in words]), 16)
    elif len(words) == 4:
        #   4 bytes x 4 (Cisco like)
        int_val = int(''.join(['%.4x' % int(w, 16) for w in words]), 16)
    elif len(words) == 1:
        #   16 bytes (bare, no delimiters)
        int_val = int('%016x' % int(words[0], 16), 16)
    else:
        raise AddrFormatError(
            'bad word count for EUI-64 identifier: %r!' % addr)

    return int_val


def int_to_str(int_val, dialect=None):
    """
    :param int_val: An unsigned integer.

    :param dialect: (optional) a Python class defining formatting options

    :return: An IEEE EUI-64 identifier that is equivalent to unsigned integer.
    """
    if dialect is None:
        dialect = eui64_base
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
    words = int_to_words(int_val)
    return _struct.pack('>8B', *words)


def packed_to_int(packed_int):
    """
    :param packed_int: a packed string containing an unsigned integer.
        It is assumed that string is packed in network byte order.

    :return: An unsigned integer equivalent to value of network address
        represented by packed binary string.
    """
    words = list(_struct.unpack('>8B', packed_int))

    int_val = 0
    for i, num in enumerate(reversed(words)):
        word = num
        word = word << 8 * i
        int_val = int_val | word

    return int_val


def valid_words(words, dialect=None):
    if dialect is None:
        dialect = DEFAULT_EUI64_DIALECT
    return _valid_words(words, dialect.word_size, dialect.num_words)


def int_to_words(int_val, dialect=None):
    if dialect is None:
        dialect = DEFAULT_EUI64_DIALECT
    return _int_to_words(int_val, dialect.word_size, dialect.num_words)


def words_to_int(words, dialect=None):
    if dialect is None:
        dialect = DEFAULT_EUI64_DIALECT
    return _words_to_int(words, dialect.word_size, dialect.num_words)


def valid_bits(bits, dialect=None):
    if dialect is None:
        dialect = DEFAULT_EUI64_DIALECT
    return _valid_bits(bits, width, dialect.word_sep)


def bits_to_int(bits, dialect=None):
    if dialect is None:
        dialect = DEFAULT_EUI64_DIALECT
    return _bits_to_int(bits, width, dialect.word_sep)


def int_to_bits(int_val, dialect=None):
    if dialect is None:
        dialect = DEFAULT_EUI64_DIALECT
    return _int_to_bits(
        int_val, dialect.word_size, dialect.num_words, dialect.word_sep)


def valid_bin(bin_val, dialect=None):
    if dialect is None:
        dialect = DEFAULT_EUI64_DIALECT
    return _valid_bin(bin_val, width)


def int_to_bin(int_val):
    return _int_to_bin(int_val, width)


def bin_to_int(bin_val):
    return _bin_to_int(bin_val, width)
