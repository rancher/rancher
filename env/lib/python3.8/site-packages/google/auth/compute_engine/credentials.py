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

"""Google Compute Engine credentials.

This module provides authentication for application running on Google Compute
Engine using the Compute Engine metadata server.

"""

import datetime

import six

from google.auth import _helpers
from google.auth import credentials
from google.auth import exceptions
from google.auth import iam
from google.auth import jwt
from google.auth.compute_engine import _metadata
from google.oauth2 import _client


class Credentials(credentials.ReadOnlyScoped, credentials.Credentials):
    """Compute Engine Credentials.

    These credentials use the Google Compute Engine metadata server to obtain
    OAuth 2.0 access tokens associated with the instance's service account.

    For more information about Compute Engine authentication, including how
    to configure scopes, see the `Compute Engine authentication
    documentation`_.

    .. note:: Compute Engine instances can be created with scopes and therefore
        these credentials are considered to be 'scoped'. However, you can
        not use :meth:`~google.auth.credentials.ScopedCredentials.with_scopes`
        because it is not possible to change the scopes that the instance
        has. Also note that
        :meth:`~google.auth.credentials.ScopedCredentials.has_scopes` will not
        work until the credentials have been refreshed.

    .. _Compute Engine authentication documentation:
        https://cloud.google.com/compute/docs/authentication#using
    """

    def __init__(self, service_account_email="default"):
        """
        Args:
            service_account_email (str): The service account email to use, or
                'default'. A Compute Engine instance may have multiple service
                accounts.
        """
        super(Credentials, self).__init__()
        self._service_account_email = service_account_email

    def _retrieve_info(self, request):
        """Retrieve information about the service account.

        Updates the scopes and retrieves the full service account email.

        Args:
            request (google.auth.transport.Request): The object used to make
                HTTP requests.
        """
        info = _metadata.get_service_account_info(
            request, service_account=self._service_account_email
        )

        self._service_account_email = info["email"]
        self._scopes = info["scopes"]

    def refresh(self, request):
        """Refresh the access token and scopes.

        Args:
            request (google.auth.transport.Request): The object used to make
                HTTP requests.

        Raises:
            google.auth.exceptions.RefreshError: If the Compute Engine metadata
                service can't be reached if if the instance has not
                credentials.
        """
        try:
            self._retrieve_info(request)
            self.token, self.expiry = _metadata.get_service_account_token(
                request, service_account=self._service_account_email
            )
        except exceptions.TransportError as caught_exc:
            new_exc = exceptions.RefreshError(caught_exc)
            six.raise_from(new_exc, caught_exc)

    @property
    def service_account_email(self):
        """The service account email.

        .. note:: This is not guaranteed to be set until :meth:`refresh` has been
            called.
        """
        return self._service_account_email

    @property
    def requires_scopes(self):
        """False: Compute Engine credentials can not be scoped."""
        return False


_DEFAULT_TOKEN_LIFETIME_SECS = 3600  # 1 hour in seconds
_DEFAULT_TOKEN_URI = "https://www.googleapis.com/oauth2/v4/token"


class IDTokenCredentials(credentials.Credentials, credentials.Signing):
    """Open ID Connect ID Token-based service account credentials.

    These credentials relies on the default service account of a GCE instance.

    ID token can be requested from `GCE metadata server identity endpoint`_, IAM
    token endpoint or other token endpoints you specify. If metadata server
    identity endpoint is not used, the GCE instance must have been started with
    a service account that has access to the IAM Cloud API.

    .. _GCE metadata server identity endpoint:
        https://cloud.google.com/compute/docs/instances/verifying-instance-identity
    """

    def __init__(
        self,
        request,
        target_audience,
        token_uri=None,
        additional_claims=None,
        service_account_email=None,
        signer=None,
        use_metadata_identity_endpoint=False,
    ):
        """
        Args:
            request (google.auth.transport.Request): The object used to make
                HTTP requests.
            target_audience (str): The intended audience for these credentials,
                used when requesting the ID Token. The ID Token's ``aud`` claim
                will be set to this string.
            token_uri (str): The OAuth 2.0 Token URI.
            additional_claims (Mapping[str, str]): Any additional claims for
                the JWT assertion used in the authorization grant.
            service_account_email (str): Optional explicit service account to
                use to sign JWT tokens.
                By default, this is the default GCE service account.
            signer (google.auth.crypt.Signer): The signer used to sign JWTs.
                In case the signer is specified, the request argument will be
                ignored.
            use_metadata_identity_endpoint (bool): Whether to use GCE metadata
                identity endpoint. For backward compatibility the default value
                is False. If set to True, ``token_uri``, ``additional_claims``,
                ``service_account_email``, ``signer`` argument should not be set;
                otherwise ValueError will be raised.

        Raises:
            ValueError:
                If ``use_metadata_identity_endpoint`` is set to True, and one of
                ``token_uri``, ``additional_claims``, ``service_account_email``,
                 ``signer`` arguments is set.
        """
        super(IDTokenCredentials, self).__init__()

        self._use_metadata_identity_endpoint = use_metadata_identity_endpoint
        self._target_audience = target_audience

        if use_metadata_identity_endpoint:
            if token_uri or additional_claims or service_account_email or signer:
                raise ValueError(
                    "If use_metadata_identity_endpoint is set, token_uri, "
                    "additional_claims, service_account_email, signer arguments"
                    " must not be set"
                )
            self._token_uri = None
            self._additional_claims = None
            self._signer = None

        if service_account_email is None:
            sa_info = _metadata.get_service_account_info(request)
            self._service_account_email = sa_info["email"]
        else:
            self._service_account_email = service_account_email

        if not use_metadata_identity_endpoint:
            if signer is None:
                signer = iam.Signer(
                    request=request,
                    credentials=Credentials(),
                    service_account_email=self._service_account_email,
                )
            self._signer = signer
            self._token_uri = token_uri or _DEFAULT_TOKEN_URI

            if additional_claims is not None:
                self._additional_claims = additional_claims
            else:
                self._additional_claims = {}

    def with_target_audience(self, target_audience):
        """Create a copy of these credentials with the specified target
        audience.
        Args:
            target_audience (str): The intended audience for these credentials,
            used when requesting the ID Token.
        Returns:
            google.auth.service_account.IDTokenCredentials: A new credentials
                instance.
        """
        # since the signer is already instantiated,
        # the request is not needed
        if self._use_metadata_identity_endpoint:
            return self.__class__(
                None,
                target_audience=target_audience,
                use_metadata_identity_endpoint=True,
            )
        else:
            return self.__class__(
                None,
                service_account_email=self._service_account_email,
                token_uri=self._token_uri,
                target_audience=target_audience,
                additional_claims=self._additional_claims.copy(),
                signer=self.signer,
                use_metadata_identity_endpoint=False,
            )

    def _make_authorization_grant_assertion(self):
        """Create the OAuth 2.0 assertion.
        This assertion is used during the OAuth 2.0 grant to acquire an
        ID token.
        Returns:
            bytes: The authorization grant assertion.
        """
        now = _helpers.utcnow()
        lifetime = datetime.timedelta(seconds=_DEFAULT_TOKEN_LIFETIME_SECS)
        expiry = now + lifetime

        payload = {
            "iat": _helpers.datetime_to_secs(now),
            "exp": _helpers.datetime_to_secs(expiry),
            # The issuer must be the service account email.
            "iss": self.service_account_email,
            # The audience must be the auth token endpoint's URI
            "aud": self._token_uri,
            # The target audience specifies which service the ID token is
            # intended for.
            "target_audience": self._target_audience,
        }

        payload.update(self._additional_claims)

        token = jwt.encode(self._signer, payload)

        return token

    def _call_metadata_identity_endpoint(self, request):
        """Request ID token from metadata identity endpoint.

        Args:
            request (google.auth.transport.Request): The object used to make
                HTTP requests.

        Returns:
            Tuple[str, datetime.datetime]: The ID token and the expiry of the ID token.

        Raises:
            google.auth.exceptions.RefreshError: If the Compute Engine metadata
                service can't be reached or if the instance has no credentials.
            ValueError: If extracting expiry from the obtained ID token fails.
        """
        try:
            id_token = _metadata.get(
                request,
                "instance/service-accounts/default/identity?audience={}&format=full".format(
                    self._target_audience
                ),
            )
        except exceptions.TransportError as caught_exc:
            new_exc = exceptions.RefreshError(caught_exc)
            six.raise_from(new_exc, caught_exc)

        _, payload, _, _ = jwt._unverified_decode(id_token)
        return id_token, datetime.datetime.fromtimestamp(payload["exp"])

    def refresh(self, request):
        """Refreshes the ID token.

        Args:
            request (google.auth.transport.Request): The object used to make
                HTTP requests.

        Raises:
            google.auth.exceptions.RefreshError: If the credentials could
                not be refreshed.
            ValueError: If extracting expiry from the obtained ID token fails.
        """
        if self._use_metadata_identity_endpoint:
            self.token, self.expiry = self._call_metadata_identity_endpoint(request)
        else:
            assertion = self._make_authorization_grant_assertion()
            access_token, expiry, _ = _client.id_token_jwt_grant(
                request, self._token_uri, assertion
            )
            self.token = access_token
            self.expiry = expiry

    @property
    @_helpers.copy_docstring(credentials.Signing)
    def signer(self):
        return self._signer

    def sign_bytes(self, message):
        """Signs the given message.

        Args:
            message (bytes): The message to sign.

        Returns:
            bytes: The message's cryptographic signature.

        Raises:
            ValueError:
                Signer is not available if metadata identity endpoint is used.
        """
        if self._use_metadata_identity_endpoint:
            raise ValueError(
                "Signer is not available if metadata identity endpoint is used"
            )
        return self._signer.sign(message)

    @property
    def service_account_email(self):
        """The service account email."""
        return self._service_account_email

    @property
    def signer_email(self):
        return self._service_account_email
