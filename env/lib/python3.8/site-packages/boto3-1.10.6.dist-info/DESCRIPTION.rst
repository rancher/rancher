===============================
Boto 3 - The AWS SDK for Python
===============================

|Build Status| |Version| |Gitter|

Boto3 is the Amazon Web Services (AWS) Software Development Kit (SDK) for
Python, which allows Python developers to write software that makes use
of services like Amazon S3 and Amazon EC2. You can find the latest, most
up to date, documentation at our `doc site`_, including a list of
services that are supported.


.. _boto: https://docs.pythonboto.org/
.. _`doc site`: https://boto3.amazonaws.com/v1/documentation/api/latest/index.html
.. |Build Status| image:: http://img.shields.io/travis/boto/boto3/develop.svg?style=flat
    :target: https://travis-ci.org/boto/boto3
    :alt: Build Status
.. |Gitter| image:: https://badges.gitter.im/boto/boto3.svg
   :target: https://gitter.im/boto/boto3
   :alt: Gitter
.. |Downloads| image:: http://img.shields.io/pypi/dm/boto3.svg?style=flat
    :target: https://pypi.python.org/pypi/boto3/
    :alt: Downloads
.. |Version| image:: http://img.shields.io/pypi/v/boto3.svg?style=flat
    :target: https://pypi.python.org/pypi/boto3/
    :alt: Version
.. |License| image:: http://img.shields.io/pypi/l/boto3.svg?style=flat
    :target: https://github.com/boto/boto3/blob/develop/LICENSE
    :alt: License

Quick Start
-----------
First, install the library and set a default region:

.. code-block:: sh

    $ pip install boto3

Next, set up credentials (in e.g. ``~/.aws/credentials``):

.. code-block:: ini

    [default]
    aws_access_key_id = YOUR_KEY
    aws_secret_access_key = YOUR_SECRET

Then, set up a default region (in e.g. ``~/.aws/config``):

.. code-block:: ini

    [default]
    region=us-east-1

Then, from a Python interpreter:

.. code-block:: python

    >>> import boto3
    >>> s3 = boto3.resource('s3')
    >>> for bucket in s3.buckets.all():
            print(bucket.name)

Development
-----------

Getting Started
~~~~~~~~~~~~~~~
Assuming that you have Python and ``virtualenv`` installed, set up your
environment and install the required dependencies like this instead of
the ``pip install boto3`` defined above:

.. code-block:: sh

    $ git clone https://github.com/boto/boto3.git
    $ cd boto3
    $ virtualenv venv
    ...
    $ . venv/bin/activate
    $ pip install -r requirements.txt
    $ pip install -e .

Running Tests
~~~~~~~~~~~~~
You can run tests in all supported Python versions using ``tox``. By default,
it will run all of the unit and functional tests, but you can also specify your own
``nosetests`` options. Note that this requires that you have all supported
versions of Python installed, otherwise you must pass ``-e`` or run the
``nosetests`` command directly:

.. code-block:: sh

    $ tox
    $ tox -- unit/test_session.py
    $ tox -e py26,py33 -- integration/

You can also run individual tests with your default Python version:

.. code-block:: sh

    $ nosetests tests/unit

Generating Documentation
~~~~~~~~~~~~~~~~~~~~~~~~
Sphinx is used for documentation. You can generate HTML locally with the
following:

.. code-block:: sh

    $ pip install -r requirements-docs.txt
    $ cd docs
    $ make html


Getting Help
------------

We use GitHub issues for tracking bugs and feature requests and have limited
bandwidth to address them. Please use these community resources for getting
help:

* Ask a question on `Stack Overflow <https://stackoverflow.com/>`__ and tag it with `boto3 <https://stackoverflow.com/questions/tagged/boto3>`__
* Come join the AWS Python community chat on `gitter <https://gitter.im/boto/boto3>`__
* Open a support ticket with `AWS Support <https://console.aws.amazon.com/support/home#/>`__
* If it turns out that you may have found a bug, please `open an issue <https://github.com/boto/boto3/issues/new>`__


