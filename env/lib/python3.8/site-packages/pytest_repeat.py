# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at https://www.mozilla.org/en-US/MPL/2.0/.
import warnings
from unittest import TestCase

import pytest


def pytest_addoption(parser):
    parser.addoption(
        '--count',
        action='store',
        default=1,
        type=int,
        help='Number of times to repeat each test')

    parser.addoption(
        '--repeat-scope',
        action='store',
        default='function',
        type=str,
        choices=('function', 'class', 'module', 'session'),
        help='Scope for repeating tests')


def pytest_configure(config):
    config.addinivalue_line(
        'markers',
        'repeat(n): run the given test function `n` times.')


class UnexpectedError(Exception):
    pass


@pytest.fixture(autouse=True)
def __pytest_repeat_step_number(request):
    if request.config.option.count > 1:
        try:
            return request.param
        except AttributeError:
            if issubclass(request.cls, TestCase):
                warnings.warn(
                    "Repeating unittest class tests not supported")
            else:
                raise UnexpectedError(
                    "This call couldn't work with pytest-repeat. "
                    "Please consider raising an issue with your usage.")


@pytest.hookimpl(trylast=True)
def pytest_generate_tests(metafunc):
    count = metafunc.config.option.count
    if hasattr(metafunc.function, 'repeat'):
        count = int(metafunc.function.repeat.args[0])
    if count > 1:

        def make_progress_id(i, n=count):
            return '{0}/{1}'.format(i + 1, n)

        scope = metafunc.config.option.repeat_scope
        metafunc.parametrize(
            '__pytest_repeat_step_number',
            range(count),
            indirect=True,
            ids=make_progress_id,
            scope=scope
        )
