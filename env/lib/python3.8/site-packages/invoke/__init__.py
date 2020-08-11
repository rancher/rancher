from ._version import __version_info__, __version__  # noqa
from .collection import Collection  # noqa
from .config import Config  # noqa
from .context import Context, MockContext  # noqa
from .exceptions import (  # noqa
    AmbiguousEnvVar,
    AuthFailure,
    CollectionNotFound,
    Exit,
    ParseError,
    PlatformError,
    ResponseNotAccepted,
    ThreadException,
    UncastableEnvVar,
    UnexpectedExit,
    UnknownFileType,
    UnpicklableConfigMember,
    WatcherError,
)
from .executor import Executor  # noqa
from .loader import FilesystemLoader  # noqa
from .parser import Argument  # noqa
from .program import Program  # noqa
from .runners import Runner, Local, Failure, Result  # noqa
from .tasks import task, call, Call, Task  # noqa
from .terminals import pty_size  # noqa
from .watchers import FailingResponder, Responder, StreamWatcher  # noqa


def run(command, **kwargs):
    """
    Run ``command`` in a local subprocess and return a `.Result` object.

    See `.Runner.run` for API details.

    .. note::
        This function is a convenience wrapper around Invoke's `.Context` and
        `.Runner` APIs.

        Specifically, it creates an anonymous `.Context` instance and calls its
        `~.Context.run` method, which in turn defaults to using a `.Local`
        runner subclass for command execution.

    .. versionadded:: 1.0
    """
    return Context().run(command, **kwargs)
