#-----------------------------------------------------------------------------
#   Copyright (c) 2008 by David P. D. Moss. All rights reserved.
#
#   Released under the BSD license. See the LICENSE file for details.
#-----------------------------------------------------------------------------
from netaddr.ip import IPNetwork, cidr_exclude, cidr_merge


class SubnetSplitter(object):
    """
    A handy utility class that takes a single (large) subnet and allows
    smaller subnet within its range to be extracted by CIDR prefix. Any
    leaving address space is available for subsequent extractions until
    all space is exhausted.
    """
    def __init__(self, base_cidr):
        """
        Constructor.

        :param base_cidr: an IPv4 or IPv6 address with a CIDR prefix.
            (see IPNetwork.__init__ for full details).
        """
        self._subnets = set([IPNetwork(base_cidr)])

    def extract_subnet(self, prefix, count=None):
        """Extract 1 or more subnets of size specified by CIDR prefix."""
        for cidr in self.available_subnets():
            subnets = list(cidr.subnet(prefix, count=count))
            if not subnets:
                continue
            self.remove_subnet(cidr)
            self._subnets = self._subnets.union(
                set(
                    cidr_exclude(cidr, cidr_merge(subnets)[0])
                )
            )
            return subnets
        return []

    def available_subnets(self):
        """Returns a list of the currently available subnets."""
        return sorted(self._subnets, key=lambda x: x.prefixlen, reverse=True)

    def remove_subnet(self, ip_network):
        """Remove a specified IPNetwork from available address space."""
        self._subnets.remove(ip_network)
