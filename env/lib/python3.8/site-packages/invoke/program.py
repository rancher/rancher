from __future__ import unicode_literals, print_function

import getpass
import inspect
import json
import os
import sys
import textwrap

from .util import six

from . import Collection, Config, Executor, FilesystemLoader
from .completion.complete import complete, print_completion_script
from .parser import Parser, ParserContext, Argument
from .exceptions import UnexpectedExit, CollectionNotFound, ParseError, Exit
from .terminals import pty_size
from .util import debug, enable_logging, helpline


class Program(object):
    """
    Manages top-level CLI invocation, typically via ``setup.py`` entrypoints.

    Designed for distributing Invoke task collections as standalone programs,
    but also used internally to implement the ``invoke`` program itself.

    .. seealso::
        :ref:`reusing-as-a-binary` for a tutorial/walkthrough of this
        functionality.

    .. versionadded:: 1.0
    """

    def core_args(self):
        """
        Return default core `.Argument` objects, as a list.

        .. versionadded:: 1.0
        """
        # Arguments present always, even when wrapped as a different binary
        return [
            Argument(
                names=("complete",),
                kind=bool,
                default=False,
                help="Print tab-completion candidates for given parse remainder.",  # noqa
            ),
            Argument(
                names=("print-completion-script",),
                kind=str,
                default="",
                help="Print the tab-completion script for your preferred shell (bash|zsh|fish).",  # noqa
            ),
            Argument(
                names=("debug", "d"),
                kind=bool,
                default=False,
                help="Enable debug output.",
            ),
            Argument(
                names=("prompt-for-sudo-password",),
                kind=bool,
                default=False,
                help="Prompt user at start of session for the sudo.password config value.",  # noqa
            ),
            Argument(
                names=("write-pyc",),
                kind=bool,
                default=False,
                help="Enable creation of .pyc files.",
            ),
            Argument(
                names=("echo", "e"),
                kind=bool,
                default=False,
                help="Echo executed commands before running.",
            ),
            Argument(
                names=("config", "f"),
                help="Runtime configuration file to use.",
            ),
            Argument(
                names=("help", "h"),
                optional=True,
                help="Show core or per-task help and exit.",
            ),
            Argument(
                names=("hide",),
                help="Set default value of run()'s 'hide' kwarg.",
            ),
            Argument(
                names=("list", "l"),
                optional=True,
                help="List available tasks, optionally limited to a namespace.",  # noqa
            ),
            Argument(
                names=("list-depth", "D"),
                kind=int,
                default=0,
                help="When listing tasks, only show the first INT levels.",
            ),
            Argument(
                names=("list-format", "F"),
                help="Change the display format used when listing tasks. Should be one of: flat (default), nested, json.",  # noqa
                default="flat",
            ),
            Argument(
                names=("pty", "p"),
                kind=bool,
                default=False,
                help="Use a pty when executing shell commands.",
            ),
            Argument(
                names=("version", "V"),
                kind=bool,
                default=False,
                help="Show version and exit.",
            ),
            Argument(
                names=("warn-only", "w"),
                kind=bool,
                default=False,
                help="Warn, instead of failing, when shell commands fail.",
            ),
        ]

    def task_args(self):
        """
        Return default task-related `.Argument` objects, as a list.

        These are only added to the core args in "task runner" mode (the
        default for ``invoke`` itself) - they are omitted when the constructor
        is given a non-empty ``namespace`` argument ("bundled namespace" mode).

        .. versionadded:: 1.0
        """
        # Arguments pertaining specifically to invocation as 'invoke' itself
        # (or as other arbitrary-task-executing programs, like 'fab')
        return [
            Argument(
                names=("collection", "c"),
                help="Specify collection name to load.",
            ),
            Argument(
                names=("no-dedupe",),
                kind=bool,
                default=False,
                help="Disable task deduplication.",
            ),
            Argument(
                names=("search-root", "r"),
                help="Change root directory used for finding task modules.",
            ),
        ]

    # Other class-level global variables a subclass might override sometime
    # maybe?
    leading_indent_width = 2
    leading_indent = " " * leading_indent_width
    indent_width = 4
    indent = " " * indent_width
    col_padding = 3

    def __init__(
        self,
        version=None,
        namespace=None,
        name=None,
        binary=None,
        loader_class=None,
        executor_class=None,
        config_class=None,
        binary_names=None,
    ):
        """
        Create a new, parameterized `.Program` instance.

        :param str version:
            The program's version, e.g. ``"0.1.0"``. Defaults to ``"unknown"``.

        :param namespace:
            A `.Collection` to use as this program's subcommands.

            If ``None`` (the default), the program will behave like ``invoke``,
            seeking a nearby task namespace with a `.Loader` and exposing
            arguments such as :option:`--list` and :option:`--collection` for
            inspecting or selecting specific namespaces.

            If given a `.Collection` object, will use it as if it had been
            handed to :option:`--collection`. Will also update the parser to
            remove references to tasks and task-related options, and display
            the subcommands in ``--help`` output. The result will be a program
            that has a static set of subcommands.

        :param str name:
            The program's name, as displayed in ``--version`` output.

            If ``None`` (default), is a capitalized version of the first word
            in the ``argv`` handed to `.run`. For example, when invoked from a
            binstub installed as ``foobar``, it will default to ``Foobar``.

        :param str binary:
            Descriptive lowercase binary name string used in help text.

            For example, Invoke's own internal value for this is ``inv[oke]``,
            denoting that it is installed as both ``inv`` and ``invoke``. As
            this is purely text intended for help display, it may be in any
            format you wish, though it should match whatever you've put into
            your ``setup.py``'s ``console_scripts`` entry.

            If ``None`` (default), uses the first word in ``argv`` verbatim (as
            with ``name`` above, except not capitalized).

        :param list binary_names:
            List of binary name strings, for use in completion scripts.

            This list ensures that the shell completion scripts generated by
            :option:`--print-completion-script` instruct the shell to use
            that completion for all of this program's installed names.

            For example, Invoke's internal default for this is ``["inv",
            "invoke"]``.

            If ``None`` (the default), the first word in ``argv`` (in the
            invocation of :option:`--print-completion-script`) is used in a
            single-item list.

        :param loader_class:
            The `.Loader` subclass to use when loading task collections.

            Defaults to `.FilesystemLoader`.

        :param executor_class:
            The `.Executor` subclass to use when executing tasks.

            Defaults to `.Executor`.

        :param config_class:
            The `.Config` subclass to use for the base config object.

            Defaults to `.Config`.

        .. versionchanged:: 1.2
            Added the ``binary_names`` argument.
        """
        self.version = "unknown" if version is None else version
        self.namespace = namespace
        self._name = name
        # TODO 2.0: rename binary to binary_help_name or similar. (Or write
        # code to autogenerate it from binary_names.)
        self._binary = binary
        self._binary_names = binary_names
        self.argv = None
        self.loader_class = loader_class or FilesystemLoader
        self.executor_class = executor_class or Executor
        self.config_class = config_class or Config

    def create_config(self):
        """
        Instantiate a `.Config` (or subclass, depending) for use in task exec.

        This Config is fully usable but will lack runtime-derived data like
        project & runtime config files, CLI arg overrides, etc. That data is
        added later in `update_config`. See `.Config` docstring for lifecycle
        details.

        :returns: ``None``; sets ``self.config`` instead.

        .. versionadded:: 1.0
        """
        self.config = self.config_class()

    def update_config(self, merge=True):
        """
        Update the previously instantiated `.Config` with parsed data.

        For example, this is how ``--echo`` is able to override the default
        config value for ``run.echo``.

        :param bool merge:
            Whether to merge at the end, or defer. Primarily useful for
            subclassers. Default: ``True``.

        .. versionadded:: 1.0
        """
        # Now that we have parse results handy, we can grab the remaining
        # config bits:
        # - runtime config, as it is dependent on the runtime flag/env var
        # - the overrides config level, as it is composed of runtime flag data
        # NOTE: only fill in values that would alter behavior, otherwise we
        # want the defaults to come through.
        run = {}
        if self.args["warn-only"].value:
            run["warn"] = True
        if self.args.pty.value:
            run["pty"] = True
        if self.args.hide.value:
            run["hide"] = self.args.hide.value
        if self.args.echo.value:
            run["echo"] = True
        tasks = {}
        if "no-dedupe" in self.args and self.args["no-dedupe"].value:
            tasks["dedupe"] = False
        # Handle "fill in config values at start of runtime", which for now is
        # just sudo password
        sudo = {}
        if self.args["prompt-for-sudo-password"].value:
            prompt = "Desired 'sudo.password' config value: "
            sudo["password"] = getpass.getpass(prompt)
        overrides = dict(run=run, tasks=tasks, sudo=sudo)
        self.config.load_overrides(overrides, merge=False)
        runtime_path = self.args.config.value
        if runtime_path is None:
            runtime_path = os.environ.get("INVOKE_RUNTIME_CONFIG", None)
        self.config.set_runtime_path(runtime_path)
        self.config.load_runtime(merge=False)
        if merge:
            self.config.merge()

    def run(self, argv=None, exit=True):
        """
        Execute main CLI logic, based on ``argv``.

        :param argv:
            The arguments to execute against. May be ``None``, a list of
            strings, or a string. See `.normalize_argv` for details.

        :param bool exit:
            When ``False`` (default: ``True``), will ignore `.ParseError`,
            `.Exit` and `.Failure` exceptions, which otherwise trigger calls to
            `sys.exit`.

            .. note::
                This is mostly a concession to testing. If you're setting this
                to ``False`` in a production setting, you should probably be
                using `.Executor` and friends directly instead!

        .. versionadded:: 1.0
        """
        try:
            # Create an initial config, which will hold defaults & values from
            # most config file locations (all but runtime.) Used to inform
            # loading & parsing behavior.
            self.create_config()
            # Parse the given ARGV with our CLI parsing machinery, resulting in
            # things like self.args (core args/flags), self.collection (the
            # loaded namespace, which may be affected by the core flags) and
            # self.tasks (the tasks requested for exec and their own
            # args/flags)
            self.parse_core(argv)
            # Handle collection concerns including project config
            self.parse_collection()
            # Parse remainder of argv as task-related input
            self.parse_tasks()
            # End of parsing (typically bailout stuff like --list, --help)
            self.parse_cleanup()
            # Update the earlier Config with new values from the parse step -
            # runtime config file contents and flag-derived overrides (e.g. for
            # run()'s echo, warn, etc options.)
            self.update_config()
            # Create an Executor, passing in the data resulting from the prior
            # steps, then tell it to execute the tasks.
            self.execute()
        except (UnexpectedExit, Exit, ParseError) as e:
            debug("Received a possibly-skippable exception: {!r}".format(e))
            # Print error messages from parser, runner, etc if necessary;
            # prevents messy traceback but still clues interactive user into
            # problems.
            if isinstance(e, ParseError):
                print(e, file=sys.stderr)
            if isinstance(e, Exit) and e.message:
                print(e.message, file=sys.stderr)
            if isinstance(e, UnexpectedExit) and e.result.hide:
                print(e, file=sys.stderr, end="")
            # Terminate execution unless we were told not to.
            if exit:
                if isinstance(e, UnexpectedExit):
                    code = e.result.exited
                elif isinstance(e, Exit):
                    code = e.code
                elif isinstance(e, ParseError):
                    code = 1
                sys.exit(code)
            else:
                debug("Invoked as run(..., exit=False), ignoring exception")
        except KeyboardInterrupt:
            sys.exit(1)  # Same behavior as Python itself outside of REPL

    def parse_core(self, argv):
        debug("argv given to Program.run: {!r}".format(argv))
        self.normalize_argv(argv)

        # Obtain core args (sets self.core)
        self.parse_core_args()
        debug("Finished parsing core args")

        # Set interpreter bytecode-writing flag
        sys.dont_write_bytecode = not self.args["write-pyc"].value

        # Enable debugging from here on out, if debug flag was given.
        # (Prior to this point, debugging requires setting INVOKE_DEBUG).
        if self.args.debug.value:
            enable_logging()

        # Short-circuit if --version
        if self.args.version.value:
            debug("Saw --version, printing version & exiting")
            self.print_version()
            raise Exit

        # Print (dynamic, no tasks required) completion script if requested
        if self.args["print-completion-script"].value:
            print_completion_script(
                shell=self.args["print-completion-script"].value,
                names=self.binary_names,
            )
            raise Exit

    def parse_collection(self):
        """
        Load a tasks collection & project-level config.

        .. versionadded:: 1.0
        """
        # Load a collection of tasks unless one was already set.
        if self.namespace is not None:
            debug(
                "Program was given default namespace, not loading collection"
            )
            self.collection = self.namespace
        else:
            debug(
                "No default namespace provided, trying to load one from disk"
            )  # noqa
            # If no bundled namespace & --help was given, just print it and
            # exit. (If we did have a bundled namespace, core --help will be
            # handled *after* the collection is loaded & parsing is done.)
            if self.args.help.value is True:
                debug(
                    "No bundled namespace & bare --help given; printing help."
                )
                self.print_help()
                raise Exit
            self.load_collection()
        # Set these up for potential use later when listing tasks
        # TODO: be nice if these came from the config...! Users would love to
        # say they default to nested for example. Easy 2.x feature-add.
        self.list_root = None
        self.list_depth = None
        self.list_format = "flat"
        self.scoped_collection = self.collection

        # TODO: load project conf, if possible, gracefully

    def parse_cleanup(self):
        """
        Post-parsing, pre-execution steps such as --help, --list, etc.

        .. versionadded:: 1.0
        """
        halp = self.args.help.value or self.core_via_tasks.args.help.value

        # Core (no value given) --help output (only when bundled namespace)
        if halp is True:
            debug("Saw bare --help, printing help & exiting")
            self.print_help()
            raise Exit

        # Print per-task help, if necessary
        if halp:
            if halp in self.parser.contexts:
                msg = "Saw --help <taskname>, printing per-task help & exiting"
                debug(msg)
                self.print_task_help(halp)
                raise Exit
            else:
                # TODO: feels real dumb to factor this out of Parser, but...we
                # should?
                raise ParseError("No idea what '{}' is!".format(halp))

        # Print discovered tasks if necessary
        list_root = self.args.list.value  # will be True or string
        self.list_format = self.args["list-format"].value
        self.list_depth = self.args["list-depth"].value
        if list_root:
            # Not just --list, but --list some-root - do moar work
            if isinstance(list_root, six.string_types):
                self.list_root = list_root
                try:
                    sub = self.collection.subcollection_from_path(list_root)
                    self.scoped_collection = sub
                except KeyError:
                    msg = "Sub-collection '{}' not found!"
                    raise Exit(msg.format(list_root))
            self.list_tasks()
            raise Exit

        # Print completion helpers if necessary
        if self.args.complete.value:
            complete(
                names=self.binary_names,
                core=self.core,
                initial_context=self.initial_context,
                collection=self.collection,
            )

        # Fallback behavior if no tasks were given & no default specified
        # (mostly a subroutine for overriding purposes)
        # NOTE: when there is a default task, Executor will select it when no
        # tasks were found in CLI parsing.
        if not self.tasks and not self.collection.default:
            self.no_tasks_given()

    def no_tasks_given(self):
        debug(
            "No tasks specified for execution and no default task; printing global help as fallback"  # noqa
        )
        self.print_help()
        raise Exit

    def execute(self):
        """
        Hand off data & tasks-to-execute specification to an `.Executor`.

        .. note::
            Client code just wanting a different `.Executor` subclass can just
            set ``executor_class`` in `.__init__`.

        .. versionadded:: 1.0
        """
        executor = self.executor_class(self.collection, self.config, self.core)
        executor.execute(*self.tasks)

    def normalize_argv(self, argv):
        """
        Massages ``argv`` into a useful list of strings.

        **If None** (the default), uses `sys.argv`.

        **If a non-string iterable**, uses that in place of `sys.argv`.

        **If a string**, performs a `str.split` and then executes with the
        result. (This is mostly a convenience; when in doubt, use a list.)

        Sets ``self.argv`` to the result.

        .. versionadded:: 1.0
        """
        if argv is None:
            argv = sys.argv
            debug("argv was None; using sys.argv: {!r}".format(argv))
        elif isinstance(argv, six.string_types):
            argv = argv.split()
            debug("argv was string-like; splitting: {!r}".format(argv))
        self.argv = argv

    @property
    def name(self):
        """
        Derive program's human-readable name based on `.binary`.

        .. versionadded:: 1.0
        """
        return self._name or self.binary.capitalize()

    @property
    def called_as(self):
        """
        Returns the program name we were actually called as.

        Specifically, this is the (Python's os module's concept of a) basename
        of the first argument in the parsed argument vector.

        .. versionadded:: 1.2
        """
        return os.path.basename(self.argv[0])

    @property
    def binary(self):
        """
        Derive program's help-oriented binary name(s) from init args & argv.

        .. versionadded:: 1.0
        """
        return self._binary or self.called_as

    @property
    def binary_names(self):
        """
        Derive program's completion-oriented binary name(s) from args & argv.

        .. versionadded:: 1.2
        """
        return self._binary_names or [self.called_as]

    @property
    def args(self):
        """
        Obtain core program args from ``self.core`` parse result.

        .. versionadded:: 1.0
        """
        return self.core[0].args

    @property
    def initial_context(self):
        """
        The initial parser context, aka core program flags.

        The specific arguments contained therein will differ depending on
        whether a bundled namespace was specified in `.__init__`.

        .. versionadded:: 1.0
        """
        args = self.core_args()
        if self.namespace is None:
            args += self.task_args()
        return ParserContext(args=args)

    def print_version(self):
        print("{} {}".format(self.name, self.version or "unknown"))

    def print_help(self):
        usage_suffix = "task1 [--task1-opts] ... taskN [--taskN-opts]"
        if self.namespace is not None:
            usage_suffix = "<subcommand> [--subcommand-opts] ..."
        print("Usage: {} [--core-opts] {}".format(self.binary, usage_suffix))
        print("")
        print("Core options:")
        print("")
        self.print_columns(self.initial_context.help_tuples())
        if self.namespace is not None:
            self.list_tasks()

    def parse_core_args(self):
        """
        Filter out core args, leaving any tasks or their args for later.

        Sets ``self.core`` to the `.ParseResult` from this step.

        .. versionadded:: 1.0
        """
        debug("Parsing initial context (core args)")
        parser = Parser(initial=self.initial_context, ignore_unknown=True)
        self.core = parser.parse_argv(self.argv[1:])
        msg = "Core-args parse result: {!r} & unparsed: {!r}"
        debug(msg.format(self.core, self.core.unparsed))

    def load_collection(self):
        """
        Load a task collection based on parsed core args, or die trying.

        .. versionadded:: 1.0
        """
        # NOTE: start, coll_name both fall back to configuration values within
        # Loader (which may, however, get them from our config.)
        start = self.args["search-root"].value
        loader = self.loader_class(config=self.config, start=start)
        coll_name = self.args.collection.value
        try:
            module, parent = loader.load(coll_name)
            # This is the earliest we can load project config, so we should -
            # allows project config to affect the task parsing step!
            # TODO: is it worth merging these set- and load- methods? May
            # require more tweaking of how things behave in/after __init__.
            self.config.set_project_location(parent)
            self.config.load_project()
            self.collection = Collection.from_module(
                module,
                loaded_from=parent,
                auto_dash_names=self.config.tasks.auto_dash_names,
            )
        except CollectionNotFound as e:
            raise Exit("Can't find any collection named {!r}!".format(e.name))

    def parse_tasks(self):
        """
        Parse leftover args, which are typically tasks & per-task args.

        Sets ``self.parser`` to the parser used, ``self.tasks`` to the
        parsed per-task contexts, and ``self.core_via_tasks`` to a context
        holding any core flags seen within the task contexts.

        .. versionadded:: 1.0
        """
        self.parser = Parser(
            initial=self.initial_context,
            contexts=self.collection.to_contexts(),
        )
        debug("Parsing tasks against {!r}".format(self.collection))
        result = self.parser.parse_argv(self.core.unparsed)
        # TODO: can we easily 'merge' this into self.core? Ehh
        self.core_via_tasks = result.pop(0)
        self.tasks = result
        debug("Resulting task contexts: {!r}".format(self.tasks))

    def print_task_help(self, name):
        """
        Print help for a specific task, e.g. ``inv --help <taskname>``.

        .. versionadded:: 1.0
        """
        # Setup
        ctx = self.parser.contexts[name]
        tuples = ctx.help_tuples()
        docstring = inspect.getdoc(self.collection[name])
        header = "Usage: {} [--core-opts] {} {}[other tasks here ...]"
        opts = "[--options] " if tuples else ""
        print(header.format(self.binary, name, opts))
        print("")
        print("Docstring:")
        if docstring:
            # Really wish textwrap worked better for this.
            for line in docstring.splitlines():
                if line.strip():
                    print(self.leading_indent + line)
                else:
                    print("")
            print("")
        else:
            print(self.leading_indent + "none")
            print("")
        print("Options:")
        if tuples:
            self.print_columns(tuples)
        else:
            print(self.leading_indent + "none")
            print("")

    def list_tasks(self):
        # Short circuit if no tasks to show (Collection now implements bool)
        focus = self.scoped_collection
        if not focus:
            msg = "No tasks found in collection '{}'!"
            raise Exit(msg.format(focus.name))
        # TODO: now that flat/nested are almost 100% unified, maybe rethink
        # this a bit?
        getattr(self, "list_{}".format(self.list_format))()

    def list_flat(self):
        pairs = self._make_pairs(self.scoped_collection)
        self.display_with_columns(pairs=pairs)

    def list_nested(self):
        pairs = self._make_pairs(self.scoped_collection)
        extra = "'*' denotes collection defaults"
        self.display_with_columns(pairs=pairs, extra=extra)

    def _make_pairs(self, coll, ancestors=None):
        if ancestors is None:
            ancestors = []
        pairs = []
        indent = len(ancestors) * self.indent
        ancestor_path = ".".join(x for x in ancestors)
        for name, task in sorted(six.iteritems(coll.tasks)):
            is_default = name == coll.default
            # Start with just the name and just the aliases, no prefixes or
            # dots.
            displayname = name
            aliases = list(map(coll.transform, sorted(task.aliases)))
            # If displaying a sub-collection (or if we are displaying a given
            # namespace/root), tack on some dots to make it clear these names
            # require dotted paths to invoke.
            if ancestors or self.list_root:
                displayname = ".{}".format(displayname)
                aliases = [".{}".format(x) for x in aliases]
            # Nested? Indent, and add asterisks to default-tasks.
            if self.list_format == "nested":
                prefix = indent
                if is_default:
                    displayname += "*"
            # Flat? Prefix names and aliases with ancestor names to get full
            # dotted path; and give default-tasks their collection name as the
            # first alias.
            if self.list_format == "flat":
                prefix = ancestor_path
                # Make sure leading dots are present for subcollections if
                # scoped display
                if prefix and self.list_root:
                    prefix = "." + prefix
                aliases = [prefix + alias for alias in aliases]
                if is_default and ancestors:
                    aliases.insert(0, prefix)
            # Generate full name and help columns and add to pairs.
            alias_str = " ({})".format(", ".join(aliases)) if aliases else ""
            full = prefix + displayname + alias_str
            pairs.append((full, helpline(task)))
        # Determine whether we're at max-depth or not
        truncate = self.list_depth and (len(ancestors) + 1) >= self.list_depth
        for name, subcoll in sorted(six.iteritems(coll.collections)):
            displayname = name
            if ancestors or self.list_root:
                displayname = ".{}".format(displayname)
            if truncate:
                tallies = [
                    "{} {}".format(len(getattr(subcoll, attr)), attr)
                    for attr in ("tasks", "collections")
                    if getattr(subcoll, attr)
                ]
                displayname += " [{}]".format(", ".join(tallies))
            if self.list_format == "nested":
                pairs.append((indent + displayname, helpline(subcoll)))
            elif self.list_format == "flat" and truncate:
                # NOTE: only adding coll-oriented pair if limiting by depth
                pairs.append((ancestor_path + displayname, helpline(subcoll)))
            # Recurse, if not already at max depth
            if not truncate:
                recursed_pairs = self._make_pairs(
                    coll=subcoll, ancestors=ancestors + [name]
                )
                pairs.extend(recursed_pairs)
        return pairs

    def list_json(self):
        # Sanity: we can't cleanly honor the --list-depth argument without
        # changing the data schema or otherwise acting strangely; and it also
        # doesn't make a ton of sense to limit depth when the output is for a
        # script to handle. So we just refuse, for now. TODO: find better way
        if self.list_depth:
            raise Exit(
                "The --list-depth option is not supported with JSON format!"
            )  # noqa
        # TODO: consider using something more formal re: the format this emits,
        # eg json-schema or whatever. Would simplify the
        # relatively-concise-but-only-human docs that currently describe this.
        coll = self.scoped_collection
        data = coll.serialized()
        print(json.dumps(data))

    def task_list_opener(self, extra=""):
        root = self.list_root
        depth = self.list_depth
        specifier = " '{}'".format(root) if root else ""
        tail = ""
        if depth or extra:
            depthstr = "depth={}".format(depth) if depth else ""
            joiner = "; " if (depth and extra) else ""
            tail = " ({}{}{})".format(depthstr, joiner, extra)
        text = "Available{} tasks{}".format(specifier, tail)
        # TODO: do use cases w/ bundled namespace want to display things like
        # root and depth too? Leaving off for now...
        if self.namespace is not None:
            text = "Subcommands"
        return text

    def display_with_columns(self, pairs, extra=""):
        root = self.list_root
        print("{}:\n".format(self.task_list_opener(extra=extra)))
        self.print_columns(pairs)
        # TODO: worth stripping this out for nested? since it's signified with
        # asterisk there? ugggh
        default = self.scoped_collection.default
        if default:
            specific = ""
            if root:
                specific = " '{}'".format(root)
                default = ".{}".format(default)
            # TODO: trim/prefix dots
            print("Default{} task: {}\n".format(specific, default))

    def print_columns(self, tuples):
        """
        Print tabbed columns from (name, help) ``tuples``.

        Useful for listing tasks + docstrings, flags + help strings, etc.

        .. versionadded:: 1.0
        """
        # Calculate column sizes: don't wrap flag specs, give what's left over
        # to the descriptions.
        name_width = max(len(x[0]) for x in tuples)
        desc_width = (
            pty_size()[0]
            - name_width
            - self.leading_indent_width
            - self.col_padding
            - 1
        )
        wrapper = textwrap.TextWrapper(width=desc_width)
        for name, help_str in tuples:
            if help_str is None:
                help_str = ""
            # Wrap descriptions/help text
            help_chunks = wrapper.wrap(help_str)
            # Print flag spec + padding
            name_padding = name_width - len(name)
            spec = "".join(
                (
                    self.leading_indent,
                    name,
                    name_padding * " ",
                    self.col_padding * " ",
                )
            )
            # Print help text as needed
            if help_chunks:
                print(spec + help_chunks[0])
                for chunk in help_chunks[1:]:
                    print((" " * len(spec)) + chunk)
            else:
                print(spec.rstrip())
        print("")
