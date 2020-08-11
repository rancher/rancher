# PYTHON_ARGCOMPLETE_OK
"""
pytest: unit and functional testing with Python.
"""


# else we are imported

from _pytest.config import main, UsageError, cmdline, hookspec, hookimpl
from _pytest.fixtures import fixture, yield_fixture
from _pytest.assertion import register_assert_rewrite
from _pytest.freeze_support import freeze_includes
from _pytest import __version__
from _pytest.debugging import pytestPDB as __pytestPDB
from _pytest.recwarn import warns, deprecated_call
from _pytest.outcomes import fail, skip, importorskip, exit, xfail
from _pytest.mark import MARK_GEN as mark, param
from _pytest.main import Session
from _pytest.nodes import Item, Collector, File
from _pytest.fixtures import fillfixtures as _fillfuncargs
from _pytest.python import Package, Module, Class, Instance, Function, Generator
from _pytest.python_api import approx, raises
from _pytest.warning_types import (
    PytestWarning,
    PytestDeprecationWarning,
    RemovedInPytest4Warning,
    PytestExperimentalApiWarning,
)

set_trace = __pytestPDB.set_trace

__all__ = [
    "__version__",
    "_fillfuncargs",
    "approx",
    "Class",
    "cmdline",
    "Collector",
    "deprecated_call",
    "exit",
    "fail",
    "File",
    "fixture",
    "freeze_includes",
    "Function",
    "Generator",
    "hookimpl",
    "hookspec",
    "importorskip",
    "Instance",
    "Item",
    "main",
    "mark",
    "Module",
    "Package",
    "param",
    "PytestDeprecationWarning",
    "PytestExperimentalApiWarning",
    "PytestWarning",
    "raises",
    "register_assert_rewrite",
    "RemovedInPytest4Warning",
    "Session",
    "set_trace",
    "skip",
    "UsageError",
    "warns",
    "xfail",
    "yield_fixture",
]

if __name__ == "__main__":
    # if run as a script or by 'python -m pytest'
    # we trigger the below "else" condition by the following import
    import pytest

    raise SystemExit(pytest.main())
else:

    from _pytest.compat import _setup_collect_fakemodule

    _setup_collect_fakemodule()
