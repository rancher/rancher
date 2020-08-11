# Copyright (C) 2008 John Paulett (john -at- paulett.org)
# Copyright (C) 2009-2018 David Aguilar (davvid -at- gmail.com)
# All rights reserved.
#
# This software is licensed as described in the file COPYING, which
# you should have received as part of this distribution.
from __future__ import absolute_import, division, unicode_literals
import quopri
import sys

from . import compat
from . import util
from . import tags
from . import handlers
from .compat import numeric_types
from .backend import json


def decode(
    string, backend=None, context=None, keys=False, reset=True, safe=False, classes=None
):
    """Convert a JSON string into a Python object.

    The keyword argument 'keys' defaults to False.
    If set to True then jsonpickle will decode non-string dictionary keys
    into python objects via the jsonpickle protocol.

    The keyword argument 'classes' defaults to None.
    If set to a single class, or a sequence (list, set, tuple) of classes,
    then the classes will be made available when constructing objects.  This
    can be used to give jsonpickle access to local classes that are not
    available through the global module import scope.

    >>> decode('"my string"') == 'my string'
    True
    >>> decode('36')
    36
    """
    backend = backend or json
    context = context or Unpickler(keys=keys, backend=backend, safe=safe)
    data = backend.decode(string)
    return context.restore(data, reset=reset, classes=classes)


def _safe_hasattr(obj, attr):
    """Workaround unreliable hasattr() availability on sqlalchemy objects"""
    try:
        object.__getattribute__(obj, attr)
        return True
    except AttributeError:
        return False


def _is_json_key(key):
    """Has this key a special object that has been encoded to JSON?"""
    return isinstance(key, compat.string_types) and key.startswith(tags.JSON_KEY)


class _Proxy(object):
    """Proxies are dummy objects that are later replaced by real instances

    The `restore()` function has to solve a tricky problem when pickling
    objects with cyclical references -- the parent instance does not yet
    exist.

    The problem is that `__getnewargs__()`, `__getstate__()`, custom handlers,
    and cyclical objects graphs are allowed to reference the yet-to-be-created
    object via the referencing machinery.

    In other words, objects are allowed to depend on themselves for
    construction!

    We solve this problem by placing dummy Proxy objects into the referencing
    machinery so that we can construct the child objects before constructing
    the parent.  Objects are initially created with Proxy attribute values
    instead of real references.

    We collect all objects that contain references to proxies and run
    a final sweep over them to swap in the real instance.  This is done
    at the very end of the top-level `restore()`.

    The `instance` attribute below is replaced with the real instance
    after `__new__()` has been used to construct the object and is used
    when swapping proxies with real instances.

    """

    def __init__(self):
        self.instance = None

    def get(self):
        return self.instance

    def reset(self, instance):
        self.instance = instance


class _IDProxy(_Proxy):
    def __init__(self, objs, index):
        self._index = index
        self._objs = objs

    def get(self):
        return self._objs[self._index]


def _obj_setattr(obj, attr, proxy):
    setattr(obj, attr, proxy.get())


def _obj_setvalue(obj, idx, proxy):
    obj[idx] = proxy.get()


class Unpickler(object):
    def __init__(self, backend=None, keys=False, safe=False):
        self.backend = backend or json
        self.keys = keys
        self.safe = safe

        self.reset()

    def reset(self):
        """Resets the object's internal state.
        """
        # Map reference names to object instances
        self._namedict = {}

        # The stack of names traversed for child objects
        self._namestack = []

        # Map of objects to their index in the _objs list
        self._obj_to_idx = {}
        self._objs = []
        self._proxies = []

        # Extra local classes not accessible globally
        self._classes = {}

    def restore(self, obj, reset=True, classes=None):
        """Restores a flattened object to its original python state.

        Simply returns any of the basic builtin types

        >>> u = Unpickler()
        >>> u.restore('hello world') == 'hello world'
        True
        >>> u.restore({'key': 'value'}) == {'key': 'value'}
        True

        """
        if reset:
            self.reset()
        if classes:
            self.register_classes(classes)
        value = self._restore(obj)
        if reset:
            self._swap_proxies()
        return value

    def register_classes(self, classes):
        """Register one or more classes

        :param classes: sequence of classes or a single class to register

        """
        if isinstance(classes, (list, tuple, set)):
            for cls in classes:
                self.register_classes(cls)
        else:
            self._classes[util.importable_name(classes)] = classes

    def _swap_proxies(self):
        """Replace proxies with their corresponding instances"""
        for (obj, attr, proxy, method) in self._proxies:
            method(obj, attr, proxy)
        self._proxies = []

    def _restore(self, obj):
        if has_tag(obj, tags.B64):
            restore = self._restore_base64
        elif has_tag(obj, tags.B85):
            restore = self._restore_base85
        elif has_tag(obj, tags.BYTES):  # Backwards compatibility
            restore = self._restore_quopri
        elif has_tag(obj, tags.ID):
            restore = self._restore_id
        elif has_tag(obj, tags.REF):  # Backwards compatibility
            restore = self._restore_ref
        elif has_tag(obj, tags.ITERATOR):
            restore = self._restore_iterator
        elif has_tag(obj, tags.TYPE):
            restore = self._restore_type
        elif has_tag(obj, tags.REPR):  # Backwards compatibility
            restore = self._restore_repr
        elif has_tag(obj, tags.REDUCE):
            restore = self._restore_reduce
        elif has_tag(obj, tags.OBJECT):
            restore = self._restore_object
        elif has_tag(obj, tags.FUNCTION):
            restore = self._restore_function
        elif util.is_list(obj):
            restore = self._restore_list
        elif has_tag(obj, tags.TUPLE):
            restore = self._restore_tuple
        elif has_tag(obj, tags.SET):
            restore = self._restore_set
        elif util.is_dictionary(obj):
            restore = self._restore_dict
        else:

            def restore(x):
                return x

        return restore(obj)

    def _restore_base64(self, obj):
        return util.b64decode(obj[tags.B64].encode('utf-8'))

    def _restore_base85(self, obj):
        return util.b85decode(obj[tags.B85].encode('utf-8'))

    #: For backwards compatibility with bytes data produced by older versions
    def _restore_quopri(self, obj):
        return quopri.decodestring(obj[tags.BYTES].encode('utf-8'))

    def _restore_iterator(self, obj):
        return iter(self._restore_list(obj[tags.ITERATOR]))

    def _restore_reduce(self, obj):
        """
        Supports restoring with all elements of __reduce__ as per pep 307.
        Assumes that iterator items (the last two) are represented as lists
        as per pickler implementation.
        """
        proxy = _Proxy()
        self._mkref(proxy)
        reduce_val = list(map(self._restore, obj[tags.REDUCE]))
        if len(reduce_val) < 5:
            reduce_val.extend([None] * (5 - len(reduce_val)))
        f, args, state, listitems, dictitems = reduce_val

        if f == tags.NEWOBJ or getattr(f, '__name__', '') == '__newobj__':
            # mandated special case
            cls = args[0]
            if not isinstance(cls, type):
                cls = self._restore(cls)
            stage1 = cls.__new__(cls, *args[1:])
        else:
            stage1 = f(*args)

        if state:
            try:
                stage1.__setstate__(state)
            except AttributeError:
                # it's fine - we'll try the prescribed default methods
                try:
                    # we can't do a straight update here because we
                    # need object identity of the state dict to be
                    # preserved so that _swap_proxies works out
                    for k, v in stage1.__dict__.items():
                        state.setdefault(k, v)
                    stage1.__dict__ = state
                except AttributeError:
                    # next prescribed default
                    try:
                        for k, v in state.items():
                            setattr(stage1, k, v)
                    except Exception:
                        dict_state, slots_state = state
                        if dict_state:
                            stage1.__dict__.update(dict_state)
                        if slots_state:
                            for k, v in slots_state.items():
                                setattr(stage1, k, v)

        if listitems:
            # should be lists if not None
            try:
                stage1.extend(listitems)
            except AttributeError:
                for x in listitems:
                    stage1.append(x)

        if dictitems:
            for k, v in dictitems:
                stage1.__setitem__(k, v)

        proxy.reset(stage1)
        self._swapref(proxy, stage1)
        return stage1

    def _restore_id(self, obj):
        try:
            idx = obj[tags.ID]
            return self._objs[idx]
        except IndexError:
            return _IDProxy(self._objs, idx)

    def _restore_ref(self, obj):
        return self._namedict.get(obj[tags.REF])

    def _restore_type(self, obj):
        typeref = loadclass(obj[tags.TYPE], classes=self._classes)
        if typeref is None:
            return obj
        return typeref

    def _restore_repr(self, obj):
        if self.safe:
            # eval() is not allowed in safe mode
            return None
        obj = loadrepr(obj[tags.REPR])
        return self._mkref(obj)

    def _restore_object(self, obj):
        class_name = obj[tags.OBJECT]
        cls = loadclass(class_name, classes=self._classes)
        handler = handlers.get(cls, handlers.get(class_name))
        if handler is not None:  # custom handler
            proxy = _Proxy()
            self._mkref(proxy)
            instance = handler(self).restore(obj)
            proxy.reset(instance)
            self._swapref(proxy, instance)
            return instance

        if cls is None:
            return self._mkref(obj)

        return self._restore_object_instance(obj, cls)

    def _restore_function(self, obj):
        return loadclass(obj[tags.FUNCTION], classes=self._classes)

    def _loadfactory(self, obj):
        try:
            default_factory = obj['default_factory']
        except KeyError:
            return None
        del obj['default_factory']
        return self._restore(default_factory)

    def _restore_object_instance(self, obj, cls):
        # This is a placeholder proxy object which allows child objects to
        # reference the parent object before it has been instantiated.
        proxy = _Proxy()
        self._mkref(proxy)

        # An object can install itself as its own factory, so load the factory
        # after the instance is available for referencing.
        factory = self._loadfactory(obj)

        if has_tag(obj, tags.NEWARGSEX):
            args, kwargs = obj[tags.NEWARGSEX]
        else:
            args = getargs(obj, classes=self._classes)
            kwargs = {}
        if args:
            args = self._restore(args)
        if kwargs:
            kwargs = self._restore(kwargs)

        is_oldstyle = not (isinstance(cls, type) or getattr(cls, '__meta__', None))
        try:
            if (not is_oldstyle) and hasattr(cls, '__new__'):
                # new style classes
                if factory:
                    instance = cls.__new__(cls, factory, *args, **kwargs)
                    instance.default_factory = factory
                else:
                    instance = cls.__new__(cls, *args, **kwargs)
            else:
                instance = object.__new__(cls)
        except TypeError:  # old-style classes
            is_oldstyle = True

        if is_oldstyle:
            try:
                instance = cls(*args)
            except TypeError:  # fail gracefully
                try:
                    instance = make_blank_classic(cls)
                except Exception:  # fail gracefully
                    return self._mkref(obj)

        proxy.reset(instance)
        self._swapref(proxy, instance)

        if isinstance(instance, tuple):
            return instance

        instance = self._restore_object_instance_variables(obj, instance)

        if _safe_hasattr(instance, 'default_factory') and isinstance(
            instance.default_factory, _Proxy
        ):
            instance.default_factory = instance.default_factory.get()

        return instance

    def _restore_from_dict(self, obj, instance, ignorereserved=True):
        restore_key = self._restore_key_fn()
        method = _obj_setattr
        deferred = {}

        for k, v in util.items(obj):
            # ignore the reserved attribute
            if ignorereserved and k in tags.RESERVED:
                continue
            if isinstance(k, numeric_types):
                str_k = k.__str__()
            else:
                str_k = k
            self._namestack.append(str_k)
            k = restore_key(k)
            # step into the namespace
            value = self._restore(v)
            if util.is_noncomplex(instance) or util.is_dictionary_subclass(instance):
                try:
                    if k == '__dict__':
                        setattr(instance, k, value)
                    else:
                        instance[k] = value
                except TypeError:
                    # Immutable object, must be constructed in one shot
                    if k != '__dict__':
                        deferred[k] = value
                    self._namestack.pop()
                    continue
            else:
                setattr(instance, k, value)

            # This instance has an instance variable named `k` that is
            # currently a proxy and must be replaced
            if isinstance(value, _Proxy):
                self._proxies.append((instance, k, value, method))

            # step out
            self._namestack.pop()

        if deferred:
            # SQLAlchemy Immutable mappings must be constructed in one shot
            instance = instance.__class__(deferred)

        return instance

    def _restore_object_instance_variables(self, obj, instance):
        instance = self._restore_from_dict(obj, instance)

        # Handle list and set subclasses
        if has_tag(obj, tags.SEQ):
            if hasattr(instance, 'append'):
                for v in obj[tags.SEQ]:
                    instance.append(self._restore(v))
            elif hasattr(instance, 'add'):
                for v in obj[tags.SEQ]:
                    instance.add(self._restore(v))

        if has_tag(obj, tags.STATE):
            instance = self._restore_state(obj, instance)

        return instance

    def _restore_state(self, obj, instance):
        state = self._restore(obj[tags.STATE])
        has_slots = (
            isinstance(state, tuple) and len(state) == 2 and isinstance(state[1], dict)
        )
        has_slots_and_dict = has_slots and isinstance(state[0], dict)
        if hasattr(instance, '__setstate__'):
            instance.__setstate__(state)
        elif isinstance(state, dict):
            # implements described default handling
            # of state for object with instance dict
            # and no slots
            instance = self._restore_from_dict(state, instance, ignorereserved=False)
        elif has_slots:
            instance = self._restore_from_dict(state[1], instance, ignorereserved=False)
            if has_slots_and_dict:
                instance = self._restore_from_dict(
                    state[0], instance, ignorereserved=False
                )
        elif not hasattr(instance, '__getnewargs__') and not hasattr(
            instance, '__getnewargs_ex__'
        ):
            # __setstate__ is not implemented so that means that the best
            # we can do is return the result of __getstate__() rather than
            # return an empty shell of an object.
            # However, if there were newargs, it's not an empty shell
            instance = state
        return instance

    def _restore_list(self, obj):
        parent = []
        self._mkref(parent)
        children = [self._restore(v) for v in obj]
        parent.extend(children)
        method = _obj_setvalue
        proxies = [
            (parent, idx, value, method)
            for idx, value in enumerate(parent)
            if isinstance(value, _Proxy)
        ]
        self._proxies.extend(proxies)
        return parent

    def _restore_tuple(self, obj):
        return tuple([self._restore(v) for v in obj[tags.TUPLE]])

    def _restore_set(self, obj):
        return {self._restore(v) for v in obj[tags.SET]}

    def _restore_dict(self, obj):
        data = {}

        # If we are decoding dicts that can have non-string keys then we
        # need to do a two-phase decode where the non-string keys are
        # processed last.  This ensures a deterministic order when
        # assigning object IDs for references.
        if self.keys:
            # Phase 1: regular non-special keys.
            for k, v in util.items(obj):
                if _is_json_key(k):
                    continue
                if isinstance(k, numeric_types):
                    str_k = k.__str__()
                else:
                    str_k = k
                self._namestack.append(str_k)
                data[k] = self._restore(v)

                self._namestack.pop()

            # Phase 2: object keys only.
            for k, v in util.items(obj):
                if not _is_json_key(k):
                    continue
                self._namestack.append(k)

                k = self._restore_pickled_key(k)
                data[k] = result = self._restore(v)
                # k is currently a proxy and must be replaced
                if isinstance(result, _Proxy):
                    self._proxies.append((data, k, result, _obj_setvalue))

                self._namestack.pop()
        else:
            # No special keys, thus we don't need to restore the keys either.
            for k, v in util.items(obj):
                if isinstance(k, numeric_types):
                    str_k = k.__str__()
                else:
                    str_k = k
                self._namestack.append(str_k)
                data[k] = self._restore(v)
                self._namestack.pop()
        return data

    def _restore_key_fn(self):
        """Return a callable that restores keys

        This function is responsible for restoring non-string keys
        when we are decoding with `keys=True`.

        """
        # This function is called before entering a tight loop
        # where the returned function will be called.
        # We return a specific function after checking self.keys
        # instead of doing so in the body of the function to
        # avoid conditional branching inside a tight loop.
        if self.keys:
            restore_key = self._restore_pickled_key
        else:

            def restore_key(key):
                return key

        return restore_key

    def _restore_pickled_key(self, key):
        """Restore a possibly pickled key"""
        if _is_json_key(key):
            key = decode(
                key[len(tags.JSON_KEY) :],
                backend=self.backend,
                context=self,
                keys=True,
                reset=False,
            )
        return key

    def _refname(self):
        """Calculates the name of the current location in the JSON stack.

        This is called as jsonpickle traverses the object structure to
        create references to previously-traversed objects.  This allows
        cyclical data structures such as doubly-linked lists.
        jsonpickle ensures that duplicate python references to the same
        object results in only a single JSON object definition and
        special reference tags to represent each reference.

        >>> u = Unpickler()
        >>> u._namestack = []
        >>> u._refname() == '/'
        True
        >>> u._namestack = ['a']
        >>> u._refname() == '/a'
        True
        >>> u._namestack = ['a', 'b']
        >>> u._refname() == '/a/b'
        True

        """
        return '/' + '/'.join(self._namestack)

    def _mkref(self, obj):
        obj_id = id(obj)
        try:
            self._obj_to_idx[obj_id]
        except KeyError:
            self._obj_to_idx[obj_id] = len(self._objs)
            self._objs.append(obj)
            # Backwards compatibility: old versions of jsonpickle
            # produced "py/ref" references.
            self._namedict[self._refname()] = obj
        return obj

    def _swapref(self, proxy, instance):
        proxy_id = id(proxy)
        instance_id = id(instance)

        instance_index = self._obj_to_idx[proxy_id]
        self._obj_to_idx[instance_id] = instance_index
        del self._obj_to_idx[proxy_id]

        self._objs[instance_index] = instance
        self._namedict[self._refname()] = instance


def loadclass(module_and_name, classes=None):
    """Loads the module and returns the class.

    >>> cls = loadclass('datetime.datetime')
    >>> cls.__name__
    'datetime'

    >>> loadclass('does.not.exist')

    >>> loadclass('builtins.int')()
    0

    """
    # Check if the class exists in a caller-provided scope
    if classes:
        try:
            return classes[module_and_name]
        except KeyError:
            pass
    # Otherwise, load classes from globally-accessible imports
    names = module_and_name.split('.')
    # First assume that everything up to the last dot is the module name,
    # then try other splits to handle classes that are defined within
    # classes
    for up_to in range(len(names) - 1, 0, -1):
        module = util.untranslate_module_name('.'.join(names[:up_to]))
        try:
            __import__(module)
            obj = sys.modules[module]
            for class_name in names[up_to:]:
                obj = getattr(obj, class_name)
            return obj
        except (AttributeError, ImportError, ValueError):
            continue
    return None


def getargs(obj, classes=None):
    """Return arguments suitable for __new__()"""
    # Let saved newargs take precedence over everything
    if has_tag(obj, tags.NEWARGSEX):
        raise ValueError("__newargs_ex__ returns both args and kwargs")

    if has_tag(obj, tags.NEWARGS):
        return obj[tags.NEWARGS]

    if has_tag(obj, tags.INITARGS):
        return obj[tags.INITARGS]

    try:
        seq_list = obj[tags.SEQ]
        obj_dict = obj[tags.OBJECT]
    except KeyError:
        return []
    typeref = loadclass(obj_dict, classes=classes)
    if not typeref:
        return []
    if hasattr(typeref, '_fields'):
        if len(typeref._fields) == len(seq_list):
            return seq_list
    return []


class _trivialclassic:
    """
    A trivial class that can be instantiated with no args
    """


def make_blank_classic(cls):
    """
    Implement the mandated strategy for dealing with classic classes
    which cannot be instantiated without __getinitargs__ because they
    take parameters
    """
    instance = _trivialclassic()
    instance.__class__ = cls
    return instance


def loadrepr(reprstr):
    """Returns an instance of the object from the object's repr() string.
    It involves the dynamic specification of code.

    >>> obj = loadrepr('datetime/datetime.datetime.now()')
    >>> obj.__class__.__name__
    'datetime'

    """
    module, evalstr = reprstr.split('/')
    mylocals = locals()
    localname = module
    if '.' in localname:
        localname = module.split('.', 1)[0]
    mylocals[localname] = __import__(module)
    return eval(evalstr)


def has_tag(obj, tag):
    """Helper class that tests to see if the obj is a dictionary
    and contains a particular key/tag.

    >>> obj = {'test': 1}
    >>> has_tag(obj, 'test')
    True
    >>> has_tag(obj, 'fail')
    False

    >>> has_tag(42, 'fail')
    False

    """
    return type(obj) is dict and tag in obj
