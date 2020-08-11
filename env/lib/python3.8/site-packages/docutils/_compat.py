# $Id: _compat.py 8164 2017-08-14 11:28:48Z milde $
# Author: Georg Brandl <georg@python.org>
# Copyright: This module has been placed in the public domain.

"""
Python 2/3 compatibility definitions.

This module currently provides the following helper symbols:

* u_prefix (unicode repr prefix: 'u' in 2.x, '' in 3.x)
  (Required in docutils/test/test_publisher.py)
* BytesIO (a StringIO class that works with bytestrings)
"""

import sys

if sys.version_info < (3,0):
    u_prefix = 'u'
    from io import StringIO as BytesIO
else:
    u_prefix = b''
    # using this hack since 2to3 "fixes" the relative import
    # when using ``from io import BytesIO``
    BytesIO = __import__('io').BytesIO
