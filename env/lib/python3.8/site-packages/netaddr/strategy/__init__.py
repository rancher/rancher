#-----------------------------------------------------------------------------
#   Copyright (c) 2008 by David P. D. Moss. All rights reserved.
#
#   Released under the BSD license. See the LICENSE file for details.
#-----------------------------------------------------------------------------
"""
Shared logic for various address types.
"""
import re as _re

from netaddr.compat import _range, _is_str


def bytes_to_bits():
    """
    :return: A 256 element list containing 8-bit binary digit strings. The
        list index value is equivalent to its bit string value.
    """
    lookup = []
    bits_per_byte = _range(7, -1, -1)
    for num in range(256):
        bits = 8 * [None]
        for i in bits_per_byte:
            bits[i] = '01'[num & 1]
            num >>= 1
        lookup.append(''.join(bits))
    return lookup

#: A lookup table of 8-bit integer values to their binary digit bit strings.
BYTES_TO_BITS = bytes_to_bits()


def valid_words(words, word_size, num_words):
    """
    :param words: A sequence of unsigned integer word values.

    :param word_size: Width (in bits) of each unsigned integer word value.

    :param num_words: Number of unsigned integer words expected.

    :return: ``True`` if word sequence is valid for this address type,
        ``False`` otherwise.
    """
    if not hasattr(words, '__iter__'):
        return False

    if len(words) != num_words:
        return False

    max_word = 2 ** word_size - 1

    for i in words:
        if not 0 <= i <= max_word:
            return False

    return True


def int_to_words(int_val, word_size, num_words):
    """
    :param int_val: Unsigned integer to be divided into words of equal size.

    :param word_size: Width (in bits) of each unsigned integer word value.

    :param num_words: Number of unsigned integer words expected.

    :return: A tuple contain unsigned integer word values split according
        to provided arguments.
    """
    max_int = 2 ** (num_words * word_size) - 1

    if not 0 <= int_val <= max_int:
        raise IndexError('integer out of bounds: %r!' % hex(int_val))

    max_word = 2 ** word_size - 1

    words = []
    for _ in range(num_words):
        word = int_val & max_word
        words.append(int(word))
        int_val >>= word_size

    return tuple(reversed(words))


def words_to_int(words, word_size, num_words):
    """
    :param words: A sequence of unsigned integer word values.

    :param word_size: Width (in bits) of each unsigned integer word value.

    :param num_words: Number of unsigned integer words expected.

    :return: An unsigned integer that is equivalent to value represented
        by word sequence.
    """
    if not valid_words(words, word_size, num_words):
        raise ValueError('invalid integer word sequence: %r!' % words)

    int_val = 0
    for i, num in enumerate(reversed(words)):
        word = num
        word = word << word_size * i
        int_val = int_val | word

    return int_val


def valid_bits(bits, width, word_sep=''):
    """
    :param bits: A network address in a delimited binary string format.

    :param width: Maximum width (in bits) of a network address (excluding
        delimiters).

    :param word_sep: (optional) character or string used to delimit word
        groups (default: '', no separator).

    :return: ``True`` if network address is valid, ``False`` otherwise.
    """
    if not _is_str(bits):
        return False

    if word_sep != '':
        bits = bits.replace(word_sep, '')

    if len(bits) != width:
        return False

    max_int = 2 ** width - 1

    try:
        if 0 <= int(bits, 2) <= max_int:
            return True
    except ValueError:
        pass

    return False


def bits_to_int(bits, width, word_sep=''):
    """
    :param bits: A network address in a delimited binary string format.

    :param width: Maximum width (in bits) of a network address (excluding
        delimiters).

    :param word_sep: (optional) character or string used to delimit word
        groups (default: '', no separator).

    :return: An unsigned integer that is equivalent to value represented
        by network address in readable binary form.
    """
    if not valid_bits(bits, width, word_sep):
        raise ValueError('invalid readable binary string: %r!' % bits)

    if word_sep != '':
        bits = bits.replace(word_sep, '')

    return int(bits, 2)


def int_to_bits(int_val, word_size, num_words, word_sep=''):
    """
    :param int_val: An unsigned integer.

    :param word_size: Width (in bits) of each unsigned integer word value.

    :param num_words: Number of unsigned integer words expected.

    :param word_sep: (optional) character or string used to delimit word
        groups (default: '', no separator).

    :return: A network address in a delimited binary string format that is
        equivalent in value to unsigned integer.
    """
    bit_words = []

    for word in int_to_words(int_val, word_size, num_words):
        bits = []
        while word:
            bits.append(BYTES_TO_BITS[word & 255])
            word >>= 8
        bits.reverse()
        bit_str = ''.join(bits) or '0' * word_size
        bits = ('0' * word_size + bit_str)[-word_size:]
        bit_words.append(bits)

    if word_sep is not '':
        #   Check custom separator.
        if not _is_str(word_sep):
            raise ValueError('word separator is not a string: %r!' % word_sep)

    return word_sep.join(bit_words)


def valid_bin(bin_val, width):
    """
    :param bin_val: A network address in Python's binary representation format
        ('0bxxx').

    :param width: Maximum width (in bits) of a network address (excluding
        delimiters).

    :return: ``True`` if network address is valid, ``False`` otherwise.
    """
    if not _is_str(bin_val):
        return False

    if not bin_val.startswith('0b'):
        return False

    bin_val = bin_val.replace('0b', '')

    if len(bin_val) > width:
        return False

    max_int = 2 ** width - 1

    try:
        if 0 <= int(bin_val, 2) <= max_int:
            return True
    except ValueError:
        pass

    return False


def int_to_bin(int_val, width):
    """
    :param int_val: An unsigned integer.

    :param width: Maximum allowed width (in bits) of a unsigned integer.

    :return: Equivalent string value in Python's binary representation format
        ('0bxxx').
    """
    bin_tokens = []

    try:
        #   Python 2.6.x and upwards.
        bin_val = bin(int_val)
    except NameError:
        #   Python 2.4.x and 2.5.x
        i = int_val
        while i > 0:
            word = i & 0xff
            bin_tokens.append(BYTES_TO_BITS[word])
            i >>= 8

        bin_tokens.reverse()
        bin_val = '0b' + _re.sub(r'^[0]+([01]+)$', r'\1', ''.join(bin_tokens))

    if len(bin_val[2:]) > width:
        raise IndexError('binary string out of bounds: %s!' % bin_val)

    return bin_val


def bin_to_int(bin_val, width):
    """
    :param bin_val: A string containing an unsigned integer in Python's binary
        representation format ('0bxxx').

    :param width: Maximum allowed width (in bits) of a unsigned integer.

    :return: An unsigned integer that is equivalent to value represented
        by Python binary string format.
    """
    if not valid_bin(bin_val, width):
        raise ValueError('not a valid Python binary string: %r!' % bin_val)

    return int(bin_val.replace('0b', ''), 2)
