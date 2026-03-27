"""In-memory TTL cache for flag values."""

from __future__ import annotations

import threading
import time
from typing import Any, Dict, Optional


class _CacheEntry:
    __slots__ = ("value", "expires_at")

    def __init__(self, value: Any, ttl: float) -> None:
        self.value = value
        self.expires_at = time.monotonic() + ttl


class TTLCache:
    """Thread-safe in-memory cache with per-entry TTL expiration.

    Parameters
    ----------
    default_ttl:
        Default time-to-live in seconds for cache entries.
    """

    def __init__(self, default_ttl: float = 30.0) -> None:
        self._default_ttl = default_ttl
        self._store: Dict[str, _CacheEntry] = {}
        self._lock = threading.Lock()

    @property
    def default_ttl(self) -> float:
        return self._default_ttl

    @default_ttl.setter
    def default_ttl(self, value: float) -> None:
        self._default_ttl = value

    def get(self, key: str) -> Optional[Any]:
        """Return the cached value for *key*, or ``None`` if missing/expired."""
        with self._lock:
            entry = self._store.get(key)
            if entry is None:
                return None
            if time.monotonic() > entry.expires_at:
                del self._store[key]
                return None
            return entry.value

    def set(self, key: str, value: Any, ttl: Optional[float] = None) -> None:
        """Store *value* under *key* with an optional custom *ttl*."""
        effective_ttl = ttl if ttl is not None else self._default_ttl
        with self._lock:
            self._store[key] = _CacheEntry(value, effective_ttl)

    def delete(self, key: str) -> None:
        """Remove *key* from the cache if present."""
        with self._lock:
            self._store.pop(key, None)

    def clear(self) -> None:
        """Remove all entries from the cache."""
        with self._lock:
            self._store.clear()

    def evict_expired(self) -> int:
        """Remove all expired entries and return the count removed."""
        now = time.monotonic()
        removed = 0
        with self._lock:
            expired_keys = [k for k, v in self._store.items() if now > v.expires_at]
            for k in expired_keys:
                del self._store[k]
                removed += 1
        return removed
