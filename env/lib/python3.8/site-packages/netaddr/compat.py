#-----------------------------------------------------------------------------
#   Copyright (c) 2008 by David P. D. Moss. All rights reserved.
#
#   Released under the BSD license. See the LICENSE file for details.
#-----------------------------------------------------------------------------
"""
Compatibility wrappers providing uniform behaviour for Python code required to
run under both Python 2.x and 3.x.

All operations emulate 2.x behaviour where applicable.
"""
import sys as _sys

if _sys.version_info[0] == 3:
    # Python 3.x specific logic.
    _sys_maxint = _sys.maxsize

    _int_type = int

    _str_type = str

    _bytes_type = lambda x: bytes(x, 'UTF-8')

    _is_str = lambda x: isinstance(x, (str, type(''.encode())))

    _is_int = lambda x: isinstance(x, int)

    _callable = lambda x: hasattr(x, '__call__')

    _func_doc = lambda x: x.__doc__

    _dict_keys = lambda x: list(x.keys())

    _dict_items = lambda x: list(x.items())

    _iter_dict_keys = lambda x: x.keys()

    def _bytes_join(*args):
        return ''.encode().join(*args)

    def _zip(*args):
        return list(zip(*args))

    def _range(*args, **kwargs):
        return list(range(*args, **kwargs))

    _iter_range = range

    def _func_name(f, name=None):
        if name is not None:
            f.__name__ = name
        else:
            return f.__name__

    def _func_doc(f, docstring=None):
        if docstring is not None:
            f.__doc__ = docstring
        else:
            return f.__doc__

    def _iter_next(x):
        return next(x)

elif _sys.version_info[0:2] > [2, 3]:
    # Python 2.4 or higher.
    _sys_maxint = _sys.maxint

    _int_type = (int, long)

    _str_type = basestring

    _bytes_type = str

    _is_str = lambda x: isinstance(x, basestring)

    _is_int = lambda x: isinstance(x, (int, long))

    _callable = lambda x: callable(x)

    _dict_keys = lambda x: x.keys()

    _dict_items = lambda x: x.items()

    _iter_dict_keys = lambda x: iter(x.keys())

    def _bytes_join(*args):
        return ''.join(*args)

    def _zip(*args):
        return zip(*args)

    def _range(*args, **kwargs):
        return range(*args, **kwargs)

    _iter_range = xrange

    def _func_name(f, name=None):
        if name is not None:
            f.func_name = name
        else:
            return f.func_name

    def _func_doc(f, docstring=None):
        if docstring is not None:
            f.func_doc = docstring
        else:
            return f.func_doc

    def _iter_next(x):
        return x.next()
else:
    # Unsupported versions.
    raise RuntimeError(
        'this module only supports Python 2.4.x or higher (including 3.x)!')

