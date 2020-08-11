"""
Utility functions surrounding terminal devices & I/O.

Much of this code performs platform-sensitive branching, e.g. Windows support.

This is its own module to abstract away what would otherwise be distracting
logic-flow interruptions.
"""

from contextlib import contextmanager
import os
import select
import sys

# TODO: move in here? They're currently platform-agnostic...
from .util import has_fileno, isatty


WINDOWS = sys.platform == "win32"
"""
Whether or not the current platform appears to be Windows in nature.

Note that Cygwin's Python is actually close enough to "real" UNIXes that it
doesn't need (or want!) to use PyWin32 -- so we only test for literal Win32
setups (vanilla Python, ActiveState etc) here.

.. versionadded:: 1.0
"""

if WINDOWS:
    import msvcrt
    from ctypes import Structure, c_ushort, windll, POINTER, byref
    from ctypes.wintypes import HANDLE, _COORD, _SMALL_RECT
else:
    import fcntl
    import struct
    import termios
    import tty


def pty_size():
    """
    Determine current local pseudoterminal dimensions.

    :returns:
        A ``(num_cols, num_rows)`` two-tuple describing PTY size. Defaults to
        ``(80, 24)`` if unable to get a sensible result dynamically.

    .. versionadded:: 1.0
    """
    cols, rows = _pty_size() if not WINDOWS else _win_pty_size()
    # TODO: make defaults configurable?
    return ((cols or 80), (rows or 24))


def _pty_size():
    """
    Suitable for most POSIX platforms.

    .. versionadded:: 1.0
    """
    # Sentinel values to be replaced w/ defaults by caller
    size = (None, None)
    # We want two short unsigned integers (rows, cols)
    fmt = "HH"
    # Create an empty (zeroed) buffer for ioctl to map onto. Yay for C!
    buf = struct.pack(fmt, 0, 0)
    # Call TIOCGWINSZ to get window size of stdout, returns our filled
    # buffer
    try:
        result = fcntl.ioctl(sys.stdout, termios.TIOCGWINSZ, buf)
        # Unpack buffer back into Python data types
        # NOTE: this unpack gives us rows x cols, but we return the
        # inverse.
        rows, cols = struct.unpack(fmt, result)
        return (cols, rows)
    # Fallback to emptyish return value in various failure cases:
    # * sys.stdout being monkeypatched, such as in testing, and lacking .fileno
    # * sys.stdout having a .fileno but not actually being attached to a TTY
    # * termios not having a TIOCGWINSZ attribute (happens sometimes...)
    # * other situations where ioctl doesn't explode but the result isn't
    #   something unpack can deal with
    except (struct.error, TypeError, IOError, AttributeError):
        pass
    return size


def _win_pty_size():
    class CONSOLE_SCREEN_BUFFER_INFO(Structure):
        _fields_ = [
            ("dwSize", _COORD),
            ("dwCursorPosition", _COORD),
            ("wAttributes", c_ushort),
            ("srWindow", _SMALL_RECT),
            ("dwMaximumWindowSize", _COORD),
        ]

    GetStdHandle = windll.kernel32.GetStdHandle
    GetConsoleScreenBufferInfo = windll.kernel32.GetConsoleScreenBufferInfo
    GetStdHandle.restype = HANDLE
    GetConsoleScreenBufferInfo.argtypes = [
        HANDLE,
        POINTER(CONSOLE_SCREEN_BUFFER_INFO),
    ]

    hstd = GetStdHandle(-11)  # STD_OUTPUT_HANDLE = -11
    csbi = CONSOLE_SCREEN_BUFFER_INFO()
    ret = GetConsoleScreenBufferInfo(hstd, byref(csbi))

    if ret:
        sizex = csbi.srWindow.Right - csbi.srWindow.Left + 1
        sizey = csbi.srWindow.Bottom - csbi.srWindow.Top + 1
        return sizex, sizey
    else:
        return (None, None)


def stdin_is_foregrounded_tty(stream):
    """
    Detect if given stdin ``stream`` seems to be in the foreground of a TTY.

    Specifically, compares the current Python process group ID to that of the
    stream's file descriptor to see if they match; if they do not match, it is
    likely that the process has been placed in the background.

    This is used as a test to determine whether we should manipulate an active
    stdin so it runs in a character-buffered mode; touching the terminal in
    this way when the process is backgrounded, causes most shells to pause
    execution.

    .. note::
        Processes that aren't attached to a terminal to begin with, will always
        fail this test, as it starts with "do you have a real ``fileno``?".

    .. versionadded:: 1.0
    """
    if not has_fileno(stream):
        return False
    return os.getpgrp() == os.tcgetpgrp(stream.fileno())


def cbreak_already_set(stream):
    # Explicitly not docstringed to remain private, for now. Eh.
    # Checks whether tty.setcbreak appears to have already been run against
    # ``stream`` (or if it would otherwise just not do anything).
    # Used to effect idempotency for character-buffering a stream, which also
    # lets us avoid multiple capture-then-restore cycles.
    attrs = termios.tcgetattr(stream)
    lflags, cc = attrs[3], attrs[6]
    echo = bool(lflags & termios.ECHO)
    icanon = bool(lflags & termios.ICANON)
    # setcbreak sets ECHO and ICANON to 0/off, CC[VMIN] to 1-ish, and CC[VTIME]
    # to 0-ish. If any of that is not true we can reasonably assume it has not
    # yet been executed against this stream.
    sentinels = (
        not echo,
        not icanon,
        cc[termios.VMIN] in [1, b"\x01"],
        cc[termios.VTIME] in [0, b"\x00"],
    )
    return all(sentinels)


@contextmanager
def character_buffered(stream):
    """
    Force local terminal ``stream`` be character, not line, buffered.

    Only applies to Unix-based systems; on Windows this is a no-op.

    .. versionadded:: 1.0
    """
    if (
        WINDOWS
        or not isatty(stream)
        or not stdin_is_foregrounded_tty(stream)
        or cbreak_already_set(stream)
    ):
        yield
    else:
        old_settings = termios.tcgetattr(stream)
        tty.setcbreak(stream)
        try:
            yield
        finally:
            termios.tcsetattr(stream, termios.TCSADRAIN, old_settings)


def ready_for_reading(input_):
    """
    Test ``input_`` to determine whether a read action will succeed.

    :param input_: Input stream object (file-like).

    :returns: ``True`` if a read should succeed, ``False`` otherwise.

    .. versionadded:: 1.0
    """
    # A "real" terminal stdin needs select/kbhit to tell us when it's ready for
    # a nonblocking read().
    # Otherwise, assume a "safer" file-like object that can be read from in a
    # nonblocking fashion (e.g. a StringIO or regular file).
    if not has_fileno(input_):
        return True
    if WINDOWS:
        return msvcrt.kbhit()
    else:
        reads, _, _ = select.select([input_], [], [], 0.0)
        return bool(reads and reads[0] is input_)


def bytes_to_read(input_):
    """
    Query stream ``input_`` to see how many bytes may be readable.

    .. note::
        If we are unable to tell (e.g. if ``input_`` isn't a true file
        descriptor or isn't a valid TTY) we fall back to suggesting reading 1
        byte only.

    :param input: Input stream object (file-like).

    :returns: `int` number of bytes to read.

    .. versionadded:: 1.0
    """
    # NOTE: we have to check both possibilities here; situations exist where
    # it's not a tty but has a fileno, or vice versa; neither is typically
    # going to work re: ioctl().
    if not WINDOWS and isatty(input_) and has_fileno(input_):
        fionread = fcntl.ioctl(input_, termios.FIONREAD, "  ")
        return struct.unpack("h", fionread)[0]
    return 1
