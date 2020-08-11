# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at http://mozilla.org/MPL/2.0/.

FORMAT_HTML = 'html'
FORMAT_IMAGE = 'image'
FORMAT_JSON = 'json'
FORMAT_TEXT = 'text'
FORMAT_URL = 'url'


def extra(content, format, name=None, mime_type=None, extension=None):
    return {'name': name, 'format': format, 'content': content,
            'mime_type': mime_type, 'extension': extension}


def html(content):
    return extra(content, FORMAT_HTML)


def image(content, name='Image', mime_type='image/png', extension='png'):
    return extra(content, FORMAT_IMAGE, name, mime_type, extension)


def png(content, name='Image'):
    return image(content, name, mime_type='image/png', extension='png')


def jpg(content, name='Image'):
    return image(content, name, mime_type='image/jpeg', extension='jpg')


def svg(content, name='Image'):
    return image(content, name, mime_type='image/svg+xml', extension='svg')


def json(content, name='JSON'):
    return extra(content, FORMAT_JSON, name, 'application/json', 'json')


def text(content, name='Text'):
    return extra(content, FORMAT_TEXT, name, 'text/plain', 'txt')


def url(content, name='URL'):
    return extra(content, FORMAT_URL, name)
