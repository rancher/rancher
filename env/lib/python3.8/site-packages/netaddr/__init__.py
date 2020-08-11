#-----------------------------------------------------------------------------
#   Copyright (c) 2008 by David P. D. Moss. All rights reserved.
#
#   Released under the BSD license. See the LICENSE file for details.
#-----------------------------------------------------------------------------
"""A Python library for manipulating IP and EUI network addresses."""

#: Version info (major, minor, maintenance, status)
VERSION = (0, 7, 19)
STATUS = ''
__version__ = '%d.%d.%d' % VERSION[0:3] + STATUS

import sys as _sys

if _sys.version_info[0:2] < (2, 4):
    raise RuntimeError('Python 2.4.x or higher is required!')

from netaddr.core import (AddrConversionError, AddrFormatError,
    NotRegisteredError, ZEROFILL, Z, INET_PTON, P, NOHOST, N)

from netaddr.ip import (IPAddress, IPNetwork, IPRange, all_matching_cidrs,
    cidr_abbrev_to_verbose, cidr_exclude, cidr_merge, iprange_to_cidrs,
    iter_iprange, iter_unique_ips, largest_matching_cidr,
    smallest_matching_cidr, spanning_cidr)

from netaddr.ip.sets import IPSet

from netaddr.ip.glob import (IPGlob, cidr_to_glob, glob_to_cidrs,
    glob_to_iprange, glob_to_iptuple, iprange_to_globs, valid_glob)

from netaddr.ip.nmap import valid_nmap_range, iter_nmap_range

from netaddr.ip.rfc1924 import base85_to_ipv6, ipv6_to_base85

from netaddr.eui import EUI, IAB, OUI

from netaddr.strategy.ipv4 import valid_str as valid_ipv4

from netaddr.strategy.ipv6 import (valid_str as valid_ipv6, ipv6_compact,
    ipv6_full, ipv6_verbose)

from netaddr.strategy.eui48 import (mac_eui48, mac_unix, mac_unix_expanded,
        mac_cisco, mac_bare, mac_pgsql, valid_str as valid_mac)

from netaddr.strategy.eui64 import (eui64_base, eui64_unix, eui64_unix_expanded,
        eui64_cisco, eui64_bare, valid_str as valid_eui64)

from netaddr.contrib.subnet_splitter import SubnetSplitter
