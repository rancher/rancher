import os.path
import logging
import socket
from base64 import b64encode

from urllib3 import PoolManager, ProxyManager, proxy_from_url, Timeout
from urllib3.util.retry import Retry
from urllib3.util.ssl_ import (
    ssl, OP_NO_SSLv2, OP_NO_SSLv3, OP_NO_COMPRESSION, DEFAULT_CIPHERS,
)
from urllib3.exceptions import SSLError as URLLib3SSLError
from urllib3.exceptions import ReadTimeoutError as URLLib3ReadTimeoutError
from urllib3.exceptions import ConnectTimeoutError as URLLib3ConnectTimeoutError
from urllib3.exceptions import NewConnectionError, ProtocolError, ProxyError
try:
    # Always import the original SSLContext, even if it has been patched
    from urllib3.contrib.pyopenssl import orig_util_SSLContext as SSLContext
except ImportError:
    from urllib3.util.ssl_ import SSLContext

import botocore.awsrequest
from botocore.vendored import six
from botocore.vendored.six.moves.urllib_parse import unquote
from botocore.compat import filter_ssl_warnings, urlparse
from botocore.exceptions import (
    ConnectionClosedError, EndpointConnectionError, HTTPClientError,
    ReadTimeoutError, ProxyConnectionError, ConnectTimeoutError, SSLError
)

filter_ssl_warnings()
logger = logging.getLogger(__name__)
DEFAULT_TIMEOUT = 60
MAX_POOL_CONNECTIONS = 10
DEFAULT_CA_BUNDLE = os.path.join(os.path.dirname(__file__), 'cacert.pem')

try:
    from certifi import where
except ImportError:
    def where():
        return DEFAULT_CA_BUNDLE


def get_cert_path(verify):
    if verify is not True:
        return verify

    return where()


def create_urllib3_context(ssl_version=None, cert_reqs=None,
                           options=None, ciphers=None):
    """ This function is a vendored version of the same function in urllib3

        We vendor this function to ensure that the SSL contexts we construct
        always use the std lib SSLContext instead of pyopenssl.
    """
    context = SSLContext(ssl_version or ssl.PROTOCOL_SSLv23)

    # Setting the default here, as we may have no ssl module on import
    cert_reqs = ssl.CERT_REQUIRED if cert_reqs is None else cert_reqs

    if options is None:
        options = 0
        # SSLv2 is easily broken and is considered harmful and dangerous
        options |= OP_NO_SSLv2
        # SSLv3 has several problems and is now dangerous
        options |= OP_NO_SSLv3
        # Disable compression to prevent CRIME attacks for OpenSSL 1.0+
        # (issue urllib3#309)
        options |= OP_NO_COMPRESSION

    context.options |= options

    if getattr(context, 'supports_set_ciphers', True):
        # Platform-specific: Python 2.6
        context.set_ciphers(ciphers or DEFAULT_CIPHERS)

    context.verify_mode = cert_reqs
    if getattr(context, 'check_hostname', None) is not None:
        # Platform-specific: Python 3.2
        # We do our own verification, including fingerprints and alternative
        # hostnames. So disable it here
        context.check_hostname = False
    return context


class ProxyConfiguration(object):
    """Represents a proxy configuration dictionary.

    This class represents a proxy configuration dictionary and provides utility
    functions to retreive well structured proxy urls and proxy headers from the
    proxy configuration dictionary.
    """
    def __init__(self, proxies=None):
        if proxies is None:
            proxies = {}
        self._proxies = proxies

    def proxy_url_for(self, url):
        """Retrirves the corresponding proxy url for a given url. """
        parsed_url = urlparse(url)
        proxy = self._proxies.get(parsed_url.scheme)
        if proxy:
            proxy = self._fix_proxy_url(proxy)
        return proxy

    def proxy_headers_for(self, proxy_url):
        """Retrirves the corresponding proxy headers for a given proxy url. """
        headers = {}
        username, password = self._get_auth_from_url(proxy_url)
        if username and password:
            basic_auth = self._construct_basic_auth(username, password)
            headers['Proxy-Authorization'] = basic_auth
        return headers

    def _fix_proxy_url(self, proxy_url):
        if proxy_url.startswith('http:') or proxy_url.startswith('https:'):
            return proxy_url
        elif proxy_url.startswith('//'):
            return 'http:' + proxy_url
        else:
            return 'http://' + proxy_url

    def _construct_basic_auth(self, username, password):
        auth_str = '{0}:{1}'.format(username, password)
        encoded_str = b64encode(auth_str.encode('ascii')).strip().decode()
        return 'Basic {0}'.format(encoded_str)

    def _get_auth_from_url(self, url):
        parsed_url = urlparse(url)
        try:
            return unquote(parsed_url.username), unquote(parsed_url.password)
        except (AttributeError, TypeError):
            return None, None


class URLLib3Session(object):
    """A basic HTTP client that supports connection pooling and proxies.

    This class is inspired by requests.adapters.HTTPAdapter, but has been
    boiled down to meet the use cases needed by botocore. For the most part
    this classes matches the functionality of HTTPAdapter in requests v2.7.0
    (the same as our vendored version). The only major difference of note is
    that we currently do not support sending chunked requests. While requests
    v2.7.0 implemented this themselves, later version urllib3 support this
    directly via a flag to urlopen so enabling it if needed should be trivial.
    """
    def __init__(self,
                 verify=True,
                 proxies=None,
                 timeout=None,
                 max_pool_connections=MAX_POOL_CONNECTIONS,
                 socket_options=None,
                 client_cert=None,
    ):
        self._verify = verify
        self._proxy_config = ProxyConfiguration(proxies=proxies)
        self._pool_classes_by_scheme = {
            'http': botocore.awsrequest.AWSHTTPConnectionPool,
            'https': botocore.awsrequest.AWSHTTPSConnectionPool,
        }
        if timeout is None:
            timeout = DEFAULT_TIMEOUT
        if not isinstance(timeout, (int, float)):
            timeout = Timeout(connect=timeout[0], read=timeout[1])

        self._cert_file = None
        self._key_file = None
        if isinstance(client_cert, str):
            self._cert_file = client_cert
        elif isinstance(client_cert, tuple):
            self._cert_file, self._key_file = client_cert

        self._timeout = timeout
        self._max_pool_connections = max_pool_connections
        self._socket_options = socket_options
        if socket_options is None:
            self._socket_options = []
        self._proxy_managers = {}
        self._manager = PoolManager(**self._get_pool_manager_kwargs())
        self._manager.pool_classes_by_scheme = self._pool_classes_by_scheme

    def _get_pool_manager_kwargs(self, **extra_kwargs):
        pool_manager_kwargs = {
            'strict': True,
            'timeout': self._timeout,
            'maxsize': self._max_pool_connections,
            'ssl_context': self._get_ssl_context(),
            'socket_options': self._socket_options,
            'cert_file': self._cert_file,
            'key_file': self._key_file,
        }
        pool_manager_kwargs.update(**extra_kwargs)
        return pool_manager_kwargs

    def _get_ssl_context(self):
        return create_urllib3_context()

    def _get_proxy_manager(self, proxy_url):
        if proxy_url not in self._proxy_managers:
            proxy_headers = self._proxy_config.proxy_headers_for(proxy_url)
            proxy_manager_kwargs = self._get_pool_manager_kwargs(
                proxy_headers=proxy_headers)
            proxy_manager = proxy_from_url(proxy_url, **proxy_manager_kwargs)
            proxy_manager.pool_classes_by_scheme = self._pool_classes_by_scheme
            self._proxy_managers[proxy_url] = proxy_manager

        return self._proxy_managers[proxy_url]

    def _path_url(self, url):
        parsed_url = urlparse(url)
        path = parsed_url.path
        if not path:
            path = '/'
        if parsed_url.query:
            path = path + '?' + parsed_url.query
        return path

    def _setup_ssl_cert(self, conn, url, verify):
        if url.lower().startswith('https') and verify:
            conn.cert_reqs = 'CERT_REQUIRED'
            conn.ca_certs = get_cert_path(verify)
        else:
            conn.cert_reqs = 'CERT_NONE'
            conn.ca_certs = None

    def _get_connection_manager(self, url, proxy_url=None):
        if proxy_url:
            manager = self._get_proxy_manager(proxy_url)
        else:
            manager = self._manager
        return manager

    def _get_request_target(self, url, proxy_url):
        if proxy_url and url.startswith('http:'):
            # HTTP proxies expect the request_target to be the absolute url to
            # know which host to establish a connection to
            return url
        else:
            # otherwise just set the request target to the url path
            return self._path_url(url)

    def _chunked(self, headers):
        return headers.get('Transfer-Encoding', '') == 'chunked'

    def send(self, request):
        try:
            proxy_url = self._proxy_config.proxy_url_for(request.url)
            manager = self._get_connection_manager(request.url, proxy_url)
            conn = manager.connection_from_url(request.url)
            self._setup_ssl_cert(conn, request.url, self._verify)

            request_target = self._get_request_target(request.url, proxy_url)
            urllib_response = conn.urlopen(
                method=request.method,
                url=request_target,
                body=request.body,
                headers=request.headers,
                retries=Retry(False),
                assert_same_host=False,
                preload_content=False,
                decode_content=False,
                chunked=self._chunked(request.headers),
            )

            http_response = botocore.awsrequest.AWSResponse(
                request.url,
                urllib_response.status,
                urllib_response.headers,
                urllib_response,
            )

            if not request.stream_output:
                # Cause the raw stream to be exhausted immediately. We do it
                # this way instead of using preload_content because
                # preload_content will never buffer chunked responses
                http_response.content

            return http_response
        except URLLib3SSLError as e:
            raise SSLError(endpoint_url=request.url, error=e)
        except (NewConnectionError, socket.gaierror) as e:
            raise EndpointConnectionError(endpoint_url=request.url, error=e)
        except ProxyError as e:
            raise ProxyConnectionError(proxy_url=proxy_url, error=e)
        except URLLib3ConnectTimeoutError as e:
            raise ConnectTimeoutError(endpoint_url=request.url, error=e)
        except URLLib3ReadTimeoutError as e:
            raise ReadTimeoutError(endpoint_url=request.url, error=e)
        except ProtocolError as e:
            raise ConnectionClosedError(
                error=e,
                request=request,
                endpoint_url=request.url
            )
        except Exception as e:
            message = 'Exception received when sending urllib3 HTTP request'
            logger.debug(message, exc_info=True)
            raise HTTPClientError(error=e)
