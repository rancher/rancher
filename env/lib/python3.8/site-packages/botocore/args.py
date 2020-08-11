# Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License"). You
# may not use this file except in compliance with the License. A copy of
# the License is located at
#
# http://aws.amazon.com/apache2.0/
#
# or in the "license" file accompanying this file. This file is
# distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
# ANY KIND, either express or implied. See the License for the specific
# language governing permissions and limitations under the License.
"""Internal module to help with normalizing botocore client args.

This module (and all function/classes within this module) should be
considered internal, and *not* a public API.

"""
import copy
import logging
import socket

import botocore.exceptions
import botocore.serialize
import botocore.utils
from botocore.signers import RequestSigner
from botocore.config import Config
from botocore.endpoint import EndpointCreator


logger = logging.getLogger(__name__)


VALID_STS_REGIONAL_ENDPOINTS_CONFIG = [
    'legacy',
    'regional',
]
LEGACY_GLOBAL_STS_REGIONS = [
    'ap-northeast-1',
    'ap-south-1',
    'ap-southeast-1',
    'ap-southeast-2',
    'aws-global',
    'ca-central-1',
    'eu-central-1',
    'eu-north-1',
    'eu-west-1',
    'eu-west-2',
    'eu-west-3',
    'sa-east-1',
    'us-east-1',
    'us-east-2',
    'us-west-1',
    'us-west-2',
]


class ClientArgsCreator(object):
    def __init__(self, event_emitter, user_agent, response_parser_factory,
                 loader, exceptions_factory, config_store):
        self._event_emitter = event_emitter
        self._user_agent = user_agent
        self._response_parser_factory = response_parser_factory
        self._loader = loader
        self._exceptions_factory = exceptions_factory
        self._config_store = config_store

    def get_client_args(self, service_model, region_name, is_secure,
                        endpoint_url, verify, credentials, scoped_config,
                        client_config, endpoint_bridge):
        final_args = self.compute_client_args(
            service_model, client_config, endpoint_bridge, region_name,
            endpoint_url, is_secure, scoped_config)

        service_name = final_args['service_name']
        parameter_validation = final_args['parameter_validation']
        endpoint_config = final_args['endpoint_config']
        protocol = final_args['protocol']
        config_kwargs = final_args['config_kwargs']
        s3_config = final_args['s3_config']
        partition = endpoint_config['metadata'].get('partition', None)
        socket_options = final_args['socket_options']

        signing_region = endpoint_config['signing_region']
        endpoint_region_name = endpoint_config['region_name']
        if signing_region is None and endpoint_region_name is None:
            signing_region, endpoint_region_name = \
                self._get_default_s3_region(service_name, endpoint_bridge)
            config_kwargs['region_name'] = endpoint_region_name

        event_emitter = copy.copy(self._event_emitter)
        signer = RequestSigner(
            service_model.service_id, signing_region,
            endpoint_config['signing_name'],
            endpoint_config['signature_version'],
            credentials, event_emitter
        )

        config_kwargs['s3'] = s3_config
        new_config = Config(**config_kwargs)
        endpoint_creator = EndpointCreator(event_emitter)

        endpoint = endpoint_creator.create_endpoint(
            service_model, region_name=endpoint_region_name,
            endpoint_url=endpoint_config['endpoint_url'], verify=verify,
            response_parser_factory=self._response_parser_factory,
            max_pool_connections=new_config.max_pool_connections,
            proxies=new_config.proxies,
            timeout=(new_config.connect_timeout, new_config.read_timeout),
            socket_options=socket_options,
            client_cert=new_config.client_cert)

        serializer = botocore.serialize.create_serializer(
            protocol, parameter_validation)
        response_parser = botocore.parsers.create_parser(protocol)
        return {
            'serializer': serializer,
            'endpoint': endpoint,
            'response_parser': response_parser,
            'event_emitter': event_emitter,
            'request_signer': signer,
            'service_model': service_model,
            'loader': self._loader,
            'client_config': new_config,
            'partition': partition,
            'exceptions_factory': self._exceptions_factory
        }

    def compute_client_args(self, service_model, client_config,
                            endpoint_bridge, region_name, endpoint_url,
                            is_secure, scoped_config):
        service_name = service_model.endpoint_prefix
        protocol = service_model.metadata['protocol']
        parameter_validation = True
        if client_config and not client_config.parameter_validation:
            parameter_validation = False
        elif scoped_config:
            raw_value = scoped_config.get('parameter_validation')
            if raw_value is not None:
                parameter_validation = botocore.utils.ensure_boolean(raw_value)

        # Override the user agent if specified in the client config.
        user_agent = self._user_agent
        if client_config is not None:
            if client_config.user_agent is not None:
                user_agent = client_config.user_agent
            if client_config.user_agent_extra is not None:
                user_agent += ' %s' % client_config.user_agent_extra

        endpoint_config = self._compute_endpoint_config(
            service_name=service_name,
            region_name=region_name,
            endpoint_url=endpoint_url,
            is_secure=is_secure,
            endpoint_bridge=endpoint_bridge,
        )
        # Create a new client config to be passed to the client based
        # on the final values. We do not want the user to be able
        # to try to modify an existing client with a client config.
        config_kwargs = dict(
            region_name=endpoint_config['region_name'],
            signature_version=endpoint_config['signature_version'],
            user_agent=user_agent)
        if client_config is not None:
            config_kwargs.update(
                connect_timeout=client_config.connect_timeout,
                read_timeout=client_config.read_timeout,
                max_pool_connections=client_config.max_pool_connections,
                proxies=client_config.proxies,
                retries=client_config.retries,
                client_cert=client_config.client_cert,
                inject_host_prefix=client_config.inject_host_prefix,
            )
        s3_config = self.compute_s3_config(scoped_config,
                                           client_config)
        return {
            'service_name': service_name,
            'parameter_validation': parameter_validation,
            'user_agent': user_agent,
            'endpoint_config': endpoint_config,
            'protocol': protocol,
            'config_kwargs': config_kwargs,
            's3_config': s3_config,
            'socket_options': self._compute_socket_options(scoped_config)
        }

    def compute_s3_config(self, scoped_config, client_config):
        s3_configuration = None

        # Check the scoped config first.
        if scoped_config is not None:
            s3_configuration = scoped_config.get('s3')
            # Until we have proper validation of the config file (including
            # nested types), we have to account for the fact that the s3
            # key could be parsed as a string, e.g 's3 = foo'.
            # In the case we'll ignore the key for now.
            if not isinstance(s3_configuration, dict):
                logger.debug("The s3 config key is not a dictionary type, "
                             "ignoring its value of: %s", s3_configuration)
                s3_configuration = None

            # Convert logic for several s3 keys in the scoped config
            # so that the various strings map to the appropriate boolean value.
            if s3_configuration:
                boolean_keys = ['use_accelerate_endpoint',
                                'use_dualstack_endpoint',
                                'payload_signing_enabled']
                s3_configuration = self._convert_config_to_bool(
                    s3_configuration, boolean_keys)

        # Next specific client config values takes precedence over
        # specific values in the scoped config.
        if client_config is not None:
            if client_config.s3 is not None:
                if s3_configuration is None:
                    s3_configuration = client_config.s3
                else:
                    # The current s3_configuration dictionary may be
                    # from a source that only should be read from so
                    # we want to be safe and just make a copy of it to modify
                    # before it actually gets updated.
                    s3_configuration = s3_configuration.copy()
                    s3_configuration.update(client_config.s3)

        return s3_configuration

    def _compute_endpoint_config(self, service_name, region_name, endpoint_url,
                                 is_secure, endpoint_bridge):
        endpoint_config = endpoint_bridge.resolve(
            service_name, region_name, endpoint_url, is_secure)
        if self._should_set_global_sts_endpoint(
                service_name, region_name, endpoint_url):
            self._set_global_sts_endpoint(endpoint_config, is_secure)
        return endpoint_config

    def _should_set_global_sts_endpoint(self, service_name, region_name,
                                        endpoint_url):
        if service_name != 'sts':
            return False
        if endpoint_url:
            return False
        return (
            self._get_sts_regional_endpoints_config() == 'legacy' and
            region_name in LEGACY_GLOBAL_STS_REGIONS
        )

    def _get_sts_regional_endpoints_config(self):
        sts_regional_endpoints_config = self._config_store.get_config_variable(
            'sts_regional_endpoints')
        if not sts_regional_endpoints_config:
            sts_regional_endpoints_config = 'legacy'
        if sts_regional_endpoints_config not in \
                VALID_STS_REGIONAL_ENDPOINTS_CONFIG:
            raise botocore.exceptions.InvalidSTSRegionalEndpointsConfigError(
                sts_regional_endpoints_config=sts_regional_endpoints_config)
        return sts_regional_endpoints_config

    def _set_global_sts_endpoint(self, endpoint_config, is_secure):
        scheme = 'https' if is_secure else 'http'
        endpoint_config['endpoint_url'] = '%s://sts.amazonaws.com' % scheme
        endpoint_config['signing_region'] = 'us-east-1'

    def _convert_config_to_bool(self, config_dict, keys):
        # Make sure any further modifications to this section of the config
        # will not affect the scoped config by making a copy of it.
        config_copy = config_dict.copy()
        present_keys = [k for k in keys if k in config_copy]
        for key in present_keys:
            config_copy[key] = botocore.utils.ensure_boolean(config_copy[key])
        return config_copy

    def _get_default_s3_region(self, service_name, endpoint_bridge):
        # If a user is providing a custom URL, the endpoint resolver will
        # refuse to infer a signing region. If we want to default to s3v4,
        # we have to account for this.
        if service_name == 's3':
            endpoint = endpoint_bridge.resolve('s3')
            return endpoint['signing_region'], endpoint['region_name']
        return None, None

    def _compute_socket_options(self, scoped_config):
        # This disables Nagle's algorithm and is the default socket options
        # in urllib3.
        socket_options = [(socket.IPPROTO_TCP, socket.TCP_NODELAY, 1)]
        if scoped_config:
            # Enables TCP Keepalive if specified in shared config file.
            if self._ensure_boolean(scoped_config.get('tcp_keepalive', False)):
                socket_options.append(
                    (socket.SOL_SOCKET, socket.SO_KEEPALIVE, 1))
        return socket_options

    def _ensure_boolean(self, val):
        if isinstance(val, bool):
            return val
        else:
            return val.lower() == 'true'
