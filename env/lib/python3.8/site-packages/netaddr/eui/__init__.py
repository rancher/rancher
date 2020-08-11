#-----------------------------------------------------------------------------
#   Copyright (c) 2008 by David P. D. Moss. All rights reserved.
#
#   Released under the BSD license. See the LICENSE file for details.
#-----------------------------------------------------------------------------
"""
Classes and functions for dealing with MAC addresses, EUI-48, EUI-64, OUI, IAB
identifiers.
"""

from netaddr.core import NotRegisteredError, AddrFormatError, DictDotLookup
from netaddr.strategy import eui48 as _eui48, eui64 as _eui64
from netaddr.strategy.eui48 import mac_eui48
from netaddr.strategy.eui64 import eui64_base
from netaddr.ip import IPAddress
from netaddr.compat import _is_int, _is_str


class BaseIdentifier(object):
    """Base class for all IEEE identifiers."""
    __slots__ = ('_value',)

    def __init__(self):
        self._value = None

    def __int__(self):
        """:return: integer value of this identifier"""
        return self._value

    def __long__(self):
        """:return: integer value of this identifier"""
        return self._value

    def __oct__(self):
        """:return: octal string representation of this identifier."""
        #   Python 2.x only.
        if self._value == 0:
            return '0'
        return '0%o' % self._value

    def __hex__(self):
        """:return: hexadecimal string representation of this identifier."""
        #   Python 2.x only.
        return '0x%x' % self._value

    def __index__(self):
        """
        :return: return the integer value of this identifier when passed to
            hex(), oct() or bin().
        """
        #   Python 3.x only.
        return self._value


class OUI(BaseIdentifier):
    """
    An individual IEEE OUI (Organisationally Unique Identifier).

    For online details see - http://standards.ieee.org/regauth/oui/

    """
    __slots__ = ('records',)

    def __init__(self, oui):
        """
        Constructor

        :param oui: an OUI string ``XX-XX-XX`` or an unsigned integer. \
            Also accepts and parses full MAC/EUI-48 address strings (but not \
            MAC/EUI-48 integers)!
        """
        super(OUI, self).__init__()

        #   Lazy loading of IEEE data structures.
        from netaddr.eui import ieee

        self.records = []

        if isinstance(oui, str):
            #TODO: Improve string parsing here.
            #TODO: Accept full MAC/EUI-48 addressses as well as XX-XX-XX
            #TODO: and just take /16 (see IAB for details)
            self._value = int(oui.replace('-', ''), 16)
        elif _is_int(oui):
            if 0 <= oui <= 0xffffff:
                self._value = oui
            else:
                raise ValueError('OUI int outside expected range: %r' % oui)
        else:
            raise TypeError('unexpected OUI format: %r' % oui)

        #   Discover offsets.
        if self._value in ieee.OUI_INDEX:
            fh = open(ieee.OUI_REGISTRY_PATH, 'rb')
            for (offset, size) in ieee.OUI_INDEX[self._value]:
                fh.seek(offset)
                data = fh.read(size).decode('UTF-8')
                self._parse_data(data, offset, size)
            fh.close()
        else:
            raise NotRegisteredError('OUI %r not registered!' % oui)

    def __eq__(self, other):
        if not isinstance(other, OUI):
            try:
                other = self.__class__(other)
            except Exception:
                return NotImplemented
        return self._value == other._value

    def __ne__(self, other):
        if not isinstance(other, OUI):
            try:
                other = self.__class__(other)
            except Exception:
                return NotImplemented
        return self._value != other._value

    def __getstate__(self):
        """:returns: Pickled state of an `OUI` object."""
        return self._value, self.records

    def __setstate__(self, state):
        """:param state: data used to unpickle a pickled `OUI` object."""
        self._value, self.records = state

    def _parse_data(self, data, offset, size):
        """Returns a dict record from raw OUI record data"""
        record = {
            'idx': 0,
            'oui': '',
            'org': '',
            'address': [],
            'offset': offset,
            'size': size,
        }

        for line in data.split("\n"):
            line = line.strip()
            if not line:
                continue

            if '(hex)' in line:
                record['idx'] = self._value
                record['org'] = line.split(None, 2)[2]
                record['oui'] = str(self)
            elif '(base 16)' in line:
                continue
            else:
                record['address'].append(line)

        self.records.append(record)

    @property
    def reg_count(self):
        """Number of registered organisations with this OUI"""
        return len(self.records)

    def registration(self, index=0):
        """
        The IEEE registration details for this OUI.

        :param index: the index of record (may contain multiple registrations)
            (Default: 0 - first registration)

        :return: Objectified Python data structure containing registration
            details.
        """
        return DictDotLookup(self.records[index])

    def __str__(self):
        """:return: string representation of this OUI"""
        int_val = self._value
        return "%02X-%02X-%02X" % (
                (int_val >> 16) & 0xff,
                (int_val >> 8) & 0xff,
                int_val & 0xff)

    def __repr__(self):
        """:return: executable Python string to recreate equivalent object."""
        return "OUI('%s')" % self


class IAB(BaseIdentifier):
    """
    An individual IEEE IAB (Individual Address Block) identifier.

    For online details see - http://standards.ieee.org/regauth/oui/

    """
    __slots__ = ('record',)

    @staticmethod
    def split_iab_mac(eui_int, strict=False):
        """
        :param eui_int: a MAC IAB as an unsigned integer.

        :param strict: If True, raises a ValueError if the last 12 bits of
            IAB MAC/EUI-48 address are non-zero, ignores them otherwise.
            (Default: False)
        """
        if 0x50c2000 <= eui_int <= 0x50c2fff:
            return eui_int, 0

        user_mask = 2 ** 12 - 1
        iab_mask = (2 ** 48 - 1) ^ user_mask
        iab_bits = eui_int >> 12
        user_bits = (eui_int | iab_mask) - iab_mask

        if 0x50c2000 <= iab_bits <= 0x50c2fff:
            if strict and user_bits != 0:
                raise ValueError('%r is not a strict IAB!' % hex(user_bits))
        else:
            raise ValueError('%r is not an IAB address!' % hex(eui_int))

        return iab_bits, user_bits

    def __init__(self, iab, strict=False):
        """
        Constructor

        :param iab: an IAB string ``00-50-C2-XX-X0-00`` or an unsigned \
            integer. This address looks like an EUI-48 but it should not \
            have any non-zero bits in the last 3 bytes.

        :param strict: If True, raises a ValueError if the last 12 bits \
            of IAB MAC/EUI-48 address are non-zero, ignores them otherwise. \
            (Default: False)
        """
        super(IAB, self).__init__()

        #   Lazy loading of IEEE data structures.
        from netaddr.eui import ieee

        self.record = {
            'idx': 0,
            'iab': '',
            'org': '',
            'address': [],
            'offset': 0,
            'size': 0,
        }

        if isinstance(iab, str):
            #TODO: Improve string parsing here.
            #TODO: '00-50-C2' is actually invalid.
            #TODO: Should be '00-50-C2-00-00-00' (i.e. a full MAC/EUI-48)
            int_val = int(iab.replace('-', ''), 16)
            iab_int, user_int = self.split_iab_mac(int_val, strict=strict)
            self._value = iab_int
        elif _is_int(iab):
            iab_int, user_int = self.split_iab_mac(iab, strict=strict)
            self._value = iab_int
        else:
            raise TypeError('unexpected IAB format: %r!' % iab)

        #   Discover offsets.
        if self._value in ieee.IAB_INDEX:
            fh = open(ieee.IAB_REGISTRY_PATH, 'rb')
            (offset, size) = ieee.IAB_INDEX[self._value][0]
            self.record['offset'] = offset
            self.record['size'] = size
            fh.seek(offset)
            data = fh.read(size).decode('UTF-8')
            self._parse_data(data, offset, size)
            fh.close()
        else:
            raise NotRegisteredError('IAB %r not unregistered!' % iab)

    def __eq__(self, other):
        if not isinstance(other, IAB):
            try:
                other = self.__class__(other)
            except Exception:
                return NotImplemented
        return self._value == other._value

    def __ne__(self, other):
        if not isinstance(other, IAB):
            try:
                other = self.__class__(other)
            except Exception:
                return NotImplemented
        return self._value != other._value

    def __getstate__(self):
        """:returns: Pickled state of an `IAB` object."""
        return self._value, self.record

    def __setstate__(self, state):
        """:param state: data used to unpickle a pickled `IAB` object."""
        self._value, self.record = state

    def _parse_data(self, data, offset, size):
        """Returns a dict record from raw IAB record data"""
        for line in data.split("\n"):
            line = line.strip()
            if not line:
                continue

            if '(hex)' in line:
                self.record['idx'] = self._value
                self.record['org'] = line.split(None, 2)[2]
                self.record['iab'] = str(self)
            elif '(base 16)' in line:
                continue
            else:
                self.record['address'].append(line)

    def registration(self):
        """The IEEE registration details for this IAB"""
        return DictDotLookup(self.record)

    def __str__(self):
        """:return: string representation of this IAB"""
        int_val = self._value << 4

        return "%02X-%02X-%02X-%02X-%02X-00" % (
                (int_val >> 32) & 0xff,
                (int_val >> 24) & 0xff,
                (int_val >> 16) & 0xff,
                (int_val >> 8) & 0xff,
                int_val & 0xff)

    def __repr__(self):
        """:return: executable Python string to recreate equivalent object."""
        return "IAB('%s')" % self


class EUI(BaseIdentifier):
    """
    An IEEE EUI (Extended Unique Identifier).

    Both EUI-48 (used for layer 2 MAC addresses) and EUI-64 are supported.

    Input parsing for EUI-48 addresses is flexible, supporting many MAC
    variants.

    """
    __slots__ = ('_module', '_dialect')

    def __init__(self, addr, version=None, dialect=None):
        """
        Constructor.

        :param addr: an EUI-48 (MAC) or EUI-64 address in string format or \
            an unsigned integer. May also be another EUI object (copy \
            construction).

        :param version: (optional) the explicit EUI address version, either \
            48 or 64. Mainly used to distinguish EUI-48 and EUI-64 identifiers \
            specified as integers which may be numerically equivalent.

        :param dialect: (optional) the mac_* dialect to be used to configure \
            the formatting of EUI-48 (MAC) addresses.
        """
        super(EUI, self).__init__()

        self._module = None

        if isinstance(addr, EUI):
            #   Copy constructor.
            if version is not None and version != addr._module.version:
                raise ValueError('cannot switch EUI versions using '
                    'copy constructor!')
            self._module = addr._module
            self._value = addr._value
            self.dialect = addr.dialect
            return

        if version is not None:
            if version == 48:
                self._module = _eui48
            elif version == 64:
                self._module = _eui64
            else:
                raise ValueError('unsupported EUI version %r' % version)
        else:
        #   Choose a default version when addr is an integer and version is
        #   not specified.
            if _is_int(addr):
                if 0 <= addr <= 0xffffffffffff:
                    self._module = _eui48
                elif 0xffffffffffff < addr <= 0xffffffffffffffff:
                    self._module = _eui64

        self.value = addr

        #   Choose a dialect for MAC formatting.
        self.dialect = dialect

    def __getstate__(self):
        """:returns: Pickled state of an `EUI` object."""
        return self._value, self._module.version, self.dialect

    def __setstate__(self, state):
        """
        :param state: data used to unpickle a pickled `EUI` object.

        """
        value, version, dialect = state

        self._value = value

        if version == 48:
            self._module = _eui48
        elif version == 64:
            self._module = _eui64
        else:
            raise ValueError('unpickling failed for object state: %s' \
                % str(state))

        self.dialect = dialect

    def _get_value(self):
        return self._value

    def _set_value(self, value):
        if self._module is None:
            #   EUI version is implicit, detect it from value.
            for module in (_eui48, _eui64):
                try:
                    self._value = module.str_to_int(value)
                    self._module = module
                    break
                except AddrFormatError:
                    try:
                        if 0 <= int(value) <= module.max_int:
                            self._value = int(value)
                            self._module = module
                            break
                    except ValueError:
                        pass

            if self._module is None:
                raise AddrFormatError('failed to detect EUI version: %r'
                    % value)
        else:
            #   EUI version is explicit.
            if _is_str(value):
                try:
                    self._value = self._module.str_to_int(value)
                except AddrFormatError:
                    raise AddrFormatError('address %r is not an EUIv%d'
                        % (value, self._module.version))
            else:
                if 0 <= int(value) <= self._module.max_int:
                    self._value = int(value)
                else:
                    raise AddrFormatError('bad address format: %r' % value)

    value = property(_get_value, _set_value, None,
        'a positive integer representing the value of this EUI indentifier.')

    def _get_dialect(self):
        return self._dialect

    def _set_dialect(self, value):
        if value is None:
            if self._module is _eui64:
                self._dialect = eui64_base
            else:
                self._dialect = mac_eui48
        else:
            if hasattr(value, 'word_size') and hasattr(value, 'word_fmt'):
                self._dialect = value
            else:
                raise TypeError('custom dialects should subclass mac_eui48!')

    dialect = property(_get_dialect, _set_dialect, None,
        "a Python class providing support for the interpretation of "
        "various MAC\n address formats.")

    @property
    def oui(self):
        """The OUI (Organisationally Unique Identifier) for this EUI."""
        if self._module == _eui48:
            return OUI(self.value >> 24)
        elif self._module == _eui64:
            return OUI(self.value >> 40)

    @property
    def ei(self):
        """The EI (Extension Identifier) for this EUI"""
        if self._module == _eui48:
            return '%02X-%02X-%02X' % tuple(self[3:6])
        elif self._module == _eui64:
            return '%02X-%02X-%02X-%02X-%02X' % tuple(self[3:8])

    def is_iab(self):
        """:return: True if this EUI is an IAB address, False otherwise"""
        return 0x50c2000 <= (self._value >> 12) <= 0x50c2fff

    @property
    def iab(self):
        """
        If is_iab() is True, the IAB (Individual Address Block) is returned,
        ``None`` otherwise.
        """
        if self.is_iab():
            return IAB(self._value >> 12)

    @property
    def version(self):
        """The EUI version represented by this EUI object."""
        return self._module.version

    def __getitem__(self, idx):
        """
        :return: The integer value of the word referenced by index (both \
            positive and negative). Raises ``IndexError`` if index is out \
            of bounds. Also supports Python list slices for accessing \
            word groups.
        """
        if _is_int(idx):
            #   Indexing, including negative indexing goodness.
            num_words = self._dialect.num_words
            if not (-num_words) <= idx <= (num_words - 1):
                raise IndexError('index out range for address type!')
            return self._module.int_to_words(self._value, self._dialect)[idx]
        elif isinstance(idx, slice):
            words = self._module.int_to_words(self._value, self._dialect)
            return [words[i] for i in range(*idx.indices(len(words)))]
        else:
            raise TypeError('unsupported type %r!' % idx)

    def __setitem__(self, idx, value):
        """Set the value of the word referenced by index in this address"""
        if isinstance(idx, slice):
            #   TODO - settable slices.
            raise NotImplementedError('settable slices are not supported!')

        if not _is_int(idx):
            raise TypeError('index not an integer!')

        if not 0 <= idx <= (self._dialect.num_words - 1):
            raise IndexError('index %d outside address type boundary!' % idx)

        if not _is_int(value):
            raise TypeError('value not an integer!')

        if not 0 <= value <= self._dialect.max_word:
            raise IndexError('value %d outside word size maximum of %d bits!'
                % (value, self._dialect.word_size))

        words = list(self._module.int_to_words(self._value, self._dialect))
        words[idx] = value
        self._value = self._module.words_to_int(words)

    def __hash__(self):
        """:return: hash of this EUI object suitable for dict keys, sets etc"""
        return hash((self.version, self._value))

    def __eq__(self, other):
        """
        :return: ``True`` if this EUI object is numerically the same as other, \
            ``False`` otherwise.
        """
        if not isinstance(other, EUI):
            try:
                other = self.__class__(other)
            except Exception:
                return NotImplemented
        return (self.version, self._value) == (other.version, other._value)

    def __ne__(self, other):
        """
        :return: ``True`` if this EUI object is numerically the same as other, \
            ``False`` otherwise.
        """
        if not isinstance(other, EUI):
            try:
                other = self.__class__(other)
            except Exception:
                return NotImplemented
        return (self.version, self._value) != (other.version, other._value)

    def __lt__(self, other):
        """
        :return: ``True`` if this EUI object is numerically lower in value than \
            other, ``False`` otherwise.
        """
        if not isinstance(other, EUI):
            try:
                other = self.__class__(other)
            except Exception:
                return NotImplemented
        return (self.version, self._value) < (other.version, other._value)

    def __le__(self, other):
        """
        :return: ``True`` if this EUI object is numerically lower or equal in \
            value to other, ``False`` otherwise.
        """
        if not isinstance(other, EUI):
            try:
                other = self.__class__(other)
            except Exception:
                return NotImplemented
        return (self.version, self._value) <= (other.version, other._value)

    def __gt__(self, other):
        """
        :return: ``True`` if this EUI object is numerically greater in value \
            than other, ``False`` otherwise.
        """
        if not isinstance(other, EUI):
            try:
                other = self.__class__(other)
            except Exception:
                return NotImplemented
        return (self.version, self._value) > (other.version, other._value)

    def __ge__(self, other):
        """
        :return: ``True`` if this EUI object is numerically greater or equal \
            in value to other, ``False`` otherwise.
        """
        if not isinstance(other, EUI):
            try:
                other = self.__class__(other)
            except Exception:
                return NotImplemented
        return (self.version, self._value) >= (other.version, other._value)

    def bits(self, word_sep=None):
        """
        :param word_sep: (optional) the separator to insert between words. \
            Default: None - use default separator for address type.

        :return: human-readable binary digit string of this address.
        """
        return self._module.int_to_bits(self._value, word_sep)

    @property
    def packed(self):
        """The value of this EUI address as a packed binary string."""
        return self._module.int_to_packed(self._value)

    @property
    def words(self):
        """A list of unsigned integer octets found in this EUI address."""
        return self._module.int_to_words(self._value)

    @property
    def bin(self):
        """
        The value of this EUI adddress in standard Python binary
        representational form (0bxxx). A back port of the format provided by
        the builtin bin() function found in Python 2.6.x and higher.
        """
        return self._module.int_to_bin(self._value)

    def eui64(self):
        """
        - If this object represents an EUI-48 it is converted to EUI-64 \
            as per the standard.
        - If this object is already an EUI-64, a new, numerically \
            equivalent object is returned instead.

        :return: The value of this EUI object as a new 64-bit EUI object.
        """
        if self.version == 48:
            # Convert 11:22:33:44:55:66 into 11:22:33:FF:FE:44:55:66.
            first_three = self._value >> 24
            last_three = self._value & 0xffffff
            new_value = (first_three << 40) | 0xfffe000000 | last_three
        else:
            # is already a EUI64
            new_value = self._value
        return self.__class__(new_value, version=64)

    def modified_eui64(self):
        """
        - create a new EUI object with a modified EUI-64 as described in RFC 4291 section 2.5.1

        :return: a new and modified 64-bit EUI object.
        """
        # Modified EUI-64 format interface identifiers are formed by inverting
        # the "u" bit (universal/local bit in IEEE EUI-64 terminology) when
        # forming the interface identifier from IEEE EUI-64 identifiers.  In
        # the resulting Modified EUI-64 format, the "u" bit is set to one (1)
        # to indicate universal scope, and it is set to zero (0) to indicate
        # local scope.
        eui64 = self.eui64()
        eui64._value ^= 0x00000000000000000200000000000000
        return eui64

    def ipv6(self, prefix):
        """
        .. note:: This poses security risks in certain scenarios. \
            Please read RFC 4941 for details. Reference: RFCs 4291 and 4941.

        :param prefix: ipv6 prefix

        :return: new IPv6 `IPAddress` object based on this `EUI` \
            using the technique described in RFC 4291.
        """
        int_val = int(prefix) + int(self.modified_eui64())
        return IPAddress(int_val, version=6)

    def ipv6_link_local(self):
        """
        .. note:: This poses security risks in certain scenarios. \
            Please read RFC 4941 for details. Reference: RFCs 4291 and 4941.

        :return: new link local IPv6 `IPAddress` object based on this `EUI` \
            using the technique described in RFC 4291.
        """
        return self.ipv6(0xfe800000000000000000000000000000)

    @property
    def info(self):
        """
        A record dict containing IEEE registration details for this EUI
        (MAC-48) if available, None otherwise.
        """
        data = {'OUI': self.oui.registration()}
        if self.is_iab():
            data['IAB'] = self.iab.registration()

        return DictDotLookup(data)

    def __str__(self):
        """:return: EUI in representational format"""
        return self._module.int_to_str(self._value, self._dialect)

    def __repr__(self):
        """:return: executable Python string to recreate equivalent object."""
        return "EUI('%s')" % self

