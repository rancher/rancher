from __future__ import absolute_import, division, unicode_literals

from .compat import string_types
from .compat import PY3_ORDERED_DICT


class JSONBackend(object):
    """Manages encoding and decoding using various backends.

    It tries these modules in this order:
        simplejson, json, demjson

    simplejson is a fast and popular backend and is tried first.
    json comes with Python and is tried second.
    demjson is the most permissive backend and is tried last.

    """

    def __init__(self, fallthrough=True):
        # Whether we should fallthrough to the next backend
        self._fallthrough = fallthrough
        # The names of backends that have been successfully imported
        self._backend_names = []

        # A dictionary mapping backend names to encode/decode functions
        self._encoders = {}
        self._decoders = {}

        # Options to pass to specific encoders
        self._encoder_options = {}

        # Options to pass to specific decoders
        self._decoder_options = {}

        # The exception class that is thrown when a decoding error occurs
        self._decoder_exceptions = {}

        # Whether we've loaded any backends successfully
        self._verified = False

        self.load_backend('simplejson')
        self.load_backend('json')
        self.load_backend('demjson', 'encode', 'decode', 'JSONDecodeError')
        self.load_backend('jsonlib', 'write', 'read', 'ReadError')
        self.load_backend('yajl')
        self.load_backend('ujson')

        # Defaults for various encoders
        sort = not PY3_ORDERED_DICT
        json_opts = ((), {'sort_keys': sort})
        self._encoder_options = {
            'ujson': ((), {'sort_keys': sort, 'escape_forward_slashes': False}),
            'json': json_opts,
            'simplejson': json_opts,
            'django.util.simplejson': json_opts,
        }

    def _verify(self):
        """Ensures that we've loaded at least one JSON backend."""
        if self._verified:
            return
        raise AssertionError(
            'jsonpickle requires at least one of the '
            'following:\n'
            '    python2.6, simplejson, or demjson'
        )

    def enable_fallthrough(self, enable):
        """
        Disable jsonpickle's fallthrough-on-error behavior

        By default, jsonpickle tries the next backend when decoding or
        encoding using a backend fails.

        This can make it difficult to force jsonpickle to use a specific
        backend, and catch errors, because the error will be suppressed and
        may not be raised by the subsequent backend.

        Calling `enable_backend(False)` will make jsonpickle immediately
        re-raise any exceptions raised by the backends.

        """
        self._fallthrough = enable

    def load_backend(self, name, dumps='dumps', loads='loads', loads_exc=ValueError):

        """Load a JSON backend by name.

        This method loads a backend and sets up references to that
        backend's loads/dumps functions and exception classes.

        :param dumps: is the name of the backend's encode method.
          The method should take an object and return a string.
          Defaults to 'dumps'.
        :param loads: names the backend's method for the reverse
          operation -- returning a Python object from a string.
        :param loads_exc: can be either the name of the exception class
          used to denote decoding errors, or it can be a direct reference
          to the appropriate exception class itself.  If it is a name,
          then the assumption is that an exception class of that name
          can be found in the backend module's namespace.
        :param load: names the backend's 'load' method.
        :param dump: names the backend's 'dump' method.
        :rtype bool: True on success, False if the backend could not be loaded.

        """
        try:
            # Load the JSON backend
            mod = __import__(name)
        except ImportError:
            return False

        # Handle submodules, e.g. django.utils.simplejson
        try:
            for attr in name.split('.')[1:]:
                mod = getattr(mod, attr)
        except AttributeError:
            return False

        if not self._store(self._encoders, name, mod, dumps) or not self._store(
            self._decoders, name, mod, loads
        ):
            return False

        if isinstance(loads_exc, string_types):
            # This backend's decoder exception is part of the backend
            if not self._store(self._decoder_exceptions, name, mod, loads_exc):
                return False
        else:
            # simplejson uses ValueError
            self._decoder_exceptions[name] = loads_exc

        # Setup the default args and kwargs for this encoder/decoder
        self._encoder_options.setdefault(name, ([], {}))
        self._decoder_options.setdefault(name, ([], {}))

        # Add this backend to the list of candidate backends
        self._backend_names.append(name)

        # Indicate that we successfully loaded a JSON backend
        self._verified = True
        return True

    def remove_backend(self, name):
        """Remove all entries for a particular backend."""
        self._encoders.pop(name, None)
        self._decoders.pop(name, None)
        self._decoder_exceptions.pop(name, None)
        self._decoder_options.pop(name, None)
        self._encoder_options.pop(name, None)
        if name in self._backend_names:
            self._backend_names.remove(name)
        self._verified = bool(self._backend_names)

    def encode(self, obj, indent=None, separators=None):
        """
        Attempt to encode an object into JSON.

        This tries the loaded backends in order and passes along the last
        exception if no backend is able to encode the object.

        """
        self._verify()

        if not self._fallthrough:
            name = self._backend_names[0]
            return self.backend_encode(name, obj, indent=indent, separators=separators)

        for idx, name in enumerate(self._backend_names):
            try:
                return self.backend_encode(
                    name, obj, indent=indent, separators=separators
                )
            except Exception as e:
                if idx == len(self._backend_names) - 1:
                    raise e

    # def dumps
    dumps = encode

    def backend_encode(self, name, obj, indent=None, separators=None):
        optargs, optkwargs = self._encoder_options.get(name, ([], {}))
        encoder_kwargs = optkwargs.copy()
        if indent is not None:
            encoder_kwargs['indent'] = indent
        if separators is not None:
            encoder_kwargs['separators'] = separators
        encoder_args = (obj,) + tuple(optargs)
        return self._encoders[name](*encoder_args, **encoder_kwargs)

    def decode(self, string):
        """
        Attempt to decode an object from a JSON string.

        This tries the loaded backends in order and passes along the last
        exception if no backends are able to decode the string.

        """
        self._verify()

        if not self._fallthrough:
            name = self._backend_names[0]
            return self.backend_decode(name, string)

        for idx, name in enumerate(self._backend_names):
            try:
                return self.backend_decode(name, string)
            except self._decoder_exceptions[name] as e:
                if idx == len(self._backend_names) - 1:
                    raise e
                else:
                    pass  # and try a more forgiving encoder, e.g. demjson

    # def loads
    loads = decode

    def backend_decode(self, name, string):
        optargs, optkwargs = self._decoder_options.get(name, ((), {}))
        decoder_kwargs = optkwargs.copy()
        return self._decoders[name](string, *optargs, **decoder_kwargs)

    def set_preferred_backend(self, name):
        """
        Set the preferred json backend.

        If a preferred backend is set then jsonpickle tries to use it
        before any other backend.

        For example::

            set_preferred_backend('simplejson')

        If the backend is not one of the built-in jsonpickle backends
        (json/simplejson, or demjson) then you must load the backend
        prior to calling set_preferred_backend.

        AssertionError is raised if the backend has not been loaded.

        """
        if name in self._backend_names:
            self._backend_names.remove(name)
            self._backend_names.insert(0, name)
        else:
            errmsg = 'The "%s" backend has not been loaded.' % name
            raise AssertionError(errmsg)

    def set_encoder_options(self, name, *args, **kwargs):
        """
        Associate encoder-specific options with an encoder.

        After calling set_encoder_options, any calls to jsonpickle's
        encode method will pass the supplied args and kwargs along to
        the appropriate backend's encode method.

        For example::

            set_encoder_options('simplejson', sort_keys=True, indent=4)
            set_encoder_options('demjson', compactly=False)

        See the appropriate encoder's documentation for details about
        the supported arguments and keyword arguments.

        """
        self._encoder_options[name] = (args, kwargs)

    def set_decoder_options(self, name, *args, **kwargs):
        """
        Associate decoder-specific options with a decoder.

        After calling set_decoder_options, any calls to jsonpickle's
        decode method will pass the supplied args and kwargs along to
        the appropriate backend's decode method.

        For example::

            set_decoder_options('simplejson', encoding='utf8', cls=JSONDecoder)
            set_decoder_options('demjson', strict=True)

        See the appropriate decoder's documentation for details about
        the supported arguments and keyword arguments.

        """
        self._decoder_options[name] = (args, kwargs)

    def _store(self, dct, backend, obj, name):
        try:
            dct[backend] = getattr(obj, name)
        except AttributeError:
            self.remove_backend(backend)
            return False
        return True


json = JSONBackend()
