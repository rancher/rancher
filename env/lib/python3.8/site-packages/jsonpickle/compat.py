from __future__ import absolute_import, division, unicode_literals
import sys
import types
import base64

PY_MAJOR = sys.version_info[0]
PY2 = PY_MAJOR == 2
PY3 = PY_MAJOR == 3
PY3_ORDERED_DICT = PY3 and sys.version_info[1] >= 6  # Python 3.6+

class_types = (type,)
iterator_types = (type(iter('')),)

if PY3:
    import builtins
    import queue
    from base64 import encodebytes, decodebytes
    from collections.abc import Iterator as abc_iterator

    string_types = (str,)
    numeric_types = (int, float)
    ustr = str
else:
    from collections import Iterator as abc_iterator  # noqa

    builtins = __import__('__builtin__')
    class_types += (types.ClassType,)
    encodebytes = base64.encodestring
    decodebytes = base64.decodestring
    string_types = (builtins.basestring,)
    numeric_types = (int, float, builtins.long)
    queue = __import__('Queue')
    ustr = builtins.unicode


def iterator(class_):
    if PY2 and hasattr(class_, '__next__'):
        class_.next = class_.__next__
    return class_
