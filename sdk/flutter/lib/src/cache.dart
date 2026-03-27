import 'models.dart';

/// Entry in the flag cache, tracking when it was stored.
class _CacheEntry {
  final Flag flag;
  final DateTime storedAt;

  _CacheEntry(this.flag) : storedAt = DateTime.now();

  bool isExpired(Duration timeout) {
    return DateTime.now().difference(storedAt) > timeout;
  }
}

/// In-memory cache for feature flags with configurable TTL.
class FlagCache {
  final Duration timeout;
  final Map<String, _CacheEntry> _entries = {};

  FlagCache({this.timeout = const Duration(minutes: 5)});

  /// Store a flag in the cache.
  void put(Flag flag) {
    _entries[flag.key] = _CacheEntry(flag);
  }

  /// Store multiple flags in the cache.
  void putAll(List<Flag> flags) {
    for (final flag in flags) {
      put(flag);
    }
  }

  /// Retrieve a flag from the cache. Returns null if absent or expired.
  Flag? get(String key) {
    final entry = _entries[key];
    if (entry == null) return null;
    if (entry.isExpired(timeout)) {
      _entries.remove(key);
      return null;
    }
    return entry.flag;
  }

  /// Return all cached flags that have not expired.
  List<Flag> getAll() {
    _evictExpired();
    return _entries.values.map((e) => e.flag).toList();
  }

  /// Remove a flag from the cache.
  void remove(String key) {
    _entries.remove(key);
  }

  /// Clear all cached flags.
  void clear() {
    _entries.clear();
  }

  /// Number of entries currently in the cache (excluding expired).
  int get length {
    _evictExpired();
    return _entries.length;
  }

  /// Whether the cache contains a non-expired entry for the key.
  bool containsKey(String key) {
    return get(key) != null;
  }

  void _evictExpired() {
    _entries.removeWhere((_, entry) => entry.isExpired(timeout));
  }
}
