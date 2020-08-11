PyJWT
=====

.. image:: https://secure.travis-ci.org/jpadilla/pyjwt.svg?branch=master
   :target: http://travis-ci.org/jpadilla/pyjwt?branch=master

.. image:: https://ci.appveyor.com/api/projects/status/h8nt70aqtwhht39t?svg=true
   :target: https://ci.appveyor.com/project/jpadilla/pyjwt

.. image:: https://img.shields.io/pypi/v/pyjwt.svg
   :target: https://pypi.python.org/pypi/pyjwt

.. image:: https://coveralls.io/repos/jpadilla/pyjwt/badge.svg?branch=master
   :target: https://coveralls.io/r/jpadilla/pyjwt?branch=master

.. image:: https://readthedocs.org/projects/pyjwt/badge/?version=latest
   :target: https://pyjwt.readthedocs.io

A Python implementation of `RFC 7519 <https://tools.ietf.org/html/rfc7519>`_. Original implementation was written by `@progrium <https://github.com/progrium>`_.

Sponsor
-------

+--------------+---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------+
| |auth0-logo| | If you want to quickly add secure token-based authentication to Python projects, feel free to check Auth0's Python SDK and free plan at `auth0.com/overview <https://auth0.com/overview?utm_source=GHsponsor&utm_medium=GHsponsor&utm_campaign=pyjwt&utm_content=auth>`_. |
+--------------+-----------------------------------------------------------------+---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------+

.. |auth0-logo| image:: https://user-images.githubusercontent.com/83319/31722733-de95bbde-b3ea-11e7-96bf-4f4e8f915588.png

Installing
----------

Install with **pip**:

.. code-block:: sh

    $ pip install PyJWT


Usage
-----

.. code:: python

    >>> import jwt
    >>> encoded = jwt.encode({'some': 'payload'}, 'secret', algorithm='HS256')
    'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzb21lIjoicGF5bG9hZCJ9.4twFt5NiznN84AWoo1d7KO1T_yoc0Z6XOpOVswacPZg'

    >>> jwt.decode(encoded, 'secret', algorithms=['HS256'])
    {'some': 'payload'}


Command line
------------

Usage::

    pyjwt [options] INPUT

Decoding examples::

    pyjwt --key=secret TOKEN
    pyjwt --no-verify TOKEN

See more options executing ``pyjwt --help``.


Documentation
-------------

View the full docs online at https://pyjwt.readthedocs.io/en/latest/


Tests
-----

You can run tests from the project root after cloning with:

.. code-block:: sh

    $ python setup.py test


