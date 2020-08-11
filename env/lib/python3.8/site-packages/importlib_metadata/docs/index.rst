===============================
 Welcome to importlib_metadata
===============================

``importlib_metadata`` is a library which provides an API for accessing an
installed package's metadata (see :pep:`566`), such as its entry points or its top-level
name.  This functionality intends to replace most uses of ``pkg_resources``
`entry point API`_ and `metadata API`_.  Along with :mod:`importlib.resources` in
Python 3.7 and newer (backported as :doc:`importlib_resources <importlib_resources:index>` for older
versions of Python), this can eliminate the need to use the older and less
efficient ``pkg_resources`` package.

``importlib_metadata`` is a backport of Python 3.8's standard library
:doc:`importlib.metadata <library/importlib.metadata>` module for Python 2.7, and 3.4 through 3.7.  Users of
Python 3.8 and beyond are encouraged to use the standard library module.
When imported on Python 3.8 and later, ``importlib_metadata`` replaces the
DistributionFinder behavior from the stdlib, but leaves the API in tact.
Developers looking for detailed API descriptions should refer to the Python
3.8 standard library documentation.

The documentation here includes a general :ref:`usage <using>` guide.


.. toctree::
   :maxdepth: 2
   :caption: Contents:

   using.rst
   changelog (links).rst


Project details
===============

 * Project home: https://gitlab.com/python-devs/importlib_metadata
 * Report bugs at: https://gitlab.com/python-devs/importlib_metadata/issues
 * Code hosting: https://gitlab.com/python-devs/importlib_metadata.git
 * Documentation: http://importlib_metadata.readthedocs.io/


Indices and tables
==================

* :ref:`genindex`
* :ref:`modindex`
* :ref:`search`


.. _`entry point API`: https://setuptools.readthedocs.io/en/latest/pkg_resources.html#entry-points
.. _`metadata API`: https://setuptools.readthedocs.io/en/latest/pkg_resources.html#metadata-api
