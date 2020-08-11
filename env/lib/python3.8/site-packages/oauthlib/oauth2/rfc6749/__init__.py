# -*- coding: utf-8 -*-
"""
oauthlib.oauth2.rfc6749
~~~~~~~~~~~~~~~~~~~~~~~

This module is an implementation of various logic needed
for consuming and providing OAuth 2.0 RFC6749.
"""
from __future__ import absolute_import, unicode_literals

import functools
import logging

from .endpoints.base import BaseEndpoint
from .endpoints.base import catch_errors_and_unavailability
from .errors import TemporarilyUnavailableError, ServerError
from .errors import FatalClientError, OAuth2Error


log = logging.getLogger(__name__)
