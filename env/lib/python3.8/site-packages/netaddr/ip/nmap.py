#-----------------------------------------------------------------------------
#   Copyright (c) 2008 by David P. D. Moss. All rights reserved.
#
#   Released under the BSD license. See the LICENSE file for details.
#-----------------------------------------------------------------------------
"""
Routines for dealing with nmap-style IPv4 address ranges.

Based on nmap's Target Specification :-

    http://nmap.org/book/man-target-specification.html
"""

from netaddr.core import AddrFormatError
from netaddr.ip import IPAddress, IPNetwork
from netaddr.compat import _iter_range, _is_str, _iter_next


def _nmap_octet_target_values(spec):
    #   Generates sequence of values for an individual octet as defined in the
    #   nmap Target Specification.
    values = set()

    for element in spec.split(','):
        if '-' in element:
            left, right = element.split('-', 1)
            if not left:
                left = 0
            if not right:
                right = 255
            low = int(left)
            high = int(right)
            if not ((0 <= low <= 255) and (0 <= high <= 255)):
                raise ValueError('octet value overflow for spec %s!' % spec)
            if low > high:
                raise ValueError('left side of hyphen must be <= right %r' % element)
            for octet in _iter_range(low, high + 1):
                values.add(octet)
        else:
            octet = int(element)
            if not (0 <= octet <= 255):
                raise ValueError('octet value overflow for spec %s!' % spec)
            values.add(octet)

    return sorted(values)


def _generate_nmap_octet_ranges(nmap_target_spec):
    #   Generate 4 lists containing all octets defined by a given nmap Target
    #   specification.
    if not _is_str(nmap_target_spec):
        raise TypeError('string expected, not %s' % type(nmap_target_spec))

    if not nmap_target_spec:
        raise ValueError('nmap target specification cannot be blank!')

    tokens = nmap_target_spec.split('.')

    if len(tokens) != 4:
        raise AddrFormatError('invalid nmap range: %s' % nmap_target_spec)

    return (_nmap_octet_target_values(tokens[0]),
            _nmap_octet_target_values(tokens[1]),
            _nmap_octet_target_values(tokens[2]),
            _nmap_octet_target_values(tokens[3]))


def _parse_nmap_target_spec(target_spec):
    if '/' in target_spec:
        _, prefix = target_spec.split('/', 1)
        if not (0 < int(prefix) < 33):
            raise AddrFormatError('CIDR prefix expected, not %s' % prefix)
        net = IPNetwork(target_spec)
        if net.version != 4:
            raise AddrFormatError('CIDR only support for IPv4!')
        for ip in net:
            yield ip
    elif ':' in target_spec:
        #   nmap only currently supports IPv6 addresses without prefixes.
        yield IPAddress(target_spec)
    else:
        octet_ranges = _generate_nmap_octet_ranges(target_spec)
        for w in octet_ranges[0]:
            for x in octet_ranges[1]:
                for y in octet_ranges[2]:
                    for z in octet_ranges[3]:
                        yield IPAddress("%d.%d.%d.%d" % (w, x, y, z), 4)


def valid_nmap_range(target_spec):
    """
    :param target_spec: an nmap-style IP range target specification.

    :return: ``True`` if IP range target spec is valid, ``False`` otherwise.
    """
    try:
        _iter_next(_parse_nmap_target_spec(target_spec))
        return True
    except (TypeError, ValueError, AddrFormatError):
        pass
    return False


def iter_nmap_range(*nmap_target_spec):
    """
    An generator that yields IPAddress objects from defined by nmap target
    specifications.

    See https://nmap.org/book/man-target-specification.html for details.

    :param *nmap_target_spec: one or more nmap IP range target specification.

    :return: an iterator producing IPAddress objects for each IP in the target spec(s).
    """
    for target_spec in nmap_target_spec:
        for addr in _parse_nmap_target_spec(target_spec):
            yield addr
