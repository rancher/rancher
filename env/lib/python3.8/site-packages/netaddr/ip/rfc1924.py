#-----------------------------------------------------------------------------
#   Copyright (c) 2008 by David P. D. Moss. All rights reserved.
#
#   Released under the BSD license. See the LICENSE file for details.
#-----------------------------------------------------------------------------
"""A basic implementation of RFC 1924 ;-)"""

from netaddr.core import AddrFormatError
from netaddr.ip import IPAddress

from netaddr.compat import _zip


def chr_range(low, high):
    """Returns all characters between low and high chars."""
    return [chr(i) for i in range(ord(low), ord(high) + 1)]

#: Base 85 integer index to character lookup table.
BASE_85 = (
    chr_range('0', '9') + chr_range('A', 'Z') +
    chr_range('a', 'z') +
    ['!', '#', '$', '%', '&', '(', ')', '*', '+', '-', ';', '<', '=', '>',
     '?', '@', '^', '_', '`', '{', '|', '}', '~']
)

#: Base 85 digit to integer lookup table.
BASE_85_DICT = dict(_zip(BASE_85, range(0, 86)))


def ipv6_to_base85(addr):
    """Convert a regular IPv6 address to base 85."""
    ip = IPAddress(addr)
    int_val = int(ip)

    remainder = []
    while int_val > 0:
        remainder.append(int_val % 85)
        int_val //= 85

    encoded = ''.join([BASE_85[w] for w in reversed(remainder)])
    leading_zeroes = (20 - len(encoded)) * "0"
    return leading_zeroes + encoded


def base85_to_ipv6(addr):
    """
    Convert a base 85 IPv6 address to its hexadecimal format.
    """
    tokens = list(addr)

    if len(tokens) != 20:
        raise AddrFormatError('Invalid base 85 IPv6 address: %r' % addr)

    result = 0
    for i, num in enumerate(reversed(tokens)):
        num = BASE_85_DICT[num]
        result += (num * 85 ** i)

    ip = IPAddress(result, 6)

    return str(ip)
