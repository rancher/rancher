# -*- coding: utf-8 -*-
#
# Copyright (C) 2008 John Paulett (john -at- paulett.org)
# Copyright (C) 2009, 2011, 2013 David Aguilar (davvid -at- gmail.com)
# All rights reserved.
#
# This software is licensed as described in the file COPYING, which
# you should have received as part of this distribution.

"""Python library for serializing any arbitrary object graph into JSON.

.. warning::

    jsonpickle can execute arbitrary Python code. Do not load jsonpickles from
    untrusted / unauthenticated sources.

jsonpickle can take almost any Python object and turn the object into JSON.
Additionally, it can reconstitute the object back into Python.

The object must be accessible globally via a module and must
inherit from object (AKA new-style classes).

Create an object::

    class Thing(object):
        def __init__(self, name):
            self.name = name

    obj = Thing('Awesome')

Use jsonpickle to transform the object into a JSON string::

    import jsonpickle
    frozen = jsonpickle.encode(obj)

Use jsonpickle to recreate a Python object from a JSON string::

    thawed = jsonpickle.decode(frozen)

The new object has the same type and data, but essentially is now a copy of
the original.

.. code-block:: python

    assert obj.name == thawed.name

If you will never need to load (regenerate the Python class from JSON), you can
pass in the keyword unpicklable=False to prevent extra information from being
added to JSON::

    oneway = jsonpickle.encode(obj, unpicklable=False)
    result = jsonpickle.decode(oneway)
    assert obj.name == result['name'] == 'Awesome'

"""
from __future__ import absolute_import, division, unicode_literals

from .backend import json
from .pickler import encode
from .unpickler import decode

# Export other names not in __all__
from .backend import JSONBackend  # noqa: F401
from .version import __version__  # noqa: F401
from .handlers import register  # noqa: F401
from .handlers import unregister  # noqa: F401
from .pickler import Pickler  # noqa: F401
from .unpickler import Unpickler  # noqa: F401

__all__ = ('encode', 'decode')

# register built-in handlers
__import__('jsonpickle.handlers', level=0)

# Export specific JSONPluginMgr methods into the jsonpickle namespace
set_preferred_backend = json.set_preferred_backend
set_decoder_options = json.set_decoder_options
set_encoder_options = json.set_encoder_options
load_backend = json.load_backend
remove_backend = json.remove_backend
enable_fallthrough = json.enable_fallthrough

# json.load(), loads(), dump(), dumps() compatibility
dumps = encode
loads = decode
