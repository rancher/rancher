import os

import py
import pytest


def auto_detect_cpus():
    try:
        from os import sched_getaffinity
    except ImportError:
        if os.environ.get("TRAVIS") == "true":
            # workaround https://bitbucket.org/pypy/pypy/issues/2375
            return 2
        try:
            from os import cpu_count
        except ImportError:
            from multiprocessing import cpu_count
    else:

        def cpu_count():
            return len(sched_getaffinity(0))

    try:
        n = cpu_count()
    except NotImplementedError:
        return 1
    return n if n else 1


def parse_numprocesses(s):
    if s == "auto":
        return auto_detect_cpus()
    else:
        return int(s)


def pytest_addoption(parser):
    group = parser.getgroup("xdist", "distributed and subprocess testing")
    group._addoption(
        "-n",
        "--numprocesses",
        dest="numprocesses",
        metavar="numprocesses",
        action="store",
        type=parse_numprocesses,
        help="shortcut for '--dist=load --tx=NUM*popen', "
        "you can use 'auto' here for auto detection CPUs number on "
        "host system",
    )
    group.addoption(
        "--max-worker-restart",
        "--max-slave-restart",
        action="store",
        default=None,
        dest="maxworkerrestart",
        help="maximum number of workers that can be restarted "
        "when crashed (set to zero to disable this feature)\n"
        "'--max-slave-restart' option is deprecated and will be removed in "
        "a future release",
    )
    group.addoption(
        "--dist",
        metavar="distmode",
        action="store",
        choices=["each", "load", "loadscope", "loadfile", "no"],
        dest="dist",
        default="no",
        help=(
            "set mode for distributing tests to exec environments.\n\n"
            "each: send each test to all available environments.\n\n"
            "load: load balance by sending any pending test to any"
            " available environment.\n\n"
            "loadscope: load balance by sending pending groups of tests in"
            " the same scope to any available environment.\n\n"
            "loadfile: load balance by sending test grouped by file"
            " to any available environment.\n\n"
            "(default) no: run tests inprocess, don't distribute."
        ),
    )
    group.addoption(
        "--tx",
        dest="tx",
        action="append",
        default=[],
        metavar="xspec",
        help=(
            "add a test execution environment. some examples: "
            "--tx popen//python=python2.5 --tx socket=192.168.1.102:8888 "
            "--tx ssh=user@codespeak.net//chdir=testcache"
        ),
    )
    group._addoption(
        "-d",
        action="store_true",
        dest="distload",
        default=False,
        help="load-balance tests.  shortcut for '--dist=load'",
    )
    group.addoption(
        "--rsyncdir",
        action="append",
        default=[],
        metavar="DIR",
        help="add directory for rsyncing to remote tx nodes.",
    )
    group.addoption(
        "--rsyncignore",
        action="append",
        default=[],
        metavar="GLOB",
        help="add expression for ignores when rsyncing to remote tx nodes.",
    )

    group.addoption(
        "--boxed",
        action="store_true",
        help="backward compatibility alias for pytest-forked --forked",
    )
    parser.addini(
        "rsyncdirs",
        "list of (relative) paths to be rsynced for" " remote distributed testing.",
        type="pathlist",
    )
    parser.addini(
        "rsyncignore",
        "list of (relative) glob-style paths to be ignored " "for rsyncing.",
        type="pathlist",
    )
    parser.addini(
        "looponfailroots",
        type="pathlist",
        help="directories to check for changes",
        default=[py.path.local()],
    )


# -------------------------------------------------------------------------
# distributed testing hooks
# -------------------------------------------------------------------------


def pytest_addhooks(pluginmanager):
    from xdist import newhooks

    # avoid warnings with pytest-2.8
    method = getattr(pluginmanager, "add_hookspecs", None)
    if method is None:
        method = pluginmanager.addhooks
    method(newhooks)


# -------------------------------------------------------------------------
# distributed testing initialization
# -------------------------------------------------------------------------


@pytest.mark.trylast
def pytest_configure(config):
    if config.getoption("dist") != "no" and not config.getvalue("collectonly"):
        from xdist.dsession import DSession

        session = DSession(config)
        config.pluginmanager.register(session, "dsession")
        tr = config.pluginmanager.getplugin("terminalreporter")
        tr.showfspath = False
    if config.getoption("boxed"):
        config.option.forked = True


@pytest.mark.tryfirst
def pytest_cmdline_main(config):
    if config.option.numprocesses:
        if config.option.dist == "no":
            config.option.dist = "load"
        config.option.tx = ["popen"] * config.option.numprocesses
    if config.option.distload:
        config.option.dist = "load"
    val = config.getvalue
    if not val("collectonly"):
        usepdb = config.getoption("usepdb")  # a core option
        if val("dist") != "no":
            if usepdb:
                raise pytest.UsageError(
                    "--pdb is incompatible with distributing tests; try using -n0."
                )  # noqa: E501


# -------------------------------------------------------------------------
# fixtures
# -------------------------------------------------------------------------


@pytest.fixture(scope="session")
def worker_id(request):
    """Return the id of the current worker ('gw0', 'gw1', etc) or 'master'
    if running on the master node.
    """
    if hasattr(request.config, "workerinput"):
        return request.config.workerinput["workerid"]
    else:
        return "master"
