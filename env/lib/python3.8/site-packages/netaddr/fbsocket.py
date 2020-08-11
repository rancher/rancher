#-----------------------------------------------------------------------------
#   Copyright (c) 2008 by David P. D. Moss. All rights reserved.
#
#   Released under the BSD license. See the LICENSE file for details.
#-----------------------------------------------------------------------------
"""Fallback routines for Python's standard library socket module"""

from struct import unpack as _unpack, pack as _pack

from netaddr.compat import _bytes_join, _is_str

AF_INET = 2
AF_INET6 = 10


def inet_ntoa(packed_ip):
    """
    Convert an IP address from 32-bit packed binary format to string format.
    """
    if not _is_str(packed_ip):
        raise TypeError('string type expected, not %s' % str(type(packed_ip)))

    if len(packed_ip) != 4:
        raise ValueError('invalid length of packed IP address string')

    return '%d.%d.%d.%d' % _unpack('4B', packed_ip)


def _compact_ipv6_tokens(tokens):
    new_tokens = []

    positions = []
    start_index = None
    num_tokens = 0

    #   Discover all runs of zeros.
    for idx, token in enumerate(tokens):
        if token == '0':
            if start_index is None:
                start_index = idx
            num_tokens += 1
        else:
            if num_tokens > 1:
                positions.append((num_tokens, start_index))
            start_index = None
            num_tokens = 0

        new_tokens.append(token)

    #   Store any position not saved before loop exit.
    if num_tokens > 1:
        positions.append((num_tokens, start_index))

    #   Replace first longest run with an empty string.
    if len(positions) != 0:
        #   Locate longest, left-most run of zeros.
        positions.sort(key=lambda x: x[1])
        best_position = positions[0]
        for position in positions:
            if position[0] > best_position[0]:
                best_position = position
        #   Replace chosen zero run.
        (length, start_idx) = best_position
        new_tokens = new_tokens[0:start_idx] + [''] + new_tokens[start_idx + length:]

        #   Add start and end blanks so join creates '::'.
        if new_tokens[0] == '':
            new_tokens.insert(0, '')

        if new_tokens[-1] == '':
            new_tokens.append('')

    return new_tokens


def inet_ntop(af, packed_ip):
    """Convert an packed IP address of the given family to string format."""
    if af == AF_INET:
        #   IPv4.
        return inet_ntoa(packed_ip)
    elif af == AF_INET6:
        #   IPv6.
        if len(packed_ip) != 16 or not _is_str(packed_ip):
            raise ValueError('invalid length of packed IP address string')

        tokens = ['%x' % i for i in _unpack('>8H', packed_ip)]

        #   Convert packed address to an integer value.
        words = list(_unpack('>8H', packed_ip))
        int_val = 0
        for i, num in enumerate(reversed(words)):
            word = num
            word = word << 16 * i
            int_val = int_val | word

        if 0xffff < int_val <= 0xffffffff or int_val >> 32 == 0xffff:
            #   IPv4 compatible / mapped IPv6.
            packed_ipv4 = _pack('>2H', *[int(i, 16) for i in tokens[-2:]])
            ipv4_str = inet_ntoa(packed_ipv4)
            tokens = tokens[0:-2] + [ipv4_str]

        return ':'.join(_compact_ipv6_tokens(tokens))
    else:
        raise ValueError('unknown address family %d' % af)


def _inet_pton_af_inet(ip_string):
    """
    Convert an IP address in string format (123.45.67.89) to the 32-bit packed
    binary format used in low-level network functions. Differs from inet_aton
    by only support decimal octets. Using octal or hexadecimal values will
    raise a ValueError exception.
    """
    #TODO: optimise this ... use inet_aton with mods if available ...
    if _is_str(ip_string):
        invalid_addr = ValueError('illegal IP address string %r' % ip_string)
        #   Support for hexadecimal and octal octets.
        tokens = ip_string.split('.')

        #   Pack octets.
        if len(tokens) == 4:
            words = []
            for token in tokens:
                if token.startswith('0x') or (token.startswith('0') and len(token) > 1):
                    raise invalid_addr
                try:
                    octet = int(token)
                except ValueError:
                    raise invalid_addr

                if (octet >> 8) != 0:
                    raise invalid_addr
                words.append(_pack('B', octet))
            return _bytes_join(words)
        else:
            raise invalid_addr

    raise ValueError('argument should be a string, not %s' % type(ip_string))


def inet_pton(af, ip_string):
    """
    Convert an IP address from string format to a packed string suitable for
    use with low-level network functions.
    """
    if af == AF_INET:
        #   IPv4.
        return _inet_pton_af_inet(ip_string)
    elif af == AF_INET6:
        invalid_addr = ValueError('illegal IP address string %r' % ip_string)
        #   IPv6.
        values = []

        if not _is_str(ip_string):
            raise invalid_addr

        if 'x' in ip_string:
            #   Don't accept hextets with the 0x prefix.
            raise invalid_addr

        if '::' in ip_string:
            if ip_string == '::':
                #   Unspecified address.
                return '\x00'.encode() * 16
            #   IPv6 compact mode.
            try:
                prefix, suffix = ip_string.split('::')
            except ValueError:
                raise invalid_addr

            l_prefix = []
            l_suffix = []

            if prefix != '':
                l_prefix = prefix.split(':')

            if suffix != '':
                l_suffix = suffix.split(':')

            #   IPv6 compact IPv4 compatibility mode.
            if len(l_suffix) and '.' in l_suffix[-1]:
                ipv4_str = _inet_pton_af_inet(l_suffix.pop())
                l_suffix.append('%x' % _unpack('>H', ipv4_str[0:2])[0])
                l_suffix.append('%x' % _unpack('>H', ipv4_str[2:4])[0])

            token_count = len(l_prefix) + len(l_suffix)

            if not 0 <= token_count <= 8 - 1:
                raise invalid_addr

            gap_size = 8 - ( len(l_prefix) + len(l_suffix) )

            values = (
                [_pack('>H', int(i, 16)) for i in l_prefix] +
                ['\x00\x00'.encode() for i in range(gap_size)] +
                [_pack('>H', int(i, 16)) for i in l_suffix]
            )
            try:
                for token in l_prefix + l_suffix:
                    word = int(token, 16)
                    if not 0 <= word <= 0xffff:
                        raise invalid_addr
            except ValueError:
                raise invalid_addr
        else:
            #   IPv6 verbose mode.
            if ':' in ip_string:
                tokens = ip_string.split(':')

                if '.' in ip_string:
                    ipv6_prefix = tokens[:-1]
                    if ipv6_prefix[:-1] != ['0', '0', '0', '0', '0']:
                        raise invalid_addr

                    if ipv6_prefix[-1].lower() not in ('0', 'ffff'):
                        raise invalid_addr

                    #   IPv6 verbose IPv4 compatibility mode.
                    if len(tokens) != 7:
                        raise invalid_addr

                    ipv4_str = _inet_pton_af_inet(tokens.pop())
                    tokens.append('%x' % _unpack('>H', ipv4_str[0:2])[0])
                    tokens.append('%x' % _unpack('>H', ipv4_str[2:4])[0])

                    values = [_pack('>H', int(i, 16)) for i in tokens]
                else:
                    #   IPv6 verbose mode.
                    if len(tokens) != 8:
                        raise invalid_addr
                try:
                    tokens = [int(token, 16) for token in tokens]
                    for token in tokens:
                        if not 0 <= token <= 0xffff:
                            raise invalid_addr

                except ValueError:
                    raise invalid_addr

                values = [_pack('>H', i) for i in tokens]
            else:
                raise invalid_addr

        return _bytes_join(values)
    else:
        raise ValueError('Unknown address family %d' % af)
