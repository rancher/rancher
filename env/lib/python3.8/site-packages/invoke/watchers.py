import re
import threading

from .exceptions import ResponseNotAccepted


class StreamWatcher(threading.local):
    """
    A class whose subclasses may act on seen stream data from subprocesses.

    Subclasses must exhibit the following API; see `Responder` for a concrete
    example.

    * ``__init__`` is completely up to each subclass, though as usual,
      subclasses *of* subclasses should be careful to make use of `super` where
      appropriate.
    * `submit` must accept the entire current contents of the stream being
      watched, as a Unicode string, and may optionally return an iterable of
      Unicode strings (or act as a generator iterator, i.e. multiple calls to
      ``yield <unicode string>``), which will each be written to the
      subprocess' standard input.

    .. note::
        `StreamWatcher` subclasses exist in part to enable state tracking, such
        as detecting when a submitted password didn't work & erroring (or
        prompting a user, or etc). Such bookkeeping isn't easily achievable
        with simple callback functions.

    .. note::
        `StreamWatcher` subclasses `threading.local` so that its instances can
        be used to 'watch' both subprocess stdout and stderr in separate
        threads.

    .. versionadded:: 1.0
    """

    def submit(self, stream):
        """
        Act on ``stream`` data, potentially returning responses.

        :param unicode stream:
            All data read on this stream since the beginning of the session.

        :returns:
            An iterable of Unicode strings (which may be empty).

        .. versionadded:: 1.0
        """
        raise NotImplementedError


class Responder(StreamWatcher):
    """
    A parameterizable object that submits responses to specific patterns.

    Commonly used to implement password auto-responds for things like ``sudo``.

    .. versionadded:: 1.0
    """

    def __init__(self, pattern, response):
        """
        Imprint this `Responder` with necessary parameters.

        :param pattern:
            A raw string (e.g. ``r"\[sudo\] password for .*:"``) which will be
            turned into a regular expression.

        :param response:
            The string to submit to the subprocess' stdin when ``pattern`` is
            detected.
        """
        # TODO: precompile the keys into regex objects
        self.pattern = pattern
        self.response = response
        self.index = 0

    def pattern_matches(self, stream, pattern, index_attr):
        """
        Generic "search for pattern in stream, using index" behavior.

        Used here and in some subclasses that want to track multiple patterns
        concurrently.

        :param unicode stream: The same data passed to ``submit``.
        :param unicode pattern: The pattern to search for.
        :param unicode index_attr: The name of the index attribute to use.
        :returns: An iterable of string matches.

        .. versionadded:: 1.0
        """
        # NOTE: generifies scanning so it can be used to scan for >1 pattern at
        # once, e.g. in FailingResponder.
        # Only look at stream contents we haven't seen yet, to avoid dupes.
        index = getattr(self, index_attr)
        new_ = stream[index:]
        # Search, across lines if necessary
        matches = re.findall(pattern, new_, re.S)
        # Update seek index if we've matched
        if matches:
            setattr(self, index_attr, index + len(new_))
        return matches

    def submit(self, stream):
        # Iterate over findall() response in case >1 match occurred.
        for _ in self.pattern_matches(stream, self.pattern, "index"):
            yield self.response


class FailingResponder(Responder):
    """
    Variant of `Responder` which is capable of detecting incorrect responses.

    This class adds a ``sentinel`` parameter to ``__init__``, and its
    ``submit`` will raise `.ResponseNotAccepted` if it detects that sentinel
    value in the stream.

    .. versionadded:: 1.0
    """

    def __init__(self, pattern, response, sentinel):
        super(FailingResponder, self).__init__(pattern, response)
        self.sentinel = sentinel
        self.failure_index = 0
        self.tried = False

    def submit(self, stream):
        # Behave like regular Responder initially
        response = super(FailingResponder, self).submit(stream)
        # Also check stream for our failure sentinel
        failed = self.pattern_matches(stream, self.sentinel, "failure_index")
        # Error out if we seem to have failed after a previous response.
        if self.tried and failed:
            err = 'Auto-response to r"{}" failed with {!r}!'.format(
                self.pattern, self.sentinel
            )
            raise ResponseNotAccepted(err)
        # Once we see that we had a response, take note
        if response:
            self.tried = True
        # Again, behave regularly by default.
        return response
