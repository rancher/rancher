# -*- coding: utf-8 -*-
import os
import json
import logging
import requests
try:
    import urlparse
except ImportError:
    from urllib import parse as urlparse


GET = 'GET'
POST = 'POST'
DELETE = 'DELETE'
PUT = 'PUT'
REQUEST_TIMEOUT_ENV_VAR = 'PYTHON_DIGITALOCEAN_REQUEST_TIMEOUT_SEC'


class Error(Exception):
    """Base exception class for this module"""
    pass


class TokenError(Error):
    pass


class DataReadError(Error):
    pass


class JSONReadError(Error):
    pass


class NotFoundError(Error):
    pass


class BaseAPI(object):
    """
        Basic api class for
    """
    token = ""
    end_point = "https://api.digitalocean.com/v2/"

    def __init__(self, *args, **kwargs):
        self.token = ""
        self.end_point = "https://api.digitalocean.com/v2/"
        self._log = logging.getLogger(__name__)

        for attr in kwargs.keys():
            setattr(self, attr, kwargs[attr])

    def __perform_request(self, url, type=GET, params=None):
        """
            This method will perform the real request,
            in this way we can customize only the "output" of the API call by
            using self.__call_api method.
            This method will return the request object.
        """
        if params is None:
            params = {}

        if not self.token:
            raise TokenError("No token provided. Please use a valid token")

        url = urlparse.urljoin(self.end_point, url)

        # lookup table to find out the appropriate requests method,
        # headers and payload type (json or query parameters)
        identity = lambda x: x
        json_dumps = lambda x: json.dumps(x)
        lookup = {
            GET: (requests.get, {}, 'params', identity),
            POST: (requests.post, {'Content-type': 'application/json'}, 'data',
                   json_dumps),
            PUT: (requests.put, {'Content-type': 'application/json'}, 'data',
                  json_dumps),
            DELETE: (requests.delete,
                     {'content-type': 'application/json'},
                     'data', json_dumps),
        }

        requests_method, headers, payload, transform = lookup[type]
        headers.update({'Authorization': 'Bearer ' + self.token})
        kwargs = {'headers': headers, payload: transform(params)}

        timeout = self.get_timeout()
        if timeout:
            kwargs['timeout'] = timeout

        # remove token from log
        headers_str = str(headers).replace(self.token.strip(), 'TOKEN')
        self._log.debug('%s %s %s:%s %s %s' %
                        (type, url, payload, params, headers_str, timeout))

        return requests_method(url, **kwargs)

    def __deal_with_pagination(self, url, method, params, data):
        """
            Perform multiple calls in order to have a full list of elements
            when the API are "paginated". (content list is divided in more
            than one page)
        """
        all_data = data
        while data.get("links", {}).get("pages", {}).get("next"):
            url, query = data["links"]["pages"]["next"].split("?", 1)

            # Merge the query parameters
            for key, value in urlparse.parse_qs(query).items():
                params[key] = value

            data = self.__perform_request(url, method, params).json()

            # Merge the dictionaries
            for key, value in data.items():
                if isinstance(value, list) and key in all_data:
                    all_data[key] += value
                else:
                    all_data[key] = value

        return all_data

    def __init_ratelimit(self, headers):
        # Add the account requests/hour limit
        self.ratelimit_limit = headers.get('Ratelimit-Limit', None)
        # Add the account requests remaining
        self.ratelimit_remaining = headers.get('Ratelimit-Remaining', None)
        # Add the account requests limit reset time
        self.ratelimit_reset = headers.get('Ratelimit-Reset', None)

    def get_timeout(self):
        """
            Checks if any timeout for the requests to DigitalOcean is required.
            To set a timeout, use the REQUEST_TIMEOUT_ENV_VAR environment
            variable.
        """
        timeout_str = os.environ.get(REQUEST_TIMEOUT_ENV_VAR)
        if timeout_str:
            try:
                return float(timeout_str)
            except:
                self._log.error('Failed parsing the request read timeout of '
                                '"%s". Please use a valid float number!' %
                                        timeout_str)
        return None

    def get_data(self, url, type=GET, params=None):
        """
            This method is a basic implementation of __call_api that checks
            errors too. In case of success the method will return True or the
            content of the response to the request.

            Pagination is automatically detected and handled accordingly
        """
        if params is None:
            params = dict()

        # If per_page is not set, make sure it has a sane default
        if type is GET:
            params.setdefault("per_page", 200)

        req = self.__perform_request(url, type, params)
        if req.status_code == 204:
            return True

        if req.status_code == 404:
            raise NotFoundError()

        try:
            data = req.json()
        except ValueError as e:
            raise JSONReadError(
                'Read failed from DigitalOcean: %s' % str(e)
            )

        if not req.ok:
            msg = [data[m] for m in ("id", "message") if m in data][1]
            raise DataReadError(msg)

        # init request limits
        self.__init_ratelimit(req.headers)

        # If there are more elements available (total) than the elements per
        # page, try to deal with pagination. Note: Breaking the logic on
        # multiple pages,
        pages = data.get("links", {}).get("pages", {})
        if pages.get("next") and "page" not in params:
            return self.__deal_with_pagination(url, type, params, data)
        else:
            return data

    def __str__(self):
        return "<%s>" % self.__class__.__name__

    def __unicode__(self):
        return u"%s" % self.__str__()

    def __repr__(self):
        return str(self)
