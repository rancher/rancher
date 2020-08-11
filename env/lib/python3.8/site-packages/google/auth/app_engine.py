# Copyright 2016 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

"""Google App Engine standard environment support.

This module provides authentication and signing for applications running on App
Engine in the standard environment using the `App Identity API`_.


.. _App Identity API:
    https://cloud.google.com/appengine/docs/python/appidentity/
"""

import datetime

from google.auth import _helpers
from google.auth import credentials
from google.auth import crypt

# pytype: disable=import-error
try:
    from google.appengine.api import app_identity
except ImportError:
    app_identity = None
# pytype: enable=import-error


class Signer(crypt.Signer):
    """Signs messages using the App Engine App Identity service.

    This can be used in place of :class:`google.auth.crypt.Signer` when
    running in the App Engine standard environment.
    """

    @property
    def key_id(self):
        """Optional[str]: The key ID used to identify this private key.

        .. warning::
           This is always ``None``. The key ID used by App Engine can not
           be reliably determined ahead of time.
        """
        return None

    @_helpers.copy_docstring(crypt.Signer)
    def sign(self, message):
        message = _helpers.to_bytes(message)
        _, signature = app_identity.sign_blob(message)
        return signature


def get_project_id():
    """Gets the project ID for the current App Engine application.

    Returns:
        str: The project ID

    Raises:
        EnvironmentError: If the App Engine APIs are unavailable.
    """
    # pylint: disable=missing-raises-doc
    # Pylint rightfully thinks EnvironmentError is OSError, but doesn't
    # realize it's a valid alias.
    if app_identity is None:
        raise EnvironmentError("The App Engine APIs are not available.")
    return app_identity.get_application_id()


class Credentials(credentials.Scoped, credentials.Signing, credentials.Credentials):
    """App Engine standard environment credentials.

    These credentials use the App Engine App Identity API to obtain access
    tokens.
    """

    def __init__(self, scopes=None, service_account_id=None):
        """
        Args:
            scopes (Sequence[str]): Scopes to request from the App Identity
                API.
            service_account_id (str): The service account ID passed into
                :func:`google.appengine.api.app_identity.get_access_token`.
                If not specified, the default application service account
                ID will be used.

        Raises:
            EnvironmentError: If the App Engine APIs are unavailable.
        """
        # pylint: disable=missing-raises-doc
        # Pylint rightfully thinks EnvironmentError is OSError, but doesn't
        # realize it's a valid alias.
        if app_identity is None:
            raise EnvironmentError("The App Engine APIs are not available.")

        super(Credentials, self).__init__()
        self._scopes = scopes
        self._service_account_id = service_account_id
        self._signer = Signer()

    @_helpers.copy_docstring(credentials.Credentials)
    def refresh(self, request):
        # pylint: disable=unused-argument
        token, ttl = app_identity.get_access_token(
            self._scopes, self._service_account_id
        )
        expiry = datetime.datetime.utcfromtimestamp(ttl)

        self.token, self.expiry = token, expiry

    @property
    def service_account_email(self):
        """The service account email."""
        if self._service_account_id is None:
            self._service_account_id = app_identity.get_service_account_name()
        return self._service_account_id

    @property
    def requires_scopes(self):
        """Checks if the credentials requires scopes.

        Returns:
            bool: True if there are no scopes set otherwise False.
        """
        return not self._scopes

    @_helpers.copy_docstring(credentials.Scoped)
    def with_scopes(self, scopes):
        return self.__class__(
            scopes=scopes, service_account_id=self._service_account_id
        )

    @_helpers.copy_docstring(credentials.Signing)
    def sign_bytes(self, message):
        return self._signer.sign(message)

    @property
    @_helpers.copy_docstring(credentials.Signing)
    def signer_email(self):
        return self.service_account_email

    @property
    @_helpers.copy_docstring(credentials.Signing)
    def signer(self):
        return self._signer
