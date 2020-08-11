from .util import six

from .config import Config
from .parser import ParserContext
from .util import debug
from .tasks import Call, Task


class Executor(object):
    """
    An execution strategy for Task objects.

    Subclasses may override various extension points to change, add or remove
    behavior.

    .. versionadded:: 1.0
    """

    def __init__(self, collection, config=None, core=None):
        """
        Initialize executor with handles to necessary data structures.

        :param collection:
            A `.Collection` used to look up requested tasks (and their default
            config data, if any) by name during execution.

        :param config:
            An optional `.Config` holding configuration state. Defaults to an
            empty `.Config` if not given.

        :param core:
            An optional `.ParseResult` holding parsed core program arguments.
            Defaults to ``None``.
        """
        self.collection = collection
        self.config = config if config is not None else Config()
        self.core = core

    def execute(self, *tasks):
        """
        Execute one or more ``tasks`` in sequence.

        :param tasks:
            An all-purpose iterable of "tasks to execute", each member of which
            may take one of the following forms:

            **A string** naming a task from the Executor's `.Collection`. This
            name may contain dotted syntax appropriate for calling namespaced
            tasks, e.g. ``subcollection.taskname``. Such tasks are executed
            without arguments.

            **A two-tuple** whose first element is a task name string (as
            above) and whose second element is a dict suitable for use as
            ``**kwargs`` when calling the named task. E.g.::

                [
                    ('task1', {}),
                    ('task2', {'arg1': 'val1'}),
                    ...
                ]

            is equivalent, roughly, to::

                task1()
                task2(arg1='val1')

            **A `.ParserContext`** instance, whose ``.name`` attribute is used
            as the task name and whose ``.as_kwargs`` attribute is used as the
            task kwargs (again following the above specifications).

            .. note::
                When called without any arguments at all (i.e. when ``*tasks``
                is empty), the default task from ``self.collection`` is used
                instead, if defined.

        :returns:
            A dict mapping task objects to their return values.

            This dict may include pre- and post-tasks if any were executed. For
            example, in a collection with a ``build`` task depending on another
            task named ``setup``, executing ``build`` will result in a dict
            with two keys, one for ``build`` and one for ``setup``.

        .. versionadded:: 1.0
        """
        # Normalize input
        debug("Examining top level tasks {!r}".format([x for x in tasks]))
        calls = self.normalize(tasks)
        debug("Tasks (now Calls) with kwargs: {!r}".format(calls))
        # Obtain copy of directly-given tasks since they should sometimes
        # behave differently
        direct = list(calls)
        # Expand pre/post tasks
        # TODO: may make sense to bundle expansion & deduping now eh?
        expanded = self.expand_calls(calls)
        # Get some good value for dedupe option, even if config doesn't have
        # the tree we expect. (This is a concession to testing.)
        try:
            dedupe = self.config.tasks.dedupe
        except AttributeError:
            dedupe = True
        # Dedupe across entire run now that we know about all calls in order
        calls = self.dedupe(expanded) if dedupe else expanded
        # Execute
        results = {}
        # TODO: maybe clone initial config here? Probably not necessary,
        # especially given Executor is not designed to execute() >1 time at the
        # moment...
        for call in calls:
            autoprint = call in direct and call.autoprint
            args = call.args
            debug("Executing {!r}".format(call))
            # Hand in reference to our config, which will preserve user
            # modifications across the lifetime of the session.
            config = self.config
            # But make sure we reset its task-sensitive levels each time
            # (collection & shell env)
            # TODO: load_collection needs to be skipped if task is anonymous
            # (Fabric 2 or other subclassing libs only)
            collection_config = self.collection.configuration(call.called_as)
            config.load_collection(collection_config)
            config.load_shell_env()
            debug("Finished loading collection & shell env configs")
            # Get final context from the Call (which will know how to generate
            # an appropriate one; e.g. subclasses might use extra data from
            # being parameterized), handing in this config for use there.
            context = call.make_context(config)
            args = (context,) + args
            result = call.task(*args, **call.kwargs)
            if autoprint:
                print(result)
            # TODO: handle the non-dedupe case / the same-task-different-args
            # case, wherein one task obj maps to >1 result.
            results[call.task] = result
        return results

    def normalize(self, tasks):
        """
        Transform arbitrary task list w/ various types, into `.Call` objects.

        See docstring for `~.Executor.execute` for details.

        .. versionadded:: 1.0
        """
        calls = []
        for task in tasks:
            name, kwargs = None, {}
            if isinstance(task, six.string_types):
                name = task
            elif isinstance(task, ParserContext):
                name = task.name
                kwargs = task.as_kwargs
            else:
                name, kwargs = task
            c = Call(task=self.collection[name], kwargs=kwargs, called_as=name)
            calls.append(c)
        if not tasks and self.collection.default is not None:
            calls = [Call(task=self.collection[self.collection.default])]
        return calls

    def dedupe(self, calls):
        """
        Deduplicate a list of `tasks <.Call>`.

        :param calls: An iterable of `.Call` objects representing tasks.

        :returns: A list of `.Call` objects.

        .. versionadded:: 1.0
        """
        deduped = []
        debug("Deduplicating tasks...")
        for call in calls:
            if call not in deduped:
                debug("{!r}: no duplicates found, ok".format(call))
                deduped.append(call)
            else:
                debug("{!r}: found in list already, skipping".format(call))
        return deduped

    def expand_calls(self, calls):
        """
        Expand a list of `.Call` objects into a near-final list of same.

        The default implementation of this method simply adds a task's
        pre/post-task list before/after the task itself, as necessary.

        Subclasses may wish to do other things in addition (or instead of) the
        above, such as multiplying the `calls <.Call>` by argument vectors or
        similar.

        .. versionadded:: 1.0
        """
        ret = []
        for call in calls:
            # Normalize to Call (this method is sometimes called with pre/post
            # task lists, which may contain 'raw' Task objects)
            if isinstance(call, Task):
                call = Call(task=call)
            debug("Expanding task-call {!r}".format(call))
            # TODO: this is where we _used_ to call Executor.config_for(call,
            # config)...
            # TODO: now we may need to preserve more info like where the call
            # came from, etc, but I feel like that shit should go _on the call
            # itself_ right???
            # TODO: we _probably_ don't even want the config in here anymore,
            # we want this to _just_ be about the recursion across pre/post
            # tasks or parameterization...?
            ret.extend(self.expand_calls(call.pre))
            ret.append(call)
            ret.extend(self.expand_calls(call.post))
        return ret
