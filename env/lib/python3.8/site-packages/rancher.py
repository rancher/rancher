# !/usr/bin/env python


import re
import requests
import collections
import hashlib
import os
import json
import time


def _prefix(cmd):
    prefix = os.path.basename(cmd.replace('-', '_'))
    for i in ['.pyc', '.py', '-cli', '-tool', '-util']:
        prefix = prefix.replace(i, '')
    return prefix.upper()


PREFIX = _prefix(__file__)
CACHE_DIR = '~/.' + PREFIX.lower()
TIME = not os.environ.get('TIME_API') is None
DEFAULT_TIMEOUT = 45

LIST = 'list-'
CREATE = 'create-'
UPDATE = 'update-'
DELETE = 'delete-'
ACTION = 'action-'
TRIM = True
JSON = False

GET_METHOD = 'GET'
POST_METHOD = 'POST'
PUT_METHOD = 'PUT'
DELETE_METHOD = 'DELETE'

HEADERS = {'Accept': 'application/json'}

LIST_METHODS = {'__iter__': True, '__len__': True, '__getitem__': True}


def echo(fn):
    def wrapped(*args, **kw):
        ret = fn(*args, **kw)
        print(fn.__name__, repr(ret))
        return ret
    return wrapped


def timed_url(fn):
    def wrapped(*args, **kw):
        if TIME:
            start = time.time()
            ret = fn(*args, **kw)
            delta = time.time() - start
            print(delta, args[1], fn.__name__)
            return ret
        else:
            return fn(*args, **kw)
    return wrapped


class RestObject(object):
    def __init__(self):
        pass

    def __str__(self):
        return self.__repr__()

    def __repr__(self):
        data = {}
        for k, v in self.__dict__.items():
            if self._is_public(k, v):
                data[k] = v
        return repr(data)

    def __getattr__(self, k):
        if self._is_list() and k in LIST_METHODS:
            return getattr(self.data, k)
        return getattr(self.__dict__, k)

    def __getitem__(self, key):
        return self.__dict__[key]

    def __iter__(self):
        if self._is_list():
            return iter(self.data)
        else:
            data = {}
            for k, v in self.__dict__.items():
                if self._is_public(k, v):
                    data[k] = v
            return iter(data.keys())

    def __len__(self):
        if self._is_list():
            return len(self.data)
        else:
            data = {}
            for k, v in self.__dict__.items():
                if self._is_public(k, v):
                    data[k] = v
            return len(data)

    @staticmethod
    def _is_public(k, v):
        return not callable(v)

    def _is_list(self):
        return 'data' in self.__dict__ and isinstance(self.data, list)

    def data_dict(self):
        data = {}
        for k, v in self.__dict__.items():
            if self._is_public(k, v):
                data[k] = v
        return data


class Schema(object):
    def __init__(self, text, obj):
        self.text = text
        self.types = {}
        for t in obj:
            if t.type != 'schema':
                continue

            self.types[t.id] = t
            t.creatable = False
            try:
                if POST_METHOD in t.collectionMethods:
                    t.creatable = True
            except AttributeError:
                pass

            t.updatable = False
            try:
                if PUT_METHOD in t.resourceMethods:
                    t.updatable = True
            except AttributeError:
                pass

            t.deletable = False
            try:
                if DELETE_METHOD in t.resourceMethods:
                    t.deletable = True
            except AttributeError:
                pass

            t.listable = False
            try:
                if GET_METHOD in t.collectionMethods:
                    t.listable = True
            except AttributeError:
                pass

            if not hasattr(t, 'collectionFilters'):
                t.collectionFilters = {}

    def __str__(self):
        return str(self.text)

    def __repr(self):
        return repr(self.text)


class ApiError(Exception):
    def __init__(self, obj):
        self.error = obj
        try:
            msg = '{} : {}\n\t{}'.format(obj.code, obj.message, obj)
            super(ApiError, self).__init__(self, msg)
        except Exception:
            super(ApiError, self).__init__(self, 'API Error')


class ClientApiError(Exception):
    pass


class Client(object):
    def __init__(self, access_key=None, secret_key=None, url=None, cache=False,
                 cache_time=86400, strict=False, headers=None, token=None,
                 verify=True, **kw):
        if verify == 'False':
            verify = False
        self._headers = HEADERS.copy()
        if headers is not None:
            for k, v in headers.items():
                self._headers[k] = v
        if token is not None:
            self.token = token
            self._headers['Authorization'] = 'Bearer ' + token
        self._access_key = access_key
        self._secret_key = secret_key
        if self._access_key is None:
            self._auth = None
        else:
            self._auth = (self._access_key, self._secret_key)
        self._url = url
        self._cache = cache
        self._cache_time = cache_time
        self._strict = strict
        self.schema = None
        self._session = requests.Session()
        self._session.verify = verify

        if not self._cache_time:
            self._cache_time = 60 * 60 * 24  # 24 Hours

        self._load_schemas()

    def valid(self):
        return self._url is not None and self.schema is not None

    def object_hook(self, obj):
        if isinstance(obj, list):
            return [self.object_hook(x) for x in obj]

        if isinstance(obj, dict):
            result = RestObject()

            for k, v in obj.items():
                setattr(result, k, self.object_hook(v))

            for link in ['next', 'prev']:
                try:
                    url = getattr(result.pagination, link)
                    if url is not None:
                        setattr(result, link, lambda url=url: self._get(url))
                except AttributeError:
                    pass

            if hasattr(result, 'type') and isinstance(getattr(result, 'type'),
                                                      str):
                if hasattr(result, 'links'):
                    for link_name, link in result.links.items():
                        def cb_link(_link=link, **kw):
                            return self._get(_link, data=kw)
                        if hasattr(result, link_name):
                            setattr(result, link_name + '_link', cb_link)
                        else:
                            setattr(result, link_name, cb_link)

                if hasattr(result, 'actions'):
                    for link_name, link in result.actions.items():
                        def cb_action(_link_name=link_name, _result=result,
                                      *args, **kw):
                            return self.action(_result, _link_name,
                                               *args, **kw)
                        if hasattr(result, link_name):
                            setattr(result, link_name + '_action', cb_action)
                        else:
                            setattr(result, link_name, cb_action)

            return result

        return obj

    def object_pairs_hook(self, pairs):
        ret = collections.OrderedDict()
        for k, v in pairs:
            ret[k] = v
        return self.object_hook(ret)

    def _get(self, url, data=None):
        return self._unmarshall(self._get_raw(url, data=data))

    def _error(self, text):
        raise ApiError(self._unmarshall(text))

    @timed_url
    def _get_raw(self, url, data=None):
        r = self._get_response(url, data)
        return r.text

    def _get_response(self, url, data=None):
        r = self._session.get(url, auth=self._auth, params=data,
                              headers=self._headers)
        if r.status_code < 200 or r.status_code >= 300:
            self._error(r.text)

        return r

    @timed_url
    def _post(self, url, data=None):
        r = self._session.post(url, auth=self._auth, data=self._marshall(data),
                               headers=self._headers)
        if r.status_code < 200 or r.status_code >= 300:
            self._error(r.text)

        return self._unmarshall(r.text)

    @timed_url
    def _put(self, url, data=None):
        r = self._session.put(url, auth=self._auth, data=self._marshall(data),
                              headers=self._headers)
        if r.status_code < 200 or r.status_code >= 300:
            self._error(r.text)

        return self._unmarshall(r.text)

    @timed_url
    def _delete(self, url):
        r = self._session.delete(url, auth=self._auth, headers=self._headers)
        if r.status_code < 200 or r.status_code >= 300:
            self._error(r.text)

        return self._unmarshall(r.text)

    def _unmarshall(self, text):
        if text is None or text == '':
            return text
        obj = json.loads(text, object_hook=self.object_hook,
                         object_pairs_hook=self.object_pairs_hook)
        return obj

    def _marshall(self, obj, indent=None, sort_keys=False):
        if obj is None:
            return None
        return json.dumps(self._to_dict(obj), indent=indent, sort_keys=True)

    def _load_schemas(self, force=False):
        if self.schema and not force:
            return

        schema_text = self._get_cached_schema()

        if force or not schema_text:
            response = self._get_response(self._url)
            schema_url = response.headers.get('X-API-Schemas')
            if schema_url is not None and self._url != schema_url:
                schema_text = self._get_raw(schema_url)
            else:
                schema_text = response.text
            self._cache_schema(schema_text)

        obj = self._unmarshall(schema_text)

        schema = Schema(schema_text, obj)

        if len(schema.types) > 0:
            self._bind_methods(schema)
            self.schema = schema

    def reload_schema(self):
        self._load_schemas(force=True)

    def by_id(self, type, id, **kw):
        id = str(id)
        url = self.schema.types[type].links.collection
        if url.endswith('/'):
            url += id
        else:
            url = '/'.join([url, id])
        try:
            return self._get(url, self._to_dict(**kw))
        except ApiError as e:
            if e.error.status == 404:
                return None
            else:
                raise e

    def update_by_id(self, type, id, *args, **kw):
        url = self.schema.types[type].links.collection
        if url.endswith('/'):
            url = url + id
        else:
            url = '/'.join([url, id])

        return self._put_and_retry(url, *args, **kw)

    def update(self, obj, *args, **kw):
        url = obj.links.self
        return self._put_and_retry(url, *args, **kw)

    def _put_and_retry(self, url, *args, **kw):
        retries = kw.get('retries', 3)
        for i in range(retries):
            try:
                return self._put(url, data=self._to_dict(*args, **kw))
            except ApiError as e:
                if i == retries-1:
                    raise e
                if e.error.status == 409:
                    time.sleep(.1)
                else:
                    raise e

    def _post_and_retry(self, url, *args, **kw):
        retries = kw.get('retries', 3)
        for i in range(retries):
            try:
                return self._post(url, data=self._to_dict(*args, **kw))
            except ApiError as e:
                if i == retries-1:
                    raise e
                if e.error.status == 409:
                    time.sleep(.1)
                else:
                    raise e

    def _validate_list(self, type, **kw):
        if not self._strict:
            return

        collection_filters = self.schema.types[type].collectionFilters

        for k in kw:
            if hasattr(collection_filters, k):
                return

            for filter_name, filter_value in collection_filters.items():
                for m in filter_value.modifiers:
                    if k == '_'.join([filter_name, m]):
                        return

            raise ClientApiError(k + ' is not searchable field')

    def list(self, type, **kw):
        if type not in self.schema.types:
            raise ClientApiError(type + ' is not a valid type')

        self._validate_list(type, **kw)
        collection_url = self.schema.types[type].links.collection
        return self._get(collection_url, data=self._to_dict(**kw))

    def reload(self, obj):
        return self.by_id(obj.type, obj.id)

    def create(self, type, *args, **kw):
        collection_url = self.schema.types[type].links.collection
        return self._post(collection_url, data=self._to_dict(*args, **kw))

    def delete(self, *args):
        for i in args:
            if isinstance(i, RestObject):
                return self._delete(i.links.self)

    def action(self, obj, action_name, *args, **kw):
        url = getattr(obj.actions, action_name)
        return self._post_and_retry(url, *args, **kw)

    def _is_list(self, obj):
        if isinstance(obj, list):
            return True

        if isinstance(obj, RestObject) and 'type' in obj.__dict__ and \
                obj.type == 'collection':
            return True

        return False

    def _to_value(self, value):
        if isinstance(value, dict):
            ret = {}
            for k, v in value.items():
                ret[k] = self._to_value(v)
            return ret

        if isinstance(value, list):
            ret = []
            for v in value:
                ret.append(self._to_value(v))
            return ret

        if isinstance(value, RestObject):
            ret = {}
            for k, v in vars(value).items():
                if not k.startswith('_') and \
                        not isinstance(v, RestObject) and not callable(v):
                    ret[k] = self._to_value(v)
                elif not k.startswith('_') and isinstance(v, RestObject):
                    ret[k] = self._to_dict(v)
            return ret

        return value

    def _to_dict(self, *args, **kw):
        if len(kw) == 0 and len(args) == 1 and self._is_list(args[0]):
            ret = []
            for i in args[0]:
                ret.append(self._to_dict(i))
            return ret

        ret = {}

        for i in args:
            value = self._to_value(i)
            if isinstance(value, dict):
                for k, v in value.items():
                    ret[k] = v

        for k, v in kw.items():
            ret[k] = self._to_value(v)

        return ret

    @staticmethod
    def _type_name_variants(name):
        ret = [name]
        python_name = re.sub(r'([a-z])([A-Z])', r'\1_\2', name)
        if python_name != name:
            ret.append(python_name.lower())

        return ret

    def _bind_methods(self, schema):
        bindings = [
            ('list', 'collectionMethods', GET_METHOD, self.list),
            ('by_id', 'collectionMethods', GET_METHOD, self.by_id),
            ('update_by_id', 'resourceMethods', PUT_METHOD, self.update_by_id),
            ('create', 'collectionMethods', POST_METHOD, self.create)
        ]

        for type_name, typ in schema.types.items():
            for name_variant in self._type_name_variants(type_name):
                for method_name, type_collection, test_method, m in bindings:
                    # double lambda for lexical binding hack, I'm sure there's
                    # a better way to do this
                    def cb_bind(type_name=type_name, method=m):
                        def _cb(*args, **kw):
                            return method(type_name, *args, **kw)
                        return _cb
                    if test_method in getattr(typ, type_collection, []):
                        setattr(self, '_'.join([method_name, name_variant]),
                                cb_bind())

    def _get_schema_hash(self):
        h = hashlib.new('sha1')
        h.update(self._url)
        if self._access_key is not None:
            h.update(self._access_key)
        return h.hexdigest()

    def _get_cached_schema_file_name(self):
        if not self._cache:
            return None

        h = self._get_schema_hash()

        cachedir = os.path.expanduser(CACHE_DIR)
        if not cachedir:
            return None

        if not os.path.exists(cachedir):
            os.mkdir(cachedir)

        return os.path.join(cachedir, 'schema-' + h + '.json')

    def _cache_schema(self, text):
        cached_schema = self._get_cached_schema_file_name()

        if not cached_schema:
            return None

        with open(cached_schema, 'w') as f:
            f.write(text)

    def _get_cached_schema(self):
        if not self._cache:
            return None

        cached_schema = self._get_cached_schema_file_name()

        if not cached_schema:
            return None

        if os.path.exists(cached_schema):
            mod_time = os.path.getmtime(cached_schema)
            if time.time() - mod_time < self._cache_time:
                with open(cached_schema) as f:
                    data = f.read()
                return data

        return None

    def wait_success(self, obj, timeout=-1):
        obj = self.wait_transitioning(obj, timeout)
        if obj.transitioning != 'no':
            raise ClientApiError(obj.transitioningMessage)
        return obj

    def wait_transitioning(self, obj, timeout=-1, sleep=0.01):
        timeout = _get_timeout(timeout)
        start = time.time()
        obj = self.reload(obj)
        while obj.transitioning == 'yes':
            time.sleep(sleep)
            sleep *= 2
            if sleep > 2:
                sleep = 2
            obj = self.reload(obj)
            delta = time.time() - start
            if delta > timeout:
                msg = 'Timeout waiting for [{}:{}] to be done after {} seconds'
                msg = msg.format(obj.type, obj.id, delta)
                raise Exception(msg)

        return obj


def _get_timeout(timeout):
    if timeout == -1:
        return DEFAULT_TIMEOUT
    return timeout


if __name__ == '__main__':
    print("This cli has been deprecated in favor of " +
          "https://github.com/rancher/cli")
