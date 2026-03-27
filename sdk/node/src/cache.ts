import { Flag } from './types';

interface CacheEntry {
  flag: Flag;
  expiresAt: number;
}

/**
 * In-memory flag cache with per-entry TTL support.
 *
 * Entries are lazily evicted on read. A periodic sweep can be triggered
 * via {@link purgeExpired} if deterministic memory reclamation is required.
 */
export class FlagCache {
  private readonly store = new Map<string, CacheEntry>();
  private readonly ttlMs: number;

  /**
   * @param ttlMs - Time-to-live for every cache entry in milliseconds.
   *                Defaults to 60 000 (1 minute).
   */
  constructor(ttlMs: number = 60_000) {
    this.ttlMs = ttlMs;
  }

  /** Store or update a single flag in the cache. */
  set(flag: Flag): void {
    this.store.set(flag.key, {
      flag,
      expiresAt: Date.now() + this.ttlMs,
    });
  }

  /** Bulk-insert flags, replacing any stale entries. */
  setMany(flags: Flag[]): void {
    for (const flag of flags) {
      this.set(flag);
    }
  }

  /** Retrieve a flag by key. Returns `undefined` if missing or expired. */
  get(key: string): Flag | undefined {
    const entry = this.store.get(key);
    if (!entry) return undefined;

    if (Date.now() > entry.expiresAt) {
      this.store.delete(key);
      return undefined;
    }

    return entry.flag;
  }

  /** Return all non-expired flags currently in the cache. */
  getAll(): Flag[] {
    const now = Date.now();
    const result: Flag[] = [];

    for (const [key, entry] of this.store) {
      if (now > entry.expiresAt) {
        this.store.delete(key);
      } else {
        result.push(entry.flag);
      }
    }

    return result;
  }

  /** Remove a single key from the cache. */
  delete(key: string): void {
    this.store.delete(key);
  }

  /** Remove all expired entries. Returns the number of entries purged. */
  purgeExpired(): number {
    const now = Date.now();
    let purged = 0;

    for (const [key, entry] of this.store) {
      if (now > entry.expiresAt) {
        this.store.delete(key);
        purged++;
      }
    }

    return purged;
  }

  /** Drop every entry from the cache. */
  clear(): void {
    this.store.clear();
  }

  /** Number of entries (including potentially expired ones). */
  get size(): number {
    return this.store.size;
  }
}
