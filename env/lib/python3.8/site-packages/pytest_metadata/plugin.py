# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at http://mozilla.org/MPL/2.0/.
import json
import os
import platform

try:
    import _pytest._pluggy as pluggy
except ImportError:
    import pluggy
import pytest
import py

from pytest_metadata.ci import (
    appveyor,
    bitbucket,
    circleci,
    gitlab_ci,
    jenkins,
    taskcluster,
    travis_ci,
)

CONTINUOUS_INTEGRATION = [
    appveyor.ENVIRONMENT_VARIABLES,
    bitbucket.ENVIRONMENT_VARIABLES,
    circleci.ENVIRONMENT_VARIABLES,
    gitlab_ci.ENVIRONMENT_VARIABLES,
    jenkins.ENVIRONMENT_VARIABLES,
    taskcluster.ENVIRONMENT_VARIABLES,
    travis_ci.ENVIRONMENT_VARIABLES,
]


def pytest_addhooks(pluginmanager):
    from . import hooks

    pluginmanager.add_hookspecs(hooks)


@pytest.fixture(scope="session")
def metadata(pytestconfig):
    """Provide test session metadata"""
    return pytestconfig._metadata


def pytest_addoption(parser):
    parser.addoption(
        "--metadata",
        action="append",
        default=[],
        dest="metadata",
        metavar=("key", "value"),
        nargs=2,
        help="additional metadata.",
    )
    parser.addoption(
        "--metadata-from-json",
        action="store",
        default="{}",
        dest="metadata_from_json",
        help="additional metadata from a json string.",
    )


@pytest.hookimpl(tryfirst=True)
def pytest_configure(config):
    config._metadata = {
        "Python": platform.python_version(),
        "Platform": platform.platform(),
        "Packages": {
            "pytest": pytest.__version__,
            "py": py.__version__,
            "pluggy": pluggy.__version__,
        },
    }
    config._metadata.update({k: v for k, v in config.getoption("metadata")})
    config._metadata.update(json.loads(config.getoption("metadata_from_json")))

    plugins = dict()
    for plugin, dist in config.pluginmanager.list_plugin_distinfo():
        name, version = dist.project_name, dist.version
        if name.startswith("pytest-"):
            name = name[7:]
        plugins[name] = version
    config._metadata["Plugins"] = plugins

    for provider in CONTINUOUS_INTEGRATION:
        [
            config._metadata.update({var: os.environ.get(var)})
            for var in provider
            if os.environ.get(var)
        ]

    if hasattr(config, "workeroutput"):
        config.workeroutput["metadata"] = config._metadata

    config.hook.pytest_metadata(metadata=config._metadata)


def pytest_report_header(config):
    if config.getoption("verbose") > 0:
        return "metadata: {0}".format(config._metadata)


@pytest.mark.optionalhook
def pytest_testnodedown(node):
    # note that any metadata from remote workers will be replaced with the
    # environment from the final worker to quit
    if hasattr(node, "workeroutput"):
        node.config._metadata.update(node.workeroutput["metadata"])
