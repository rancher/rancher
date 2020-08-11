import os
import sys
import imp

from . import Config
from .exceptions import CollectionNotFound
from .util import debug


class Loader(object):
    """
    Abstract class defining how to find/import a session's base `.Collection`.

    .. versionadded:: 1.0
    """

    def __init__(self, config=None):
        """
        Set up a new loader with some `.Config`.

        :param config:
            An explicit `.Config` to use; it is referenced for loading-related
            config options. Defaults to an anonymous ``Config()`` if none is
            given.
        """
        if config is None:
            config = Config()
        self.config = config

    def find(self, name):
        """
        Implementation-specific finder method seeking collection ``name``.

        Must return a 4-tuple valid for use by `imp.load_module`, which is
        typically a name string followed by the contents of the 3-tuple
        returned by `imp.find_module` (``file``, ``pathname``,
        ``description``.)

        For a sample implementation, see `.FilesystemLoader`.

        .. versionadded:: 1.0
        """
        raise NotImplementedError

    def load(self, name=None):
        """
        Load and return collection module identified by ``name``.

        This method requires a working implementation of `.find` in order to
        function.

        In addition to importing the named module, it will add the module's
        parent directory to the front of `sys.path` to provide normal Python
        import behavior (i.e. so the loaded module may load local-to-it modules
        or packages.)

        :returns:
            Two-tuple of ``(module, directory)`` where ``module`` is the
            collection-containing Python module object, and ``directory`` is
            the string path to the directory the module was found in.

        .. versionadded:: 1.0
        """
        if name is None:
            name = self.config.tasks.collection_name
        # Find the named tasks module, depending on implementation.
        # Will raise an exception if not found.
        fd, path, desc = self.find(name)
        try:
            # Ensure containing directory is on sys.path in case the module
            # being imported is trying to load local-to-it names.
            parent = os.path.dirname(path)
            if parent not in sys.path:
                sys.path.insert(0, parent)
            # Actual import
            module = imp.load_module(name, fd, path, desc)
            # Return module + path.
            # TODO: is there a reason we're not simply having clients refer to
            # os.path.dirname(module.__file__)?
            return module, parent
        finally:
            # Ensure we clean up the opened file object returned by find(), if
            # there was one (eg found packages, vs modules, don't open any
            # file.)
            if fd:
                fd.close()


class FilesystemLoader(Loader):
    """
    Loads Python files from the filesystem (e.g. ``tasks.py``.)

    Searches recursively towards filesystem root from a given start point.

    .. versionadded:: 1.0
    """

    # TODO: could introduce config obj here for transmission to Collection
    # TODO: otherwise Loader has to know about specific bits to transmit, such
    # as auto-dashes, and has to grow one of those for every bit Collection
    # ever needs to know
    def __init__(self, start=None, **kwargs):
        super(FilesystemLoader, self).__init__(**kwargs)
        if start is None:
            start = self.config.tasks.search_root
        self._start = start

    @property
    def start(self):
        # Lazily determine default CWD if configured value is falsey
        return self._start or os.getcwd()

    def find(self, name):
        # Accumulate all parent directories
        start = self.start
        debug("FilesystemLoader find starting at {!r}".format(start))
        parents = [os.path.abspath(start)]
        parents.append(os.path.dirname(parents[-1]))
        while parents[-1] != parents[-2]:
            parents.append(os.path.dirname(parents[-1]))
        # Make sure we haven't got duplicates on the end
        if parents[-1] == parents[-2]:
            parents = parents[:-1]
        # Use find_module with our list of parents. ImportError from
        # find_module means "couldn't find" not "found and couldn't import" so
        # we turn it into a more obvious exception class.
        try:
            tup = imp.find_module(name, parents)
            debug("Found module: {!r}".format(tup[1]))
            return tup
        except ImportError:
            msg = "ImportError loading {!r}, raising CollectionNotFound"
            debug(msg.format(name))
            raise CollectionNotFound(name=name, start=start)
