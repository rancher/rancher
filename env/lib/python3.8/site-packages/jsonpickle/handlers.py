"""
Custom handlers may be created to handle other objects. Each custom handler
must derive from :class:`jsonpickle.handlers.BaseHandler` and
implement ``flatten`` and ``restore``.

A handler can be bound to other types by calling
:func:`jsonpickle.handlers.register`.

"""
from __future__ import absolute_import, division, unicode_literals
import array
import copy
import datetime
import io
import re
import sys
import threading
import uuid

from . import compat
from . import util


class Registry(object):
    def __init__(self):
        self._handlers = {}
        self._base_handlers = {}

    def get(self, cls_or_name, default=None):
        """
        :param cls_or_name: the type or its fully qualified name
        :param default: default value, if a matching handler is not found

        Looks up a handler by type reference or its fully
        qualified name. If a direct match
        is not found, the search is performed over all
        handlers registered with base=True.
        """
        handler = self._handlers.get(cls_or_name)
        # attempt to find a base class
        if handler is None and util.is_type(cls_or_name):
            for cls, base_handler in self._base_handlers.items():
                if issubclass(cls_or_name, cls):
                    return base_handler
        return default if handler is None else handler

    def register(self, cls, handler=None, base=False):
        """Register the a custom handler for a class

        :param cls: The custom object class to handle
        :param handler: The custom handler class (if
            None, a decorator wrapper is returned)
        :param base: Indicates whether the handler should
            be registered for all subclasses

        This function can be also used as a decorator
        by omitting the `handler` argument::

            @jsonpickle.handlers.register(Foo, base=True)
            class FooHandler(jsonpickle.handlers.BaseHandler):
                pass

        """
        if handler is None:

            def _register(handler_cls):
                self.register(cls, handler=handler_cls, base=base)
                return handler_cls

            return _register
        if not util.is_type(cls):
            raise TypeError('{!r} is not a class/type'.format(cls))
        # store both the name and the actual type for the ugly cases like
        # _sre.SRE_Pattern that cannot be loaded back directly
        self._handlers[util.importable_name(cls)] = self._handlers[cls] = handler
        if base:
            # only store the actual type for subclass checking
            self._base_handlers[cls] = handler

    def unregister(self, cls):
        self._handlers.pop(cls, None)
        self._handlers.pop(util.importable_name(cls), None)
        self._base_handlers.pop(cls, None)


registry = Registry()
register = registry.register
unregister = registry.unregister
get = registry.get


class BaseHandler(object):
    def __init__(self, context):
        """
        Initialize a new handler to handle a registered type.

        :Parameters:
          - `context`: reference to pickler/unpickler

        """
        self.context = context

    def __call__(self, context):
        """This permits registering either Handler instances or classes

        :Parameters:
          - `context`: reference to pickler/unpickler
        """
        self.context = context
        return self

    def flatten(self, obj, data):
        """
        Flatten `obj` into a json-friendly form and write result to `data`.

        :param object obj: The object to be serialized.
        :param dict data: A partially filled dictionary which will contain the
            json-friendly representation of `obj` once this method has
            finished.
        """
        raise NotImplementedError('You must implement flatten() in %s' % self.__class__)

    def restore(self, obj):
        """
        Restore an object of the registered type from the json-friendly
        representation `obj` and return it.
        """
        raise NotImplementedError('You must implement restore() in %s' % self.__class__)

    @classmethod
    def handles(self, cls):
        """
        Register this handler for the given class. Suitable as a decorator,
        e.g.::

            @MyCustomHandler.handles
            class MyCustomClass:
                def __reduce__(self):
                    ...
        """
        registry.register(cls, self)
        return cls


class ArrayHandler(BaseHandler):
    """Flatten and restore array.array objects"""

    def flatten(self, obj, data):
        data['typecode'] = obj.typecode
        data['values'] = self.context.flatten(obj.tolist(), reset=False)
        return data

    def restore(self, data):
        typecode = data['typecode']
        values = self.context.restore(data['values'], reset=False)
        if typecode == 'c':
            values = [bytes(x) for x in values]
        return array.array(typecode, values)


ArrayHandler.handles(array.array)


class DatetimeHandler(BaseHandler):

    """Custom handler for datetime objects

    Datetime objects use __reduce__, and they generate binary strings encoding
    the payload. This handler encodes that payload to reconstruct the
    object.

    """

    def flatten(self, obj, data):
        pickler = self.context
        if not pickler.unpicklable:
            if hasattr(obj, 'isoformat'):
                result = obj.isoformat()
            else:
                result = compat.ustr(obj)
            return result
        cls, args = obj.__reduce__()
        flatten = pickler.flatten
        payload = util.b64encode(args[0])
        args = [payload] + [flatten(i, reset=False) for i in args[1:]]
        data['__reduce__'] = (flatten(cls, reset=False), args)
        return data

    def restore(self, data):
        cls, args = data['__reduce__']
        unpickler = self.context
        restore = unpickler.restore
        cls = restore(cls, reset=False)
        value = util.b64decode(args[0])
        params = (value,) + tuple([restore(i, reset=False) for i in args[1:]])
        return cls.__new__(cls, *params)


DatetimeHandler.handles(datetime.datetime)
DatetimeHandler.handles(datetime.date)
DatetimeHandler.handles(datetime.time)


class RegexHandler(BaseHandler):
    """Flatten _sre.SRE_Pattern (compiled regex) objects"""

    def flatten(self, obj, data):
        data['pattern'] = obj.pattern
        return data

    def restore(self, data):
        return re.compile(data['pattern'])


RegexHandler.handles(type(re.compile('')))


class QueueHandler(BaseHandler):
    """Opaquely serializes Queue objects

    Queues contains mutex and condition variables which cannot be serialized.
    Construct a new Queue instance when restoring.

    """

    def flatten(self, obj, data):
        return data

    def restore(self, data):
        return compat.queue.Queue()


QueueHandler.handles(compat.queue.Queue)


class CloneFactory(object):
    """Serialization proxy for collections.defaultdict's default_factory"""

    def __init__(self, exemplar):
        self.exemplar = exemplar

    def __call__(self, clone=copy.copy):
        """Create new instances by making copies of the provided exemplar"""
        return clone(self.exemplar)

    def __repr__(self):
        return '<CloneFactory object at 0x{:x} ({})>'.format(id(self), self.exemplar)


class UUIDHandler(BaseHandler):
    """Serialize uuid.UUID objects"""

    def flatten(self, obj, data):
        data['hex'] = obj.hex
        return data

    def restore(self, data):
        return uuid.UUID(data['hex'])


UUIDHandler.handles(uuid.UUID)


class LockHandler(BaseHandler):
    """Serialize threading.Lock objects"""

    def flatten(self, obj, data):
        data['locked'] = obj.locked()
        return data

    def restore(self, data):
        lock = threading.Lock()
        if data.get('locked', False):
            lock.acquire()
        return lock


_lock = threading.Lock()
LockHandler.handles(_lock.__class__)


class TextIOHandler(BaseHandler):
    """Serialize file descriptors as None because we cannot roundtrip"""

    def flatten(self, obj, data):
        return None

    def restore(self, data):
        """Restore should never get called because flatten() returns None"""
        raise AssertionError('Restoring IO.TextIOHandler is not supported')


if sys.version_info >= (3, 8):
    TextIOHandler.handles(io.TextIOWrapper)
