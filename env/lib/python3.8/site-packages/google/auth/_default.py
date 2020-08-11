# Copyright 2015 Google Inc.
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

"""Application default credentials.

Implements application default credentials and project ID detection.
"""

import io
import json
import logging
import os
import warnings

import six

from google.auth import environment_vars
from google.auth import exceptions
import google.auth.transport._http_client

_LOGGER = logging.getLogger(__name__)

# Valid types accepted for file-based credentials.
_AUTHORIZED_USER_TYPE = "authorized_user"
_SERVICE_ACCOUNT_TYPE = "service_account"
_VALID_TYPES = (_AUTHORIZED_USER_TYPE, _SERVICE_ACCOUNT_TYPE)

# Help message when no credentials can be found.
_HELP_MESSAGE = """\
Could not automatically determine credentials. Please set {env} or \
explicitly create credentials and re-run the application. For more \
information, please see \
https://cloud.google.com/docs/authentication/getting-started
""".format(
    env=environment_vars.CREDENTIALS
).strip()

# Warning when using Cloud SDK user credentials
_CLOUD_SDK_CREDENTIALS_WARNING = """\
Your application has authenticated using end user credentials from Google \
Cloud SDK without a quota project. You might receive a "quota exceeded" \
or "API not enabled" error. We recommend you rerun \
`gcloud auth application-default login` and make sure a quota project is \
added. Or you can use service accounts instead. For more information \
about service accounts, see https://cloud.google.com/docs/authentication/"""


def _warn_about_problematic_credentials(credentials):
    """Determines if the credentials are problematic.

    Credentials from the Cloud SDK that are associated with Cloud SDK's project
    are problematic because they may not have APIs enabled and have limited
    quota. If this is the case, warn about it.
    """
    from google.auth import _cloud_sdk

    if credentials.client_id == _cloud_sdk.CLOUD_SDK_CLIENT_ID:
        warnings.warn(_CLOUD_SDK_CREDENTIALS_WARNING)


def load_credentials_from_file(filename, scopes=None):
    """Loads Google credentials from a file.

    The credentials file must be a service account key or stored authorized
    user credentials.

    Args:
        filename (str): The full path to the credentials file.
        scopes (Optional[Sequence[str]]): The list of scopes for the credentials. If
            specified, the credentials will automatically be scoped if
            necessary.

    Returns:
        Tuple[google.auth.credentials.Credentials, Optional[str]]: Loaded
            credentials and the project ID. Authorized user credentials do not
            have the project ID information.

    Raises:
        google.auth.exceptions.DefaultCredentialsError: if the file is in the
            wrong format or is missing.
    """
    if not os.path.exists(filename):
        raise exceptions.DefaultCredentialsError(
            "File {} was not found.".format(filename)
        )

    with io.open(filename, "r") as file_obj:
        try:
            info = json.load(file_obj)
        except ValueError as caught_exc:
            new_exc = exceptions.DefaultCredentialsError(
                "File {} is not a valid json file.".format(filename), caught_exc
            )
            six.raise_from(new_exc, caught_exc)

    # The type key should indicate that the file is either a service account
    # credentials file or an authorized user credentials file.
    credential_type = info.get("type")

    if credential_type == _AUTHORIZED_USER_TYPE:
        from google.oauth2 import credentials

        try:
            credentials = credentials.Credentials.from_authorized_user_info(
                info, scopes=scopes
            )
        except ValueError as caught_exc:
            msg = "Failed to load authorized user credentials from {}".format(filename)
            new_exc = exceptions.DefaultCredentialsError(msg, caught_exc)
            six.raise_from(new_exc, caught_exc)
        if not credentials.quota_project_id:
            _warn_about_problematic_credentials(credentials)
        return credentials, None

    elif credential_type == _SERVICE_ACCOUNT_TYPE:
        from google.oauth2 import service_account

        try:
            credentials = service_account.Credentials.from_service_account_info(
                info, scopes=scopes
            )
        except ValueError as caught_exc:
            msg = "Failed to load service account credentials from {}".format(filename)
            new_exc = exceptions.DefaultCredentialsError(msg, caught_exc)
            six.raise_from(new_exc, caught_exc)
        return credentials, info.get("project_id")

    else:
        raise exceptions.DefaultCredentialsError(
            "The file {file} does not have a valid type. "
            "Type is {type}, expected one of {valid_types}.".format(
                file=filename, type=credential_type, valid_types=_VALID_TYPES
            )
        )


def _get_gcloud_sdk_credentials():
    """Gets the credentials and project ID from the Cloud SDK."""
    from google.auth import _cloud_sdk

    # Check if application default credentials exist.
    credentials_filename = _cloud_sdk.get_application_default_credentials_path()

    if not os.path.isfile(credentials_filename):
        return None, None

    credentials, project_id = load_credentials_from_file(credentials_filename)

    if not project_id:
        project_id = _cloud_sdk.get_project_id()

    return credentials, project_id


def _get_explicit_environ_credentials():
    """Gets credentials from the GOOGLE_APPLICATION_CREDENTIALS environment
    variable."""
    explicit_file = os.environ.get(environment_vars.CREDENTIALS)

    if explicit_file is not None:
        credentials, project_id = load_credentials_from_file(
            os.environ[environment_vars.CREDENTIALS]
        )

        return credentials, project_id

    else:
        return None, None


def _get_gae_credentials():
    """Gets Google App Engine App Identity credentials and project ID."""
    # While this library is normally bundled with app_engine, there are
    # some cases where it's not available, so we tolerate ImportError.
    try:
        import google.auth.app_engine as app_engine
    except ImportError:
        return None, None

    try:
        credentials = app_engine.Credentials()
        project_id = app_engine.get_project_id()
        return credentials, project_id
    except EnvironmentError:
        return None, None


def _get_gce_credentials(request=None):
    """Gets credentials and project ID from the GCE Metadata Service."""
    # Ping requires a transport, but we want application default credentials
    # to require no arguments. So, we'll use the _http_client transport which
    # uses http.client. This is only acceptable because the metadata server
    # doesn't do SSL and never requires proxies.

    # While this library is normally bundled with compute_engine, there are
    # some cases where it's not available, so we tolerate ImportError.
    try:
        from google.auth import compute_engine
        from google.auth.compute_engine import _metadata
    except ImportError:
        return None, None

    if request is None:
        request = google.auth.transport._http_client.Request()

    if _metadata.ping(request=request):
        # Get the project ID.
        try:
            project_id = _metadata.get_project_id(request=request)
        except exceptions.TransportError:
            project_id = None

        return compute_engine.Credentials(), project_id
    else:
        return None, None


def default(scopes=None, request=None):
    """Gets the default credentials for the current environment.

    `Application Default Credentials`_ provides an easy way to obtain
    credentials to call Google APIs for server-to-server or local applications.
    This function acquires credentials from the environment in the following
    order:

    1. If the environment variable ``GOOGLE_APPLICATION_CREDENTIALS`` is set
       to the path of a valid service account JSON private key file, then it is
       loaded and returned. The project ID returned is the project ID defined
       in the service account file if available (some older files do not
       contain project ID information).
    2. If the `Google Cloud SDK`_ is installed and has application default
       credentials set they are loaded and returned.

       To enable application default credentials with the Cloud SDK run::

            gcloud auth application-default login

       If the Cloud SDK has an active project, the project ID is returned. The
       active project can be set using::

            gcloud config set project

    3. If the application is running in the `App Engine standard environment`_
       then the credentials and project ID from the `App Identity Service`_
       are used.
    4. If the application is running in `Compute Engine`_ or the
       `App Engine flexible environment`_ then the credentials and project ID
       are obtained from the `Metadata Service`_.
    5. If no credentials are found,
       :class:`~google.auth.exceptions.DefaultCredentialsError` will be raised.

    .. _Application Default Credentials: https://developers.google.com\
            /identity/protocols/application-default-credentials
    .. _Google Cloud SDK: https://cloud.google.com/sdk
    .. _App Engine standard environment: https://cloud.google.com/appengine
    .. _App Identity Service: https://cloud.google.com/appengine/docs/python\
            /appidentity/
    .. _Compute Engine: https://cloud.google.com/compute
    .. _App Engine flexible environment: https://cloud.google.com\
            /appengine/flexible
    .. _Metadata Service: https://cloud.google.com/compute/docs\
            /storing-retrieving-metadata

    Example::

        import google.auth

        credentials, project_id = google.auth.default()

    Args:
        scopes (Sequence[str]): The list of scopes for the credentials. If
            specified, the credentials will automatically be scoped if
            necessary.
        request (google.auth.transport.Request): An object used to make
            HTTP requests. This is used to detect whether the application
            is running on Compute Engine. If not specified, then it will
            use the standard library http client to make requests.

    Returns:
        Tuple[~google.auth.credentials.Credentials, Optional[str]]:
            the current environment's credentials and project ID. Project ID
            may be None, which indicates that the Project ID could not be
            ascertained from the environment.

    Raises:
        ~google.auth.exceptions.DefaultCredentialsError:
            If no credentials were found, or if the credentials found were
            invalid.
    """
    from google.auth.credentials import with_scopes_if_required

    explicit_project_id = os.environ.get(
        environment_vars.PROJECT, os.environ.get(environment_vars.LEGACY_PROJECT)
    )

    checkers = (
        _get_explicit_environ_credentials,
        _get_gcloud_sdk_credentials,
        _get_gae_credentials,
        lambda: _get_gce_credentials(request),
    )

    for checker in checkers:
        credentials, project_id = checker()
        if credentials is not None:
            credentials = with_scopes_if_required(credentials, scopes)
            effective_project_id = explicit_project_id or project_id
            if not effective_project_id:
                _LOGGER.warning(
                    "No project ID could be determined. Consider running "
                    "`gcloud config set project` or setting the %s "
                    "environment variable",
                    environment_vars.PROJECT,
                )
            return credentials, effective_project_id

    raise exceptions.DefaultCredentialsError(_HELP_MESSAGE)
