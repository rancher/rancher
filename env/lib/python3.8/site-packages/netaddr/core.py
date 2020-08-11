#-----------------------------------------------------------------------------
#   Copyright (c) 2008 by David P. D. Moss. All rights reserved.
#
#   Released under the BSD license. See the LICENSE file for details.
#-----------------------------------------------------------------------------
"""Common code shared between various netaddr sub modules"""

import sys as _sys
import pprint as _pprint

from netaddr.compat import _callable, _iter_dict_keys

#: True if platform is natively big endian, False otherwise.
BIG_ENDIAN_PLATFORM = _sys.byteorder == 'big'

#:  Use inet_pton() semantics instead of inet_aton() when parsing IPv4.
P = INET_PTON = 1

#:  Remove any preceding zeros from IPv4 address octets before parsing.
Z = ZEROFILL = 2

#:  Remove any host bits found to the right of an applied CIDR prefix.
N = NOHOST = 4

#-----------------------------------------------------------------------------
#   Custom exceptions.
#-----------------------------------------------------------------------------
class AddrFormatError(Exception):
    """
    An Exception indicating a network address is not correctly formatted.
    """
    pass


class AddrConversionError(Exception):
    """
    An Exception indicating a failure to convert between address types or
    notations.
    """
    pass


class NotRegisteredError(Exception):
    """
    An Exception indicating that an OUI or IAB was not found in the IEEE
    Registry.
    """
    pass


try:
    a = 42
    a.bit_length()
    # No exception, must be Python 2.7 or 3.1+ -> can use bit_length()
    def num_bits(int_val):
        """
        :param int_val: an unsigned integer.

        :return: the minimum number of bits needed to represent value provided.
        """
        return int_val.bit_length()
except AttributeError:
    # a.bit_length() excepted, must be an older Python version.
    def num_bits(int_val):
        """
        :param int_val: an unsigned integer.

        :return: the minimum number of bits needed to represent value provided.
        """
        numbits = 0
        while int_val:
            numbits += 1
            int_val >>= 1
        return numbits


class Subscriber(object):
    """
    An abstract class defining the interface expected by a Publisher.
    """

    def update(self, data):
        """
        A callback method used by a Publisher to notify this Subscriber about
        updates.

        :param data: a Python object containing data provided by Publisher.
        """
        raise NotImplementedError('cannot invoke virtual method!')


class PrettyPrinter(Subscriber):
    """
    A concrete Subscriber that employs the pprint in the standard library to
    format all data from updates received, writing them to a file-like
    object.

    Useful as a debugging aid.
    """

    def __init__(self, fh=_sys.stdout, write_eol=True):
        """
        Constructor.

        :param fh: a file-like object to write updates to.
            Default: sys.stdout.


        :param write_eol: if ``True`` this object will write newlines to
            output, if ``False`` it will not.
        """
        self.fh = fh
        self.write_eol = write_eol

    def update(self, data):
        """
        A callback method used by a Publisher to notify this Subscriber about
        updates.

        :param data: a Python object containing data provided by Publisher.
        """
        self.fh.write(_pprint.pformat(data))
        if self.write_eol:
            self.fh.write("\n")


class Publisher(object):
    """
    A 'push' Publisher that maintains a list of Subscriber objects notifying
    them of state changes by passing them update data when it encounter events
    of interest.
    """

    def __init__(self):
        """Constructor"""
        self.subscribers = []

    def attach(self, subscriber):
        """
        Add a new subscriber.

        :param subscriber: a new object that implements the Subscriber object
            interface.
        """
        if hasattr(subscriber, 'update') and _callable(eval('subscriber.update')):
            if subscriber not in self.subscribers:
                self.subscribers.append(subscriber)
        else:
            raise TypeError('%r does not support required interface!' % subscriber)

    def detach(self, subscriber):
        """
        Remove an existing subscriber.

        :param subscriber: a new object that implements the Subscriber object
            interface.
        """
        try:
            self.subscribers.remove(subscriber)
        except ValueError:
            pass

    def notify(self, data):
        """
        Send update data to to all registered Subscribers.

        :param data: the data to be passed to each registered Subscriber.
        """
        for subscriber in self.subscribers:
            subscriber.update(data)


class DictDotLookup(object):
    """
    Creates objects that behave much like a dictionaries, but allow nested
    key access using object '.' (dot) lookups.

    Recipe 576586: Dot-style nested lookups over dictionary based data
    structures - http://code.activestate.com/recipes/576586/

    """

    def __init__(self, d):
        for k in d:
            if isinstance(d[k], dict):
                self.__dict__[k] = DictDotLookup(d[k])
            elif isinstance(d[k], (list, tuple)):
                l = []
                for v in d[k]:
                    if isinstance(v, dict):
                        l.append(DictDotLookup(v))
                    else:
                        l.append(v)
                self.__dict__[k] = l
            else:
                self.__dict__[k] = d[k]

    def __getitem__(self, name):
        if name in self.__dict__:
            return self.__dict__[name]

    def __iter__(self):
        return _iter_dict_keys(self.__dict__)

    def __repr__(self):
        return _pprint.pformat(self.__dict__)
