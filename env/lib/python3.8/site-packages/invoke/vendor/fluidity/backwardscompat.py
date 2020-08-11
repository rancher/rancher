import sys

if sys.version_info >= (3,):
    def callable(obj):
        return hasattr(obj, '__call__')
else:
    callable = callable

