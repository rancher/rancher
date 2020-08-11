"""
Custom exception classes.

These vary in use case from "we needed a specific data structure layout in
exceptions used for message-passing" to simply "we needed to express an error
condition in a way easily told apart from other, truly unexpected errors".
"""

from traceback import format_exception
from pprint import pformat

from .util import six

from .util import encode_output


class CollectionNotFound(Exception):
    def __init__(self, name, start):
        self.name = name
        self.start = start


class Failure(Exception):
    """
    Exception subclass representing failure of a command execution.

    "Failure" may mean the command executed and the shell indicated an unusual
    result (usually, a non-zero exit code), or it may mean something else, like
    a ``sudo`` command which was aborted when the supplied password failed
    authentication.

    Two attributes allow introspection to determine the nature of the problem:

    * ``result``: a `.Result` instance with info about the command being
      executed and, if it ran to completion, how it exited.
    * ``reason``: ``None``, if the command finished; or an exception instance
      if e.g. a `.StreamWatcher` raised `WatcherError`.

    This class is only rarely raised by itself; most of the time `.Runner.run`
    (or a wrapper of same, such as `.Context.sudo`) will raise a specific
    subclass like `UnexpectedExit` or `AuthFailure`.

    .. versionadded:: 1.0
    """

    def __init__(self, result, reason=None):
        self.result = result
        self.reason = reason


def _tail(stream):
    # TODO: make configurable
    # TODO: preserve alternate line endings? Mehhhh
    tail = "\n\n" + "\n".join(stream.splitlines()[-10:])
    # NOTE: no trailing \n preservation; easier for below display if normalized
    return tail


class UnexpectedExit(Failure):
    """
    A shell command ran to completion but exited with an unexpected exit code.

    Its string representation displays the following:

    - Command executed;
    - Exit code;
    - The last 10 lines of stdout, if it was hidden;
    - The last 10 lines of stderr, if it was hidden and non-empty (e.g.
      pty=False; when pty=True, stderr never happens.)

    .. versionadded:: 1.0
    """

    def __str__(self):
        already_printed = " already printed"
        if "stdout" not in self.result.hide:
            stdout = already_printed
        else:
            stdout = encode_output(
                _tail(self.result.stdout), self.result.encoding
            )
        if self.result.pty:
            stderr = " n/a (PTYs have no stderr)"
        else:
            if "stderr" not in self.result.hide:
                stderr = already_printed
            else:
                stderr = encode_output(
                    _tail(self.result.stderr), self.result.encoding
                )
        command = self.result.command
        exited = self.result.exited
        template = """Encountered a bad command exit code!

Command: {!r}

Exit code: {}

Stdout:{}

Stderr:{}

"""
        return template.format(command, exited, stdout, stderr)

    def __repr__(self):
        # TODO: expand?
        template = "<{}: cmd={!r} exited={}>"
        return template.format(
            self.__class__.__name__, self.result.command, self.result.exited
        )


class AuthFailure(Failure):
    """
    An authentication failure, e.g. due to an incorrect ``sudo`` password.

    .. note::
        `.Result` objects attached to these exceptions typically lack exit code
        information, since the command was never fully executed - the exception
        was raised instead.

    .. versionadded:: 1.0
    """

    def __init__(self, result, prompt):
        self.result = result
        self.prompt = prompt

    def __str__(self):
        err = "The password submitted to prompt {!r} was rejected."
        return err.format(self.prompt)


class ParseError(Exception):
    """
    An error arising from the parsing of command-line flags/arguments.

    Ambiguous input, invalid task names, invalid flags, etc.

    .. versionadded:: 1.0
    """

    def __init__(self, msg, context=None):
        super(ParseError, self).__init__(msg)
        self.context = context


class Exit(Exception):
    """
    Simple custom stand-in for SystemExit.

    Replaces scattered sys.exit calls, improves testability, allows one to
    catch an exit request without intercepting real SystemExits (typically an
    unfriendly thing to do, as most users calling `sys.exit` rather expect it
    to truly exit.)

    Defaults to a non-printing, exit-0 friendly termination behavior if the
    exception is uncaught.

    If ``code`` (an int) given, that code is used to exit.

    If ``message`` (a string) given, it is printed to standard error, and the
    program exits with code ``1`` by default (unless overridden by also giving
    ``code`` explicitly.)

    .. versionadded:: 1.0
    """

    def __init__(self, message=None, code=None):
        self.message = message
        self._code = code

    @property
    def code(self):
        if self._code is not None:
            return self._code
        return 1 if self.message else 0


class PlatformError(Exception):
    """
    Raised when an illegal operation occurs for the current platform.

    E.g. Windows users trying to use functionality requiring the ``pty``
    module.

    Typically used to present a clearer error message to the user.

    .. versionadded:: 1.0
    """

    pass


class AmbiguousEnvVar(Exception):
    """
    Raised when loading env var config keys has an ambiguous target.

    .. versionadded:: 1.0
    """

    pass


class UncastableEnvVar(Exception):
    """
    Raised on attempted env var loads whose default values are too rich.

    E.g. trying to stuff ``MY_VAR="foo"`` into ``{'my_var': ['uh', 'oh']}``
    doesn't make any sense until/if we implement some sort of transform option.

    .. versionadded:: 1.0
    """

    pass


class UnknownFileType(Exception):
    """
    A config file of an unknown type was specified and cannot be loaded.

    .. versionadded:: 1.0
    """

    pass


class UnpicklableConfigMember(Exception):
    """
    A config file contained module objects, which can't be pickled/copied.

    We raise this more easily catchable exception instead of letting the
    (unclearly phrased) TypeError bubble out of the pickle module. (However, to
    avoid our own fragile catching of that error, we head it off by explicitly
    testing for module members.)

    .. versionadded:: 1.0.2
    """

    pass


def _printable_kwargs(kwargs):
    """
    Return print-friendly version of a thread-related ``kwargs`` dict.

    Extra care is taken with ``args`` members which are very long iterables -
    those need truncating to be useful.
    """
    printable = {}
    for key, value in six.iteritems(kwargs):
        item = value
        if key == "args":
            item = []
            for arg in value:
                new_arg = arg
                if hasattr(arg, "__len__") and len(arg) > 10:
                    msg = "<... remainder truncated during error display ...>"
                    new_arg = arg[:10] + [msg]
                item.append(new_arg)
        printable[key] = item
    return printable


class ThreadException(Exception):
    """
    One or more exceptions were raised within background threads.

    The real underlying exceptions are stored in the `exceptions` attribute;
    see its documentation for data structure details.

    .. note::
        Threads which did not encounter an exception, do not contribute to this
        exception object and thus are not present inside `exceptions`.

    .. versionadded:: 1.0
    """

    #: A tuple of `ExceptionWrappers <invoke.util.ExceptionWrapper>` containing
    #: the initial thread constructor kwargs (because `threading.Thread`
    #: subclasses should always be called with kwargs) and the caught exception
    #: for that thread as seen by `sys.exc_info` (so: type, value, traceback).
    #:
    #: .. note::
    #:     The ordering of this attribute is not well-defined.
    #:
    #: .. note::
    #:     Thread kwargs which appear to be very long (e.g. IO
    #:     buffers) will be truncated when printed, to avoid huge
    #:     unreadable error display.
    exceptions = tuple()

    def __init__(self, exceptions):
        self.exceptions = tuple(exceptions)

    def __str__(self):
        details = []
        for x in self.exceptions:
            # Build useful display
            detail = "Thread args: {}\n\n{}"
            details.append(
                detail.format(
                    pformat(_printable_kwargs(x.kwargs)),
                    "\n".join(format_exception(x.type, x.value, x.traceback)),
                )
            )
        args = (
            len(self.exceptions),
            ", ".join(x.type.__name__ for x in self.exceptions),
            "\n\n".join(details),
        )
        return """
Saw {} exceptions within threads ({}):


{}
""".format(
            *args
        )


class WatcherError(Exception):
    """
    Generic parent exception class for `.StreamWatcher`-related errors.

    Typically, one of these exceptions indicates a `.StreamWatcher` noticed
    something anomalous in an output stream, such as an authentication response
    failure.

    `.Runner` catches these and attaches them to `.Failure` exceptions so they
    can be referenced by intermediate code and/or act as extra info for end
    users.

    .. versionadded:: 1.0
    """

    pass


class ResponseNotAccepted(WatcherError):
    """
    A responder/watcher class noticed a 'bad' response to its submission.

    Mostly used by `.FailingResponder` and subclasses, e.g. "oh dear I
    autosubmitted a sudo password and it was incorrect."

    .. versionadded:: 1.0
    """

    pass
