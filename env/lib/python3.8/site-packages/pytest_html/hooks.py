# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at http://mozilla.org/MPL/2.0/.


def pytest_html_results_summary(prefix, summary, postfix):
    """ Called before adding the summary section to the report """


def pytest_html_results_table_header(cells):
    """ Called after building results table header. """


def pytest_html_results_table_row(report, cells):
    """ Called after building results table row. """


def pytest_html_results_table_html(report, data):
    """ Called after building results table additional HTML. """
