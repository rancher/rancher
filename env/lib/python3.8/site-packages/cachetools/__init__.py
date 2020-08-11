"""Extensible memoizing collections and decorators."""

from .cache import Cache
from .decorators import cached, cachedmethod
from .lfu import LFUCache
from .lru import LRUCache
from .rr import RRCache
from .ttl import TTLCache

__all__ = (
    'Cache',
    'LFUCache',
    'LRUCache',
    'RRCache',
    'TTLCache',
    'cached',
    'cachedmethod'
)

__version__ = '4.1.0'
