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

"""OAuth 2.0 Credentials.

This module provides credentials based on OAuth 2.0 access and refresh tokens.
These credentials usually access resources on behalf of a user (resource
owner).

Specifically, this is intended to use access tokens acquired using the
`Authorization Code grant`_ and can refresh those tokens using a
optional `refresh token`_.

Obtaining the initial access and refresh token is outside of the scope of this
module. Consult `rfc6749 section 4.1`_ for complete details on the
Authorization Code grant flow.

.. _Authorization Code grant: https://tools.ietf.org/html/rfc6749#section-1.3.1
.. _refresh token: https://tools.ietf.org/html/rfc6749#section-6
.. _rfc6749 section 4.1: https://tools.ietf.org/html/rfc6749#section-4.1
"""

import io
import json

import six

from google.auth import _cloud_sdk
from google.auth import _helpers
from google.auth import credentials
from google.auth import exceptions
from google.oauth2 import _client


# The Google OAuth 2.0 token endpoint. Used for authorized user credentials.
_GOOGLE_OAUTH2_TOKEN_ENDPOINT = "https://oauth2.googleapis.com/token"


class Credentials(credentials.ReadOnlyScoped, credentials.Credentials):
    """Credentials using OAuth 2.0 access and refresh tokens.

    The credentials are considered immutable. If you want to modify the
    quota project, use :meth:`with_quota_project` or ::

        credentials = credentials.with_quota_project('myproject-123)
    """

    def __init__(
        self,
        token,
        refresh_token=None,
        id_token=None,
        token_uri=None,
        client_id=None,
        client_secret=None,
        scopes=None,
        quota_project_id=None,
    ):
        """
        Args:
            token (Optional(str)): The OAuth 2.0 access token. Can be None
                if refresh information is provided.
            refresh_token (str): The OAuth 2.0 refresh token. If specified,
                credentials can be refreshed.
            id_token (str): The Open ID Connect ID Token.
            token_uri (str): The OAuth 2.0 authorization server's token
                endpoint URI. Must be specified for refresh, can be left as
                None if the token can not be refreshed.
            client_id (str): The OAuth 2.0 client ID. Must be specified for
                refresh, can be left as None if the token can not be refreshed.
            client_secret(str): The OAuth 2.0 client secret. Must be specified
                for refresh, can be left as None if the token can not be
                refreshed.
            scopes (Sequence[str]): The scopes used to obtain authorization.
                This parameter is used by :meth:`has_scopes`. OAuth 2.0
                credentials can not request additional scopes after
                authorization. The scopes must be derivable from the refresh
                token if refresh information is provided (e.g. The refresh
                token scopes are a superset of this or contain a wild card
                scope like 'https://www.googleapis.com/auth/any-api').
            quota_project_id (Optional[str]): The project ID used for quota and billing.
                This project may be different from the project used to
                create the credentials.
        """
        super(Credentials, self).__init__()
        self.token = token
        self._refresh_token = refresh_token
        self._id_token = id_token
        self._scopes = scopes
        self._token_uri = token_uri
        self._client_id = client_id
        self._client_secret = client_secret
        self._quota_project_id = quota_project_id

    def __getstate__(self):
        """A __getstate__ method must exist for the __setstate__ to be called
        This is identical to the default implementation.
        See https://docs.python.org/3.7/library/pickle.html#object.__setstate__
        """
        return self.__dict__

    def __setstate__(self, d):
        """Credentials pickled with older versions of the class do not have
        all the attributes."""
        self.token = d.get("token")
        self.expiry = d.get("expiry")
        self._refresh_token = d.get("_refresh_token")
        self._id_token = d.get("_id_token")
        self._scopes = d.get("_scopes")
        self._token_uri = d.get("_token_uri")
        self._client_id = d.get("_client_id")
        self._client_secret = d.get("_client_secret")
        self._quota_project_id = d.get("_quota_project_id")

    @property
    def refresh_token(self):
        """Optional[str]: The OAuth 2.0 refresh token."""
        return self._refresh_token

    @property
    def token_uri(self):
        """Optional[str]: The OAuth 2.0 authorization server's token endpoint
        URI."""
        return self._token_uri

    @property
    def id_token(self):
        """Optional[str]: The Open ID Connect ID Token.

        Depending on the authorization server and the scopes requested, this
        may be populated when credentials are obtained and updated when
        :meth:`refresh` is called. This token is a JWT. It can be verified
        and decoded using :func:`google.oauth2.id_token.verify_oauth2_token`.
        """
        return self._id_token

    @property
    def client_id(self):
        """Optional[str]: The OAuth 2.0 client ID."""
        return self._client_id

    @property
    def client_secret(self):
        """Optional[str]: The OAuth 2.0 client secret."""
        return self._client_secret

    @property
    def quota_project_id(self):
        """Optional[str]: The project to use for quota and billing purposes."""
        return self._quota_project_id

    @property
    def requires_scopes(self):
        """False: OAuth 2.0 credentials have their scopes set when
        the initial token is requested and can not be changed."""
        return False

    def with_quota_project(self, quota_project_id):
        """Returns a copy of these credentials with a modified quota project

        Args:
            quota_project_id (str): The project to use for quota and
            billing purposes

        Returns:
            google.oauth2.credentials.Credentials: A new credentials instance.
        """
        return self.__class__(
            self.token,
            refresh_token=self.refresh_token,
            id_token=self.id_token,
            token_uri=self.token_uri,
            client_id=self.client_id,
            client_secret=self.client_secret,
            scopes=self.scopes,
            quota_project_id=quota_project_id,
        )

    @_helpers.copy_docstring(credentials.Credentials)
    def refresh(self, request):
        if (
            self._refresh_token is None
            or self._token_uri is None
            or self._client_id is None
            or self._client_secret is None
        ):
            raise exceptions.RefreshError(
                "The credentials do not contain the necessary fields need to "
                "refresh the access token. You must specify refresh_token, "
                "token_uri, client_id, and client_secret."
            )

        access_token, refresh_token, expiry, grant_response = _client.refresh_grant(
            request,
            self._token_uri,
            self._refresh_token,
            self._client_id,
            self._client_secret,
            self._scopes,
        )

        self.token = access_token
        self.expiry = expiry
        self._refresh_token = refresh_token
        self._id_token = grant_response.get("id_token")

        if self._scopes and "scopes" in grant_response:
            requested_scopes = frozenset(self._scopes)
            granted_scopes = frozenset(grant_response["scopes"].split())
            scopes_requested_but_not_granted = requested_scopes - granted_scopes
            if scopes_requested_but_not_granted:
                raise exceptions.RefreshError(
                    "Not all requested scopes were granted by the "
                    "authorization server, missing scopes {}.".format(
                        ", ".join(scopes_requested_but_not_granted)
                    )
                )

    @_helpers.copy_docstring(credentials.Credentials)
    def apply(self, headers, token=None):
        super(Credentials, self).apply(headers, token=token)
        if self.quota_project_id is not None:
            headers["x-goog-user-project"] = self.quota_project_id

    @classmethod
    def from_authorized_user_info(cls, info, scopes=None):
        """Creates a Credentials instance from parsed authorized user info.

        Args:
            info (Mapping[str, str]): The authorized user info in Google
                format.
            scopes (Sequence[str]): Optional list of scopes to include in the
                credentials.

        Returns:
            google.oauth2.credentials.Credentials: The constructed
                credentials.

        Raises:
            ValueError: If the info is not in the expected format.
        """
        keys_needed = set(("refresh_token", "client_id", "client_secret"))
        missing = keys_needed.difference(six.iterkeys(info))

        if missing:
            raise ValueError(
                "Authorized user info was not in the expected format, missing "
                "fields {}.".format(", ".join(missing))
            )

        return cls(
            None,  # No access token, must be refreshed.
            refresh_token=info["refresh_token"],
            token_uri=_GOOGLE_OAUTH2_TOKEN_ENDPOINT,
            scopes=scopes,
            client_id=info["client_id"],
            client_secret=info["client_secret"],
            quota_project_id=info.get(
                "quota_project_id"
            ),  # quota project may not exist
        )

    @classmethod
    def from_authorized_user_file(cls, filename, scopes=None):
        """Creates a Credentials instance from an authorized user json file.

        Args:
            filename (str): The path to the authorized user json file.
            scopes (Sequence[str]): Optional list of scopes to include in the
                credentials.

        Returns:
            google.oauth2.credentials.Credentials: The constructed
                credentials.

        Raises:
            ValueError: If the file is not in the expected format.
        """
        with io.open(filename, "r", encoding="utf-8") as json_file:
            data = json.load(json_file)
            return cls.from_authorized_user_info(data, scopes)

    def to_json(self, strip=None):
        """Utility function that creates a JSON representation of a Credentials
        object.

        Args:
            strip (Sequence[str]): Optional list of members to exclude from the
                                   generated JSON.

        Returns:
            str: A JSON representation of this instance. When converted into
            a dictionary, it can be passed to from_authorized_user_info()
            to create a new credential instance.
        """
        prep = {
            "token": self.token,
            "refresh_token": self.refresh_token,
            "token_uri": self.token_uri,
            "client_id": self.client_id,
            "client_secret": self.client_secret,
            "scopes": self.scopes,
        }

        # Remove empty entries
        prep = {k: v for k, v in prep.items() if v is not None}

        # Remove entries that explicitely need to be removed
        if strip is not None:
            prep = {k: v for k, v in prep.items() if k not in strip}

        return json.dumps(prep)


class UserAccessTokenCredentials(credentials.Credentials):
    """Access token credentials for user account.

    Obtain the access token for a given user account or the current active
    user account with the ``gcloud auth print-access-token`` command.

    Args:
        account (Optional[str]): Account to get the access token for. If not
            specified, the current active account will be used.
    """

    def __init__(self, account=None):
        super(UserAccessTokenCredentials, self).__init__()
        self._account = account

    def with_account(self, account):
        """Create a new instance with the given account.

        Args:
            account (str): Account to get the access token for.

        Returns:
            google.oauth2.credentials.UserAccessTokenCredentials: The created
                credentials with the given account.
        """
        return self.__class__(account=account)

    def refresh(self, request):
        """Refreshes the access token.

        Args:
            request (google.auth.transport.Request): This argument is required
                by the base class interface but not used in this implementation,
                so just set it to `None`.

        Raises:
            google.auth.exceptions.UserAccessTokenError: If the access token
                refresh failed.
        """
        self.token = _cloud_sdk.get_auth_access_token(self._account)

    @_helpers.copy_docstring(credentials.Credentials)
    def before_request(self, request, method, url, headers):
        self.refresh(request)
        self.apply(headers)
