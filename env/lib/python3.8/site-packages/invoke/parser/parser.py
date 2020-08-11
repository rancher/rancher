import copy

try:
    from ..vendor.lexicon import Lexicon
    from ..vendor.fluidity import StateMachine, state, transition
except ImportError:
    from lexicon import Lexicon
    from fluidity import StateMachine, state, transition

from ..util import debug
from ..exceptions import ParseError


def is_flag(value):
    return value.startswith("-")


def is_long_flag(value):
    return value.startswith("--")


class Parser(object):
    """
    Create parser conscious of ``contexts`` and optional ``initial`` context.

    ``contexts`` should be an iterable of ``Context`` instances which will be
    searched when new context names are encountered during a parse. These
    Contexts determine what flags may follow them, as well as whether given
    flags take values.

    ``initial`` is optional and will be used to determine validity of "core"
    options/flags at the start of the parse run, if any are encountered.

    ``ignore_unknown`` determines what to do when contexts are found which do
    not map to any members of ``contexts``. By default it is ``False``, meaning
    any unknown contexts result in a parse error exception. If ``True``,
    encountering an unknown context halts parsing and populates the return
    value's ``.unparsed`` attribute with the remaining parse tokens.

    .. versionadded:: 1.0
    """

    def __init__(self, contexts=(), initial=None, ignore_unknown=False):
        self.initial = initial
        self.contexts = Lexicon()
        self.ignore_unknown = ignore_unknown
        for context in contexts:
            debug("Adding {}".format(context))
            if not context.name:
                raise ValueError("Non-initial contexts must have names.")
            exists = "A context named/aliased {!r} is already in this parser!"
            if context.name in self.contexts:
                raise ValueError(exists.format(context.name))
            self.contexts[context.name] = context
            for alias in context.aliases:
                if alias in self.contexts:
                    raise ValueError(exists.format(alias))
                self.contexts.alias(alias, to=context.name)

    def parse_argv(self, argv):
        """
        Parse an argv-style token list ``argv``.

        Returns a list (actually a subclass, `.ParseResult`) of
        `.ParserContext` objects matching the order they were found in the
        ``argv`` and containing `.Argument` objects with updated values based
        on any flags given.

        Assumes any program name has already been stripped out. Good::

            Parser(...).parse_argv(['--core-opt', 'task', '--task-opt'])

        Bad::

            Parser(...).parse_argv(['invoke', '--core-opt', ...])

        :param argv: List of argument string tokens.
        :returns: A `.ParserContext` (``list`` subclass).

        .. versionadded:: 1.0
        """
        machine = ParseMachine(
            initial=self.initial,
            contexts=self.contexts,
            ignore_unknown=self.ignore_unknown,
        )
        # FIXME: Why isn't there str.partition for lists? There must be a
        # better way to do this. Split argv around the double-dash remainder
        # sentinel.
        debug("Starting argv: {!r}".format(argv))
        try:
            ddash = argv.index("--")
        except ValueError:
            ddash = len(argv)  # No remainder == body gets all
        body = argv[:ddash]
        remainder = argv[ddash:][1:]  # [1:] to strip off remainder itself
        if remainder:
            debug(
                "Remainder: argv[{!r}:][1:] => {!r}".format(ddash, remainder)
            )
        for index, token in enumerate(body):
            # Handle non-space-delimited forms, if not currently expecting a
            # flag value and still in valid parsing territory (i.e. not in
            # "unknown" state which implies store-only)
            # NOTE: we do this in a few steps so we can
            # split-then-check-validity; necessary for things like when the
            # previously seen flag optionally takes a value.
            mutations = []
            orig = token
            if is_flag(token) and not machine.result.unparsed:
                # Equals-sign-delimited flags, eg --foo=bar or -f=bar
                if "=" in token:
                    token, _, value = token.partition("=")
                    msg = "Splitting x=y expr {!r} into tokens {!r} and {!r}"
                    debug(msg.format(orig, token, value))
                    mutations.append((index + 1, value))
                # Contiguous boolean short flags, e.g. -qv
                elif not is_long_flag(token) and len(token) > 2:
                    full_token = token[:]
                    rest, token = token[2:], token[:2]
                    err = "Splitting {!r} into token {!r} and rest {!r}"
                    debug(err.format(full_token, token, rest))
                    # Handle boolean flag block vs short-flag + value. Make
                    # sure not to test the token as a context flag if we've
                    # passed into 'storing unknown stuff' territory (e.g. on a
                    # core-args pass, handling what are going to be task args)
                    have_flag = (
                        token in machine.context.flags
                        and machine.current_state != "unknown"
                    )
                    if have_flag and machine.context.flags[token].takes_value:
                        msg = "{!r} is a flag for current context & it takes a value, giving it {!r}"  # noqa
                        debug(msg.format(token, rest))
                        mutations.append((index + 1, rest))
                    else:
                        rest = ["-{}".format(x) for x in rest]
                        msg = (
                            "Splitting multi-flag glob {!r} into {!r} and {!r}"
                        )  # noqa
                        debug(msg.format(orig, token, rest))
                        for item in reversed(rest):
                            mutations.append((index + 1, item))
            # Here, we've got some possible mutations queued up, and 'token'
            # may have been overwritten as well. Whether we apply those and
            # continue as-is, or roll it back, depends:
            # - If the parser wasn't waiting for a flag value, we're already on
            # the right track, so apply mutations and move along to the
            # handle() step.
            # - If we ARE waiting for a value, and the flag expecting it ALWAYS
            # wants a value (it's not optional), we go back to using the
            # original token. (TODO: could reorganize this to avoid the
            # sub-parsing in this case, but optimizing for human-facing
            # execution isn't critical.)
            # - Finally, if we are waiting for a value AND it's optional, we
            # inspect the first sub-token/mutation to see if it would otherwise
            # have been a valid flag, and let that determine what we do (if
            # valid, we apply the mutations; if invalid, we reinstate the
            # original token.)
            if machine.waiting_for_flag_value:
                optional = machine.flag and machine.flag.optional
                subtoken_is_valid_flag = token in machine.context.flags
                if not (optional and subtoken_is_valid_flag):
                    token = orig
                    mutations = []
            for index, value in mutations:
                body.insert(index, value)
            machine.handle(token)
        machine.finish()
        result = machine.result
        result.remainder = " ".join(remainder)
        return result


class ParseMachine(StateMachine):
    initial_state = "context"

    state("context", enter=["complete_flag", "complete_context"])
    state("unknown", enter=["complete_flag", "complete_context"])
    state("end", enter=["complete_flag", "complete_context"])

    transition(from_=("context", "unknown"), event="finish", to="end")
    transition(
        from_="context",
        event="see_context",
        action="switch_to_context",
        to="context",
    )
    transition(
        from_=("context", "unknown"),
        event="see_unknown",
        action="store_only",
        to="unknown",
    )

    def changing_state(self, from_, to):
        debug("ParseMachine: {!r} => {!r}".format(from_, to))

    def __init__(self, initial, contexts, ignore_unknown):
        # Initialize
        self.ignore_unknown = ignore_unknown
        self.initial = self.context = copy.deepcopy(initial)
        debug("Initialized with context: {!r}".format(self.context))
        self.flag = None
        self.flag_got_value = False
        self.result = ParseResult()
        self.contexts = copy.deepcopy(contexts)
        debug("Available contexts: {!r}".format(self.contexts))
        # In case StateMachine does anything in __init__
        super(ParseMachine, self).__init__()

    @property
    def waiting_for_flag_value(self):
        # Do we have a current flag, and does it expect a value (vs being a
        # bool/toggle)?
        takes_value = self.flag and self.flag.takes_value
        if not takes_value:
            return False
        # OK, this flag is one that takes values.
        # Is it a list type (which has only just been switched to)? Then it'll
        # always accept more values.
        # TODO: how to handle somebody wanting it to be some other iterable
        # like tuple or custom class? Or do we just say unsupported?
        if self.flag.kind is list and not self.flag_got_value:
            return True
        # Not a list, okay. Does it already have a value?
        has_value = self.flag.raw_value is not None
        # If it doesn't have one, we're waiting for one (which tells the parser
        # how to proceed and typically to store the next token.)
        # TODO: in the negative case here, we should do something else instead:
        # - Except, "hey you screwed up, you already gave that flag!"
        # - Overwrite, "oh you changed your mind?" - which requires more work
        # elsewhere too, unfortunately. (Perhaps additional properties on
        # Argument that can be queried, e.g. "arg.is_iterable"?)
        return not has_value

    def handle(self, token):
        debug("Handling token: {!r}".format(token))
        # Handle unknown state at the top: we don't care about even
        # possibly-valid input if we've encountered unknown input.
        if self.current_state == "unknown":
            debug("Top-of-handle() see_unknown({!r})".format(token))
            self.see_unknown(token)
            return
        # Flag
        if self.context and token in self.context.flags:
            debug("Saw flag {!r}".format(token))
            self.switch_to_flag(token)
        elif self.context and token in self.context.inverse_flags:
            debug("Saw inverse flag {!r}".format(token))
            self.switch_to_flag(token, inverse=True)
        # Value for current flag
        elif self.waiting_for_flag_value:
            debug(
                "We're waiting for a flag value so {!r} must be it?".format(
                    token
                )
            )  # noqa
            self.see_value(token)
        # Positional args (must come above context-name check in case we still
        # need a posarg and the user legitimately wants to give it a value that
        # just happens to be a valid context name.)
        elif self.context and self.context.missing_positional_args:
            msg = "Context {!r} requires positional args, eating {!r}"
            debug(msg.format(self.context, token))
            self.see_positional_arg(token)
        # New context
        elif token in self.contexts:
            self.see_context(token)
        # Initial-context flag being given as per-task flag (e.g. --help)
        elif self.initial and token in self.initial.flags:
            debug("Saw (initial-context) flag {!r}".format(token))
            flag = self.initial.flags[token]
            # TODO: handle ambiguity? Right now, flags in the context that
            # shadow initial-context flags would always naturally "win" by
            # being higher up in this if/elsif/etc chain. Ideally we'd complain
            # to avoid users shooting themselves in the foot?
            # Flags of this type that take a value are always given the current
            # context's name as a string value.
            # TODO: document this in the parser docs
            if flag.takes_value:
                flag.value = self.context.name
            else:
                # TODO: handle inverse flags, other flag types?
                flag.value = True
            msg = "Setting (initial-context) flag {!r} to value {!r}"
            debug(msg.format(flag, flag.value))
        # Unknown
        else:
            if not self.ignore_unknown:
                debug("Can't find context named {!r}, erroring".format(token))
                self.error("No idea what {!r} is!".format(token))
            else:
                debug("Bottom-of-handle() see_unknown({!r})".format(token))
                self.see_unknown(token)

    def store_only(self, token):
        # Start off the unparsed list
        debug("Storing unknown token {!r}".format(token))
        self.result.unparsed.append(token)

    def complete_context(self):
        debug(
            "Wrapping up context {!r}".format(
                self.context.name if self.context else self.context
            )
        )
        # Ensure all of context's positional args have been given.
        if self.context and self.context.missing_positional_args:
            err = "'{}' did not receive required positional arguments: {}"
            names = ", ".join(
                "'{}'".format(x.name)
                for x in self.context.missing_positional_args
            )
            self.error(err.format(self.context.name, names))
        if self.context and self.context not in self.result:
            self.result.append(self.context)

    def switch_to_context(self, name):
        self.context = copy.deepcopy(self.contexts[name])
        debug("Moving to context {!r}".format(name))
        debug("Context args: {!r}".format(self.context.args))
        debug("Context flags: {!r}".format(self.context.flags))
        debug("Context inverse_flags: {!r}".format(self.context.inverse_flags))

    def complete_flag(self):
        if self.flag:
            msg = "Completing current flag {} before moving on"
            debug(msg.format(self.flag))
        # Barf if we needed a value and didn't get one
        if (
            self.flag
            and self.flag.takes_value
            and self.flag.raw_value is None
            and not self.flag.optional
        ):
            err = "Flag {!r} needed value and was not given one!"
            self.error(err.format(self.flag))
        # Handle optional-value flags; at this point they were not given an
        # explicit value, but they were seen, ergo they should get treated like
        # bools.
        if self.flag and self.flag.raw_value is None and self.flag.optional:
            msg = "Saw optional flag {!r} go by w/ no value; setting to True"
            debug(msg.format(self.flag.name))
            # Skip casting so the bool gets preserved
            self.flag.set_value(True, cast=False)

    def check_ambiguity(self, value):
        """
        Guard against ambiguity when current flag takes an optional value.

        .. versionadded:: 1.0
        """
        # No flag is currently being examined, or one is but it doesn't take an
        # optional value? Ambiguity isn't possible.
        if not (self.flag and self.flag.optional):
            return False
        # We *are* dealing with an optional-value flag, but it's already
        # received a value? There can't be ambiguity here either.
        if self.flag.raw_value is not None:
            return False
        # Otherwise, there *may* be ambiguity if 1 or more of the below tests
        # fail.
        tests = []
        # Unfilled posargs still exist?
        tests.append(self.context and self.context.missing_positional_args)
        # Value matches another valid task/context name?
        tests.append(value in self.contexts)
        if any(tests):
            msg = "{!r} is ambiguous when given after an optional-value flag"
            raise ParseError(msg.format(value))

    def switch_to_flag(self, flag, inverse=False):
        # Sanity check for ambiguity w/ prior optional-value flag
        self.check_ambiguity(flag)
        # Also tie it off, in case prior had optional value or etc. Seems to be
        # harmless for other kinds of flags. (TODO: this is a serious indicator
        # that we need to move some of this flag-by-flag bookkeeping into the
        # state machine bits, if possible - as-is it was REAL confusing re: why
        # this was manually required!)
        self.complete_flag()
        # Set flag/arg obj
        flag = self.context.inverse_flags[flag] if inverse else flag
        # Update state
        self.flag = self.context.flags[flag]
        debug("Moving to flag {!r}".format(self.flag))
        # Bookkeeping for iterable-type flags (where the typical 'value
        # non-empty/nondefault -> clearly it got its value already' test is
        # insufficient)
        self.flag_got_value = False
        # Handle boolean flags (which can immediately be updated)
        if not self.flag.takes_value:
            val = not inverse
            debug("Marking seen flag {!r} as {}".format(self.flag, val))
            self.flag.value = val

    def see_value(self, value):
        self.check_ambiguity(value)
        if self.flag.takes_value:
            debug("Setting flag {!r} to value {!r}".format(self.flag, value))
            self.flag.value = value
            self.flag_got_value = True
        else:
            self.error("Flag {!r} doesn't take any value!".format(self.flag))

    def see_positional_arg(self, value):
        for arg in self.context.positional_args:
            if arg.value is None:
                arg.value = value
                break

    def error(self, msg):
        raise ParseError(msg, self.context)


class ParseResult(list):
    """
    List-like object with some extra parse-related attributes.

    Specifically, a ``.remainder`` attribute, which is the string found after a
    ``--`` in any parsed argv list; and an ``.unparsed`` attribute, a list of
    tokens that were unable to be parsed.

    .. versionadded:: 1.0
    """

    def __init__(self, *args, **kwargs):
        super(ParseResult, self).__init__(*args, **kwargs)
        self.remainder = ""
        self.unparsed = []
