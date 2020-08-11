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

"""Exceptions used in the google.auth package."""


class GoogleAuthError(Exception):
    """Base class for all google.auth errors."""


class TransportError(GoogleAuthError):
    """Used to indicate an error occurred during an HTTP request."""


class RefreshError(GoogleAuthError):
    """Used to indicate that an refreshing the credentials' access token
    failed."""


class UserAccessTokenError(GoogleAuthError):
    """Used to indicate ``gcloud auth print-access-token`` command failed."""


class DefaultCredentialsError(GoogleAuthError):
    """Used to indicate that acquiring default credentials failed."""


class MutualTLSChannelError(GoogleAuthError):
    """Used to indicate that mutual TLS channel creation is failed, or mutual
    TLS channel credentials is missing or invalid."""


class ClientCertError(GoogleAuthError):
    """Used to indicate that client certificate is missing or invalid."""
