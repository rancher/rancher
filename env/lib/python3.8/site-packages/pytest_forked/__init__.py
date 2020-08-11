import os
import warnings

import py
# we know this bit is bad, but we cant help it with the current pytest setup
from _pytest import runner
import pytest


# copied from xdist remote
def serialize_report(rep):
    import py
    d = rep.__dict__.copy()
    if hasattr(rep.longrepr, 'toterminal'):
        d['longrepr'] = str(rep.longrepr)
    else:
        d['longrepr'] = rep.longrepr
    for name in d:
        if isinstance(d[name], py.path.local):
            d[name] = str(d[name])
        elif name == "result":
            d[name] = None  # for now
    return d


def pytest_addoption(parser):
    group = parser.getgroup("forked", "forked subprocess test execution")
    group.addoption(
        '--forked',
        action="store_true", dest="forked", default=False,
        help="box each test run in a separate process (unix)")


def pytest_load_initial_conftests(early_config, parser, args):
    early_config.addinivalue_line(
        "markers",
        "forked: Always fork for this test.",
    )


@pytest.mark.tryfirst
def pytest_runtest_protocol(item):
    if item.config.getvalue("forked") or item.get_closest_marker("forked"):
        reports = forked_run_report(item)
        for rep in reports:
            item.ihook.pytest_runtest_logreport(report=rep)
        return True


def forked_run_report(item):
    # for now, we run setup/teardown in the subprocess
    # XXX optionally allow sharing of setup/teardown
    from _pytest.runner import runtestprotocol
    EXITSTATUS_TESTEXIT = 4
    import marshal

    def runforked():
        try:
            reports = runtestprotocol(item, log=False)
        except KeyboardInterrupt:
            os._exit(EXITSTATUS_TESTEXIT)
        return marshal.dumps([serialize_report(x) for x in reports])

    ff = py.process.ForkedFunc(runforked)
    result = ff.waitfinish()
    if result.retval is not None:
        report_dumps = marshal.loads(result.retval)
        return [runner.TestReport(**x) for x in report_dumps]
    else:
        if result.exitstatus == EXITSTATUS_TESTEXIT:
            pytest.exit("forked test item %s raised Exit" % (item,))
        return [report_process_crash(item, result)]


def report_process_crash(item, result):
    from _pytest._code.source import getfslineno
    path, lineno = getfslineno(item)
    info = ("%s:%s: running the test CRASHED with signal %d" %
            (path, lineno, result.signal))
    from _pytest import runner
    # pytest >= 4.1
    has_from_call = getattr(runner.CallInfo, "from_call", None) is not None
    if has_from_call:
        call = runner.CallInfo.from_call(lambda: 0/0, "???")
    else:
        call = runner.CallInfo(lambda: 0/0, "???")
    call.excinfo = info
    rep = runner.pytest_runtest_makereport(item, call)
    if result.out:
        rep.sections.append(("captured stdout", result.out))
    if result.err:
        rep.sections.append(("captured stderr", result.err))

    xfail_marker = item.get_closest_marker('xfail')
    if not xfail_marker:
        return rep

    rep.outcome = "skipped"
    rep.wasxfail = (
        "reason: {xfail_reason}; "
        "pytest-forked reason: {crash_info}".
        format(
            xfail_reason=xfail_marker.kwargs['reason'],
            crash_info=info,
        )
    )
    warnings.warn(
        'pytest-forked xfail support is incomplete at the moment and may '
        'output a misleading reason message',
        RuntimeWarning,
    )

    return rep
