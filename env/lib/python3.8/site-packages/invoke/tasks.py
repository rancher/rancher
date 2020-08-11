"""
This module contains the core `.Task` class & convenience decorators used to
generate new tasks.
"""

from copy import deepcopy
import inspect
import types

from .util import six

if six.PY3:
    from itertools import zip_longest
else:
    from itertools import izip_longest as zip_longest

from .context import Context
from .parser import Argument, translate_underscores


#: Sentinel object representing a truly blank value (vs ``None``).
NO_DEFAULT = object()


class Task(object):
    """
    Core object representing an executable task & its argument specification.

    For the most part, this object is a clearinghouse for all of the data that
    may be supplied to the `@task <invoke.tasks.task>` decorator, such as
    ``name``, ``aliases``, ``positional`` etc, which appear as attributes.

    In addition, instantiation copies some introspection/documentation friendly
    metadata off of the supplied ``body`` object, such as ``__doc__``,
    ``__name__`` and ``__module__``, allowing it to "appear as" ``body`` for
    most intents and purposes.

    .. versionadded:: 1.0
    """

    # TODO: store these kwarg defaults central, refer to those values both here
    # and in @task.
    # TODO: allow central per-session / per-taskmodule control over some of
    # them, e.g. (auto_)positional, auto_shortflags.
    # NOTE: we shadow __builtins__.help here on purpose - obfuscating to avoid
    # it feels bad, given the builtin will never actually be in play anywhere
    # except a debug shell whose frame is exactly inside this class.
    def __init__(
        self,
        body,
        name=None,
        aliases=(),
        positional=None,
        optional=(),
        default=False,
        auto_shortflags=True,
        help=None,
        pre=None,
        post=None,
        autoprint=False,
        iterable=None,
        incrementable=None,
    ):
        # Real callable
        self.body = body
        # Copy a bunch of special properties from the body for the benefit of
        # Sphinx autodoc or other introspectors.
        self.__doc__ = getattr(body, "__doc__", "")
        self.__name__ = getattr(body, "__name__", "")
        self.__module__ = getattr(body, "__module__", "")
        # Default name, alternate names, and whether it should act as the
        # default for its parent collection
        self._name = name
        self.aliases = aliases
        self.is_default = default
        # Arg/flag/parser hints
        self.positional = self.fill_implicit_positionals(positional)
        self.optional = optional
        self.iterable = iterable or []
        self.incrementable = incrementable or []
        self.auto_shortflags = auto_shortflags
        self.help = help or {}
        # Call chain bidness
        self.pre = pre or []
        self.post = post or []
        self.times_called = 0
        # Whether to print return value post-execution
        self.autoprint = autoprint

    @property
    def name(self):
        return self._name or self.__name__

    def __repr__(self):
        aliases = ""
        if self.aliases:
            aliases = " ({})".format(", ".join(self.aliases))
        return "<Task {!r}{}>".format(self.name, aliases)

    def __eq__(self, other):
        if self.name != other.name:
            return False
        # Functions do not define __eq__ but func_code objects apparently do.
        # (If we're wrapping some other callable, they will be responsible for
        # defining equality on their end.)
        if self.body == other.body:
            return True
        else:
            try:
                return six.get_function_code(
                    self.body
                ) == six.get_function_code(other.body)
            except AttributeError:
                return False

    def __hash__(self):
        # Presumes name and body will never be changed. Hrm.
        # Potentially cleaner to just not use Tasks as hash keys, but let's do
        # this for now.
        return hash(self.name) + hash(self.body)

    def __call__(self, *args, **kwargs):
        # Guard against calling tasks with no context.
        if not isinstance(args[0], Context):
            err = "Task expected a Context as its first arg, got {} instead!"
            # TODO: raise a custom subclass _of_ TypeError instead
            raise TypeError(err.format(type(args[0])))
        result = self.body(*args, **kwargs)
        self.times_called += 1
        return result

    @property
    def called(self):
        return self.times_called > 0

    def argspec(self, body):
        """
        Returns two-tuple:

        * First item is list of arg names, in order defined.

            * I.e. we *cannot* simply use a dict's ``keys()`` method here.

        * Second item is dict mapping arg names to default values or
          `.NO_DEFAULT` (an 'empty' value distinct from None, since None
          is a valid value on its own).

        .. versionadded:: 1.0
        """
        # Handle callable-but-not-function objects
        # TODO: __call__ exhibits the 'self' arg; do we manually nix 1st result
        # in argspec, or is there a way to get the "really callable" spec?
        func = body if isinstance(body, types.FunctionType) else body.__call__
        spec = inspect.getargspec(func)
        arg_names = spec.args[:]
        matched_args = [reversed(x) for x in [spec.args, spec.defaults or []]]
        spec_dict = dict(zip_longest(*matched_args, fillvalue=NO_DEFAULT))
        # Pop context argument
        try:
            context_arg = arg_names.pop(0)
        except IndexError:
            # TODO: see TODO under __call__, this should be same type
            raise TypeError("Tasks must have an initial Context argument!")
        del spec_dict[context_arg]
        return arg_names, spec_dict

    def fill_implicit_positionals(self, positional):
        args, spec_dict = self.argspec(self.body)
        # If positionals is None, everything lacking a default
        # value will be automatically considered positional.
        if positional is None:
            positional = []
            for name in args:  # Go in defined order, not dict "order"
                default = spec_dict[name]
                if default is NO_DEFAULT:
                    positional.append(name)
        return positional

    def arg_opts(self, name, default, taken_names):
        opts = {}
        # Whether it's positional or not
        opts["positional"] = name in self.positional
        # Whether it is a value-optional flag
        opts["optional"] = name in self.optional
        # Whether it should be of an iterable (list) kind
        if name in self.iterable:
            opts["kind"] = list
            # If user gave a non-None default, hopefully they know better
            # than us what they want here (and hopefully it offers the list
            # protocol...) - otherwise supply useful default
            opts["default"] = default if default is not None else []
        # Whether it should increment its value or not
        if name in self.incrementable:
            opts["incrementable"] = True
        # Argument name(s) (replace w/ dashed version if underscores present,
        # and move the underscored version to be the attr_name instead.)
        if "_" in name:
            opts["attr_name"] = name
            name = translate_underscores(name)
        names = [name]
        if self.auto_shortflags:
            # Must know what short names are available
            for char in name:
                if not (char == name or char in taken_names):
                    names.append(char)
                    break
        opts["names"] = names
        # Handle default value & kind if possible
        if default not in (None, NO_DEFAULT):
            # TODO: allow setting 'kind' explicitly.
            # NOTE: skip setting 'kind' if optional is True + type(default) is
            # bool; that results in a nonsensical Argument which gives the
            # parser grief in a few ways.
            kind = type(default)
            if not (opts["optional"] and kind is bool):
                opts["kind"] = kind
            opts["default"] = default
        # Help
        if name in self.help:
            opts["help"] = self.help[name]
        return opts

    def get_arguments(self):
        """
        Return a list of Argument objects representing this task's signature.

        .. versionadded:: 1.0
        """
        # Core argspec
        arg_names, spec_dict = self.argspec(self.body)
        # Obtain list of args + their default values (if any) in
        # declaration/definition order (i.e. based on getargspec())
        tuples = [(x, spec_dict[x]) for x in arg_names]
        # Prime the list of all already-taken names (mostly for help in
        # choosing auto shortflags)
        taken_names = {x[0] for x in tuples}
        # Build arg list (arg_opts will take care of setting up shortnames,
        # etc)
        args = []
        for name, default in tuples:
            new_arg = Argument(**self.arg_opts(name, default, taken_names))
            args.append(new_arg)
            # Update taken_names list with new argument's full name list
            # (which may include new shortflags) so subsequent Argument
            # creation knows what's taken.
            taken_names.update(set(new_arg.names))
        # Now we need to ensure positionals end up in the front of the list, in
        # order given in self.positionals, so that when Context consumes them,
        # this order is preserved.
        for posarg in reversed(self.positional):
            for i, arg in enumerate(args):
                if arg.name == posarg:
                    args.insert(0, args.pop(i))
                    break
        return args


def task(*args, **kwargs):
    """
    Marks wrapped callable object as a valid Invoke task.

    May be called without any parentheses if no extra options need to be
    specified. Otherwise, the following keyword arguments are allowed in the
    parenthese'd form:

    * ``name``: Default name to use when binding to a `.Collection`. Useful for
      avoiding Python namespace issues (i.e. when the desired CLI level name
      can't or shouldn't be used as the Python level name.)
    * ``aliases``: Specify one or more aliases for this task, allowing it to be
      invoked as multiple different names. For example, a task named ``mytask``
      with a simple ``@task`` wrapper may only be invoked as ``"mytask"``.
      Changing the decorator to be ``@task(aliases=['myothertask'])`` allows
      invocation as ``"mytask"`` *or* ``"myothertask"``.
    * ``positional``: Iterable overriding the parser's automatic "args with no
      default value are considered positional" behavior. If a list of arg
      names, no args besides those named in this iterable will be considered
      positional. (This means that an empty list will force all arguments to be
      given as explicit flags.)
    * ``optional``: Iterable of argument names, declaring those args to
      have :ref:`optional values <optional-values>`. Such arguments may be
      given as value-taking options (e.g. ``--my-arg=myvalue``, wherein the
      task is given ``"myvalue"``) or as Boolean flags (``--my-arg``, resulting
      in ``True``).
    * ``iterable``: Iterable of argument names, declaring them to :ref:`build
      iterable values <iterable-flag-values>`.
    * ``incrementable``: Iterable of argument names, declaring them to
      :ref:`increment their values <incrementable-flag-values>`.
    * ``default``: Boolean option specifying whether this task should be its
      collection's default task (i.e. called if the collection's own name is
      given.)
    * ``auto_shortflags``: Whether or not to automatically create short
      flags from task options; defaults to True.
    * ``help``: Dict mapping argument names to their help strings. Will be
      displayed in ``--help`` output.
    * ``pre``, ``post``: Lists of task objects to execute prior to, or after,
      the wrapped task whenever it is executed.
    * ``autoprint``: Boolean determining whether to automatically print this
      task's return value to standard output when invoked directly via the CLI.
      Defaults to False.
    * ``klass``: Class to instantiate/return. Defaults to `.Task`.

    If any non-keyword arguments are given, they are taken as the value of the
    ``pre`` kwarg for convenience's sake. (It is an error to give both
    ``*args`` and ``pre`` at the same time.)

    .. versionadded:: 1.0
    .. versionchanged:: 1.1
        Added the ``klass`` keyword argument.
    """
    klass = kwargs.pop("klass", Task)
    # @task -- no options were (probably) given.
    if len(args) == 1 and callable(args[0]) and not isinstance(args[0], Task):
        return klass(args[0], **kwargs)
    # @task(pre, tasks, here)
    if args:
        if "pre" in kwargs:
            raise TypeError(
                "May not give *args and 'pre' kwarg simultaneously!"
            )
        kwargs["pre"] = args
    # @task(options)
    # TODO: why the heck did we originally do this in this manner instead of
    # simply delegating to Task?! Let's just remove all this sometime & see
    # what, if anything, breaks.
    name = kwargs.pop("name", None)
    aliases = kwargs.pop("aliases", ())
    positional = kwargs.pop("positional", None)
    optional = tuple(kwargs.pop("optional", ()))
    iterable = kwargs.pop("iterable", None)
    incrementable = kwargs.pop("incrementable", None)
    default = kwargs.pop("default", False)
    auto_shortflags = kwargs.pop("auto_shortflags", True)
    help = kwargs.pop("help", {})
    pre = kwargs.pop("pre", [])
    post = kwargs.pop("post", [])
    autoprint = kwargs.pop("autoprint", False)

    def inner(obj):
        obj = klass(
            obj,
            name=name,
            aliases=aliases,
            positional=positional,
            optional=optional,
            iterable=iterable,
            incrementable=incrementable,
            default=default,
            auto_shortflags=auto_shortflags,
            help=help,
            pre=pre,
            post=post,
            autoprint=autoprint,
            # Pass in any remaining kwargs as-is.
            **kwargs
        )
        return obj

    return inner


class Call(object):
    """
    Represents a call/execution of a `.Task` with given (kw)args.

    Similar to `~functools.partial` with some added functionality (such as the
    delegation to the inner task, and optional tracking of the name it's being
    called by.)

    .. versionadded:: 1.0
    """

    def __init__(self, task, called_as=None, args=None, kwargs=None):
        """
        Create a new `.Call` object.

        :param task: The `.Task` object to be executed.

        :param str called_as:
            The name the task is being called as, e.g. if it was called by an
            alias or other rebinding. Defaults to ``None``, aka, the task was
            referred to by its default name.

        :param tuple args:
            Positional arguments to call with, if any. Default: ``None``.

        :param dict kwargs:
            Keyword arguments to call with, if any. Default: ``None``.
        """
        self.task = task
        self.called_as = called_as
        self.args = args or tuple()
        self.kwargs = kwargs or dict()

    # TODO: just how useful is this? feels like maybe overkill magic
    def __getattr__(self, name):
        return getattr(self.task, name)

    def __deepcopy__(self, memo):
        return self.clone()

    def __repr__(self):
        aka = ""
        if self.called_as is not None and self.called_as != self.task.name:
            aka = " (called as: {!r})".format(self.called_as)
        return "<{} {!r}{}, args: {!r}, kwargs: {!r}>".format(
            self.__class__.__name__,
            self.task.name,
            aka,
            self.args,
            self.kwargs,
        )

    def __eq__(self, other):
        # NOTE: Not comparing 'called_as'; a named call of a given Task with
        # same args/kwargs should be considered same as an unnamed call of the
        # same Task with the same args/kwargs (e.g. pre/post task specified w/o
        # name). Ditto tasks with multiple aliases.
        for attr in "task args kwargs".split():
            if getattr(self, attr) != getattr(other, attr):
                return False
        return True

    def make_context(self, config):
        """
        Generate a `.Context` appropriate for this call, with given config.

        .. versionadded:: 1.0
        """
        return Context(config=config)

    def clone_data(self):
        """
        Return keyword args suitable for cloning this call into another.

        .. versionadded:: 1.1
        """
        return dict(
            task=self.task,
            called_as=self.called_as,
            args=deepcopy(self.args),
            kwargs=deepcopy(self.kwargs),
        )

    def clone(self, into=None, with_=None):
        """
        Return a standalone copy of this Call.

        Useful when parameterizing task executions.

        :param into:
            A subclass to generate instead of the current class. Optional.

        :param dict with_:
            A dict of additional keyword arguments to use when creating the new
            clone; typically used when cloning ``into`` a subclass that has
            extra args on top of the base class. Optional.

            .. note::
                This dict is used to ``.update()`` the original object's data
                (the return value from its `clone_data`), so in the event of
                a conflict, values in ``with_`` will win out.

        .. versionadded:: 1.0
        .. versionchanged:: 1.1
            Added the ``with_`` kwarg.
        """
        klass = into if into is not None else self.__class__
        data = self.clone_data()
        if with_ is not None:
            data.update(with_)
        return klass(**data)


def call(task, *args, **kwargs):
    """
    Describes execution of a `.Task`, typically with pre-supplied arguments.

    Useful for setting up :ref:`pre/post task invocations
    <parameterizing-pre-post-tasks>`. It's actually just a convenient wrapper
    around the `.Call` class, which may be used directly instead if desired.

    For example, here's two build-like tasks that both refer to a ``setup``
    pre-task, one with no baked-in argument values (and thus no need to use
    `.call`), and one that toggles a boolean flag::

        @task
        def setup(c, clean=False):
            if clean:
                c.run("rm -rf target")
            # ... setup things here ...
            c.run("tar czvf target.tgz target")

        @task(pre=[setup])
        def build(c):
            c.run("build, accounting for leftover files...")

        @task(pre=[call(setup, clean=True)])
        def clean_build(c):
            c.run("build, assuming clean slate...")

    Please see the constructor docs for `.Call` for details - this function's
    ``args`` and ``kwargs`` map directly to the same arguments as in that
    method.

    .. versionadded:: 1.0
    """
    return Call(task=task, args=args, kwargs=kwargs)
