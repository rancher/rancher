#!/usr/bin/env python
#-----------------------------------------------------------------------------
#   Copyright (c) 2008 by David P. D. Moss. All rights reserved.
#
#   Released under the BSD license. See the LICENSE file for details.
#-----------------------------------------------------------------------------
#
#   DISCLAIMER
#
#   netaddr is not sponsored nor endorsed by IANA.
#
#   Use of data from IANA (Internet Assigned Numbers Authority) is subject to
#   copyright and is provided with prior written permission.
#
#   IANA data files included with netaddr are not modified in any way but are
#   parsed and made available to end users through an API.
#
#   See README file and source code for URLs to latest copies of the relevant
#   files.
#
#-----------------------------------------------------------------------------
"""
Routines for accessing data published by IANA (Internet Assigned Numbers
Authority).

More details can be found at the following URLs :-

    - IANA Home Page - http://www.iana.org/
    - IEEE Protocols Information Home Page - http://www.iana.org/protocols/
"""

import os.path as _path
import sys as _sys
from xml.sax import make_parser, handler

from netaddr.core import Publisher, Subscriber
from netaddr.ip import IPAddress, IPNetwork, IPRange, cidr_abbrev_to_verbose
from netaddr.compat import _dict_items, _callable



#: Topic based lookup dictionary for IANA information.
IANA_INFO = {
    'IPv4': {},
    'IPv6': {},
    'IPv6_unicast': {},
    'multicast': {},
}


class SaxRecordParser(handler.ContentHandler):
    def __init__(self, callback=None):
        self._level = 0
        self._is_active = False
        self._record = None
        self._tag_level = None
        self._tag_payload = None
        self._tag_feeding = None
        self._callback = callback

    def startElement(self, name, attrs):
        self._level += 1

        if self._is_active is False:
            if name == 'record':
                self._is_active = True
                self._tag_level = self._level
                self._record = {}
                if 'date' in attrs:
                    self._record['date'] = attrs['date']
        elif self._level == self._tag_level + 1:
            if name == 'xref':
                if 'type' in attrs and 'data' in attrs:
                    l = self._record.setdefault(attrs['type'], [])
                    l.append(attrs['data'])
            else:
                self._tag_payload = []
                self._tag_feeding = True
        else:
            self._tag_feeding = False

    def endElement(self, name):
        if self._is_active is True:
            if name == 'record' and self._tag_level == self._level:
                self._is_active = False
                self._tag_level = None
                if _callable(self._callback):
                    self._callback(self._record)
                self._record = None
            elif self._level == self._tag_level + 1:
                if name != 'xref':
                    self._record[name] = ''.join(self._tag_payload)
                    self._tag_payload = None
                    self._tag_feeding = False

        self._level -= 1

    def characters(self, content):
        if self._tag_feeding is True:
            self._tag_payload.append(content)


class XMLRecordParser(Publisher):
    """
    A configurable Parser that understands how to parse XML based records.
    """

    def __init__(self, fh, **kwargs):
        """
        Constructor.

        fh - a valid, open file handle to XML based record data.
        """
        super(XMLRecordParser, self).__init__()

        self.xmlparser = make_parser()
        self.xmlparser.setContentHandler(SaxRecordParser(self.consume_record))

        self.fh = fh

        self.__dict__.update(kwargs)

    def process_record(self, rec):
        """
        This is the callback method invoked for every record. It is usually
        over-ridden by base classes to provide specific record-based logic.

        Any record can be vetoed (not passed to registered Subscriber objects)
        by simply returning None.
        """
        return rec

    def consume_record(self, rec):
        record = self.process_record(rec)
        if record is not None:
            self.notify(record)

    def parse(self):
        """
        Parse and normalises records, notifying registered subscribers with
        record data as it is encountered.
        """
        self.xmlparser.parse(self.fh)


class IPv4Parser(XMLRecordParser):
    """
    A XMLRecordParser that understands how to parse and retrieve data records
    from the IANA IPv4 address space file.

    It can be found online here :-

        - http://www.iana.org/assignments/ipv4-address-space/ipv4-address-space.xml
    """

    def __init__(self, fh, **kwargs):
        """
        Constructor.

        fh - a valid, open file handle to an IANA IPv4 address space file.

        kwargs - additional parser options.
        """
        super(IPv4Parser, self).__init__(fh)

    def process_record(self, rec):
        """
        Callback method invoked for every record.

        See base class method for more details.
        """

        record = {}
        for key in ('prefix', 'designation', 'date', 'whois', 'status'):
            record[key] = str(rec.get(key, '')).strip()

        #   Strip leading zeros from octet.
        if '/' in record['prefix']:
            (octet, prefix) = record['prefix'].split('/')
            record['prefix'] = '%d/%d' % (int(octet), int(prefix))

        record['status'] = record['status'].capitalize()

        return record


class IPv6Parser(XMLRecordParser):
    """
    A XMLRecordParser that understands how to parse and retrieve data records
    from the IANA IPv6 address space file.

    It can be found online here :-

        - http://www.iana.org/assignments/ipv6-address-space/ipv6-address-space.xml
    """

    def __init__(self, fh, **kwargs):
        """
        Constructor.

        fh - a valid, open file handle to an IANA IPv6 address space file.

        kwargs - additional parser options.
        """
        super(IPv6Parser, self).__init__(fh)

    def process_record(self, rec):
        """
        Callback method invoked for every record.

        See base class method for more details.
        """

        record = {
            'prefix': str(rec.get('prefix', '')).strip(),
            'allocation': str(rec.get('description', '')).strip(),
            'reference': str(rec.get('rfc', [''])[0]).strip(),
        }

        return record


class IPv6UnicastParser(XMLRecordParser):
    """
    A XMLRecordParser that understands how to parse and retrieve data records
    from the IANA IPv6 unicast address assignments file.

    It can be found online here :-

        - http://www.iana.org/assignments/ipv6-unicast-address-assignments/ipv6-unicast-address-assignments.xml
    """
    def __init__(self, fh, **kwargs):
        """
        Constructor.

        fh - a valid, open file handle to an IANA IPv6 address space file.

        kwargs - additional parser options.
        """
        super(IPv6UnicastParser, self).__init__(fh)

    def process_record(self, rec):
        """
        Callback method invoked for every record.

        See base class method for more details.
        """
        record = {
            'status': str(rec.get('status', '')).strip(),
            'description': str(rec.get('description', '')).strip(),
            'prefix': str(rec.get('prefix', '')).strip(),
            'date': str(rec.get('date', '')).strip(),
            'whois': str(rec.get('whois', '')).strip(),
        }

        return record


class MulticastParser(XMLRecordParser):
    """
    A XMLRecordParser that knows how to process the IANA IPv4 multicast address
    allocation file.

    It can be found online here :-

        - http://www.iana.org/assignments/multicast-addresses/multicast-addresses.xml
    """

    def __init__(self, fh, **kwargs):
        """
        Constructor.

        fh - a valid, open file handle to an IANA IPv4 multicast address
             allocation file.

        kwargs - additional parser options.
        """
        super(MulticastParser, self).__init__(fh)

    def normalise_addr(self, addr):
        """
        Removes variations from address entries found in this particular file.
        """
        if '-' in addr:
            (a1, a2) = addr.split('-')
            o1 = a1.strip().split('.')
            o2 = a2.strip().split('.')
            return '%s-%s' % ('.'.join([str(int(i)) for i in o1]),
                              '.'.join([str(int(i)) for i in o2]))
        else:
            o1 = addr.strip().split('.')
            return '.'.join([str(int(i)) for i in o1])

    def process_record(self, rec):
        """
        Callback method invoked for every record.

        See base class method for more details.
        """

        if 'addr' in rec:
            record = {
                'address': self.normalise_addr(str(rec['addr'])),
                'descr': str(rec.get('description', '')),
            }
            return record


class DictUpdater(Subscriber):
    """
    Concrete Subscriber that inserts records received from a Publisher into a
    dictionary.
    """

    def __init__(self, dct, topic, unique_key):
        """
        Constructor.

        dct - lookup dict or dict like object to insert records into.

        topic - high-level category name of data to be processed.

        unique_key - key name in data dict that uniquely identifies it.
        """
        self.dct = dct
        self.topic = topic
        self.unique_key = unique_key

    def update(self, data):
        """
        Callback function used by Publisher to notify this Subscriber about
        an update. Stores topic based information into dictionary passed to
        constructor.
        """
        data_id = data[self.unique_key]

        if self.topic == 'IPv4':
            cidr = IPNetwork(cidr_abbrev_to_verbose(data_id))
            self.dct[cidr] = data
        elif self.topic == 'IPv6':
            cidr = IPNetwork(cidr_abbrev_to_verbose(data_id))
            self.dct[cidr] = data
        elif self.topic == 'IPv6_unicast':
            cidr = IPNetwork(data_id)
            self.dct[cidr] = data
        elif self.topic == 'multicast':
            iprange = None
            if '-' in data_id:
                #   See if we can manage a single CIDR.
                (first, last) = data_id.split('-')
                iprange = IPRange(first, last)
                cidrs = iprange.cidrs()
                if len(cidrs) == 1:
                    iprange = cidrs[0]
            else:
                iprange = IPAddress(data_id)
            self.dct[iprange] = data


def load_info():
    """
    Parse and load internal IANA data lookups with the latest information from
    data files.
    """
    PATH = _path.dirname(__file__)

    ipv4 = IPv4Parser(open(_path.join(PATH, 'ipv4-address-space.xml')))
    ipv4.attach(DictUpdater(IANA_INFO['IPv4'], 'IPv4', 'prefix'))
    ipv4.parse()

    ipv6 = IPv6Parser(open(_path.join(PATH, 'ipv6-address-space.xml')))
    ipv6.attach(DictUpdater(IANA_INFO['IPv6'], 'IPv6', 'prefix'))
    ipv6.parse()

    ipv6ua = IPv6UnicastParser(open(_path.join(PATH, 'ipv6-unicast-address-assignments.xml')))
    ipv6ua.attach(DictUpdater(IANA_INFO['IPv6_unicast'], 'IPv6_unicast', 'prefix'))
    ipv6ua.parse()

    mcast = MulticastParser(open(_path.join(PATH, 'multicast-addresses.xml')))
    mcast.attach(DictUpdater(IANA_INFO['multicast'], 'multicast', 'address'))
    mcast.parse()


def pprint_info(fh=None):
    """
    Pretty prints IANA information to filehandle.
    """
    if fh is None:
        fh = _sys.stdout

    for category in sorted(IANA_INFO):
        fh.write('-' * len(category) + "\n")
        fh.write(category + "\n")
        fh.write('-' * len(category) + "\n")
        ipranges = IANA_INFO[category]
        for iprange in sorted(ipranges):
            details = ipranges[iprange]
            fh.write('%-45r' % (iprange) + details + "\n")


def _within_bounds(ip, ip_range):
    #   Boundary checking for multiple IP classes.
    if hasattr(ip_range, 'first'):
        #   IP network or IP range.
        return ip in ip_range
    elif hasattr(ip_range, 'value'):
        #   IP address.
        return ip == ip_range

    raise Exception('Unsupported IP range or address: %r!' % ip_range)


def query(ip_addr):
    """Returns informational data specific to this IP address."""
    info = {}

    if ip_addr.version == 4:
        for cidr, record in _dict_items(IANA_INFO['IPv4']):
            if _within_bounds(ip_addr, cidr):
                info.setdefault('IPv4', [])
                info['IPv4'].append(record)

        if ip_addr.is_multicast():
            for iprange, record in _dict_items(IANA_INFO['multicast']):
                if _within_bounds(ip_addr, iprange):
                    info.setdefault('Multicast', [])
                    info['Multicast'].append(record)

    elif ip_addr.version == 6:
        for cidr, record in _dict_items(IANA_INFO['IPv6']):
            if _within_bounds(ip_addr, cidr):
                info.setdefault('IPv6', [])
                info['IPv6'].append(record)

        for cidr, record in _dict_items(IANA_INFO['IPv6_unicast']):
            if _within_bounds(ip_addr, cidr):
                info.setdefault('IPv6_unicast', [])
                info['IPv6_unicast'].append(record)

    return info

#   On module import, read IANA data files and populate lookups dict.
load_info()
