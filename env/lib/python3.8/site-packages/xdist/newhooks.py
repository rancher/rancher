"""
xdist hooks.

Additionally, pytest-xdist will also decorate a few other hooks
with the worker instance that executed the hook originally:

``pytest_runtest_logreport``: ``rep`` parameter has a ``node`` attribute.

You can use this hooks just as you would use normal pytest hooks, but some care
must be taken in plugins in case ``xdist`` is not installed. Please see:

    http://pytest.org/en/latest/writing_plugins.html#optionally-using-hooks-from-3rd-party-plugins
"""
import pytest


def pytest_xdist_setupnodes(config, specs):
    """ called before any remote node is set up. """


def pytest_xdist_newgateway(gateway):
    """ called on new raw gateway creation. """


def pytest_xdist_rsyncstart(source, gateways):
    """ called before rsyncing a directory to remote gateways takes place. """


def pytest_xdist_rsyncfinish(source, gateways):
    """ called after rsyncing a directory to remote gateways takes place. """


def pytest_configure_node(node):
    """ configure node information before it gets instantiated. """


def pytest_testnodeready(node):
    """ Test Node is ready to operate. """


def pytest_testnodedown(node, error):
    """ Test Node is down. """


def pytest_xdist_node_collection_finished(node, ids):
    """called by the master node when a node finishes collecting.
    """


@pytest.mark.firstresult
def pytest_xdist_make_scheduler(config, log):
    """ return a node scheduler implementation """
