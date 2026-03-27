package io.deploysentry;

import java.time.Duration;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Collections;
import java.util.List;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;
import java.util.stream.Collectors;

/**
 * Thread-safe, TTL-aware cache for {@link Flag} instances backed by a
 * {@link ConcurrentHashMap}.
 */
public final class FlagCache {

    private final ConcurrentHashMap<String, CacheEntry> store = new ConcurrentHashMap<>();
    private final Duration ttl;

    /**
     * Creates a cache with the specified time-to-live for entries.
     *
     * @param ttl maximum age before an entry is considered stale
     */
    public FlagCache(Duration ttl) {
        this.ttl = ttl;
    }

    /**
     * Stores a flag in the cache. If the key already exists it is replaced.
     */
    public void put(String key, Flag flag) {
        store.put(key, new CacheEntry(flag, Instant.now()));
    }

    /**
     * Stores all flags, replacing any existing entries with the same keys.
     */
    public void putAll(Map<String, Flag> flags) {
        Instant now = Instant.now();
        flags.forEach((key, flag) -> store.put(key, new CacheEntry(flag, now)));
    }

    /**
     * Returns the cached flag for the given key, or {@code null} if absent or
     * expired.
     */
    public Flag get(String key) {
        CacheEntry entry = store.get(key);
        if (entry == null) {
            return null;
        }
        if (isExpired(entry)) {
            store.remove(key, entry);
            return null;
        }
        return entry.flag;
    }

    /**
     * Returns an unmodifiable snapshot of all non-expired cached flags.
     */
    public List<Flag> getAll() {
        evictExpired();
        return store.values().stream()
                .map(e -> e.flag)
                .collect(Collectors.toUnmodifiableList());
    }

    /**
     * Returns all non-expired flags whose metadata matches the given category.
     */
    public List<Flag> getByCategory(FlagCategory category) {
        evictExpired();
        return store.values().stream()
                .map(e -> e.flag)
                .filter(f -> f.getMetadata() != null && f.getMetadata().getCategory() == category)
                .collect(Collectors.toUnmodifiableList());
    }

    /**
     * Returns all non-expired flags whose metadata indicates they have passed
     * their expiration date.
     */
    public List<Flag> getExpired() {
        evictExpired();
        return store.values().stream()
                .map(e -> e.flag)
                .filter(f -> f.getMetadata() != null && f.getMetadata().isExpired())
                .collect(Collectors.toUnmodifiableList());
    }

    /**
     * Removes the entry for the given key.
     */
    public void remove(String key) {
        store.remove(key);
    }

    /**
     * Removes all entries from the cache.
     */
    public void clear() {
        store.clear();
    }

    /**
     * Returns the number of (non-expired) entries currently cached.
     */
    public int size() {
        evictExpired();
        return store.size();
    }

    // ---- internals ----

    private boolean isExpired(CacheEntry entry) {
        return Duration.between(entry.storedAt, Instant.now()).compareTo(ttl) > 0;
    }

    private void evictExpired() {
        store.entrySet().removeIf(e -> isExpired(e.getValue()));
    }

    private static final class CacheEntry {
        final Flag flag;
        final Instant storedAt;

        CacheEntry(Flag flag, Instant storedAt) {
            this.flag = flag;
            this.storedAt = storedAt;
        }
    }
}
