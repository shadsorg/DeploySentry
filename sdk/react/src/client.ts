import type {
  ApiFlagResponse,
  Flag,
  FlagDetail,
  FlagMetadata,
  UserContext,
} from './types';

/** Listener callback invoked whenever the flag store is updated. */
export type FlagChangeListener = () => void;

/**
 * Normalise a raw API response into the public {@link Flag} shape.
 */
function toFlag(raw: ApiFlagResponse): Flag {
  const meta = raw.metadata ?? {};
  return {
    key: raw.key,
    name: raw.name ?? raw.key,
    category: meta.category ?? 'feature',
    purpose: meta.purpose ?? '',
    owners: meta.owners ?? [],
    isPermanent: meta.isPermanent ?? false,
    expiresAt: meta.expiresAt,
    enabled: raw.enabled,
    value: raw.value,
    tags: meta.tags ?? [],
  };
}

/**
 * Lightweight HTTP + SSE client that manages the local flag store.
 *
 * This client is designed for browser environments. It uses the native
 * `fetch` and `EventSource` APIs, so no polyfills are required in modern
 * browsers.
 */
export class DeploySentryClient {
  private readonly apiKey: string;
  private readonly baseURL: string;
  private readonly environment: string;
  private readonly project: string;
  private readonly sessionId: string | undefined;
  private user: UserContext | undefined;
  private readonly sessionId: string | undefined;

  /** In-memory flag store keyed by flag key. */
  private readonly flags = new Map<string, Flag>();

  /** Subscribed listeners notified on any flag change. */
  private readonly listeners = new Set<FlagChangeListener>();

  /** Active SSE connection, if any. */
  private eventSource: EventSource | null = null;

  /** Whether the initial fetch has completed at least once. */
  private initialised = false;

  /** Whether the client has been destroyed. */
  private destroyed = false;

  /** Retry state for SSE reconnection. */
  private sseRetryTimer: ReturnType<typeof setTimeout> | null = null;
  private sseRetryCount = 0;
  private static readonly SSE_MAX_RETRY_DELAY_MS = 30_000;

  constructor(options: {
    apiKey: string;
    baseURL: string;
    environment: string;
    project: string;
    user?: UserContext;
    sessionId?: string;
  }) {
    this.apiKey = options.apiKey;
    this.baseURL = options.baseURL.replace(/\/+$/, '');
    this.environment = options.environment;
    this.project = options.project;
    this.sessionId = options.sessionId;
    this.user = options.user;
    this.sessionId = options.sessionId;
  }

  // ---------------------------------------------------------------------------
  // Public API
  // ---------------------------------------------------------------------------

  /** Fetch all flags from the API and start listening for SSE updates. */
  async init(): Promise<void> {
    await this.fetchFlags();
    this.connectSSE();
  }

  /** Stop the SSE connection and release resources. */
  destroy(): void {
    this.destroyed = true;
    this.disconnectSSE();
    this.listeners.clear();
  }

  /** Clear the local flag store and re-fetch all flags from the API. */
  async refreshSession(): Promise<void> {
    this.flags.clear();
    await this.fetchFlags();
  }

  /** Update the user context and re-fetch flags. */
  async identify(user: UserContext | undefined): Promise<void> {
    this.user = user;
    await this.fetchFlags();
  }

  /**
   * Clear the local flag store and re-fetch all flags from the API.
   * Useful when a new session starts and fresh flag state is required.
   */
  async refreshSession(): Promise<void> {
    this.flags.clear();
    await this.fetchFlags();
  }

  /** Subscribe to flag changes. Returns an unsubscribe function. */
  subscribe(listener: FlagChangeListener): () => void {
    this.listeners.add(listener);
    return () => {
      this.listeners.delete(listener);
    };
  }

  /** True once the first successful fetch has completed. */
  get isInitialised(): boolean {
    return this.initialised;
  }

  /** Return the stored {@link Flag} for the given key, or `undefined`. */
  getFlag(key: string): Flag | undefined {
    return this.flags.get(key);
  }

  /** Return all stored flags. */
  getAllFlags(): Flag[] {
    return Array.from(this.flags.values());
  }

  /** Build a {@link FlagMetadata} object for a stored flag. */
  getFlagMetadata(key: string): FlagMetadata | undefined {
    const flag = this.flags.get(key);
    if (!flag) return undefined;
    return {
      category: flag.category,
      purpose: flag.purpose,
      owners: flag.owners,
      isPermanent: flag.isPermanent,
      expiresAt: flag.expiresAt,
      tags: flag.tags,
    };
  }

  // ---------------------------------------------------------------------------
  // Typed evaluation methods
  // ---------------------------------------------------------------------------

  /**
   * Evaluate a boolean flag from the in-memory store.
   *
   * No API call is made -- the value comes from the flags already fetched
   * via {@link init} and kept up-to-date by SSE.
   */
  boolValue(key: string, defaultValue: boolean): boolean {
    const flag = this.flags.get(key);
    if (!flag || !flag.enabled) return defaultValue;
    if (typeof flag.value === 'boolean') return flag.value;
    if (flag.value === 'true') return true;
    if (flag.value === 'false') return false;
    return defaultValue;
  }

  /**
   * Evaluate a string flag from the in-memory store.
   */
  stringValue(key: string, defaultValue: string): string {
    const flag = this.flags.get(key);
    if (!flag || !flag.enabled) return defaultValue;
    if (typeof flag.value === 'string') return flag.value;
    if (flag.value != null) return String(flag.value);
    return defaultValue;
  }

  /**
   * Evaluate a number flag from the in-memory store.
   *
   * TypeScript has no separate `int` type -- this returns `number` and is
   * the idiomatic equivalent of `intValue` / `floatValue` in other SDKs.
   */
  numberValue(key: string, defaultValue: number): number {
    const flag = this.flags.get(key);
    if (!flag || !flag.enabled) return defaultValue;
    if (typeof flag.value === 'number') return flag.value;
    if (typeof flag.value === 'string') {
      const parsed = Number(flag.value);
      if (!Number.isNaN(parsed)) return parsed;
    }
    return defaultValue;
  }

  /**
   * Evaluate a JSON (object) flag from the in-memory store.
   */
  jsonValue<T extends object = object>(key: string, defaultValue: T): T {
    const flag = this.flags.get(key);
    if (!flag || !flag.enabled) return defaultValue;
    if (typeof flag.value === 'object' && flag.value !== null) return flag.value as T;
    if (typeof flag.value === 'string') {
      try {
        return JSON.parse(flag.value) as T;
      } catch {
        return defaultValue;
      }
    }
    return defaultValue;
  }

  /**
   * Return the full evaluation detail for a flag from the in-memory store.
   *
   * Returns `{ value, enabled, metadata, loading }` matching the
   * {@link FlagDetail} interface used by the `useFlagDetail` hook.
   */
  detail(key: string): FlagDetail {
    const flag = this.flags.get(key);
    const loading = !this.initialised;

    if (!flag) {
      return {
        value: undefined,
        enabled: false,
        metadata: {
          category: 'feature',
          purpose: '',
          owners: [],
          isPermanent: false,
          tags: [],
        },
        loading,
      };
    }

    return {
      value: flag.value,
      enabled: flag.enabled,
      metadata: {
        category: flag.category,
        purpose: flag.purpose,
        owners: flag.owners,
        isPermanent: flag.isPermanent,
        expiresAt: flag.expiresAt,
        tags: flag.tags,
      },
      loading,
    };
  }

  // ---------------------------------------------------------------------------
  // HTTP
  // ---------------------------------------------------------------------------

  private get headers(): Record<string, string> {
    const h: Record<string, string> = {
      Authorization: `ApiKey ${this.apiKey}`,
      'Content-Type': 'application/json',
      Accept: 'application/json',
    };
    if (this.sessionId) {
      h['X-DeploySentry-Session'] = this.sessionId;
    }
    return h;
  }

  private buildQueryParams(): URLSearchParams {
    const params = new URLSearchParams({
      environment: this.environment,
      project: this.project,
    });
    if (this.user?.id) {
      params.set('userId', this.user.id);
    }
    if (this.user?.attributes) {
      params.set('attributes', JSON.stringify(this.user.attributes));
    }
    return params;
  }

  private async fetchFlags(): Promise<void> {
    const url = `${this.baseURL}/v1/flags?${this.buildQueryParams().toString()}`;

    const response = await fetch(url, {
      method: 'GET',
      headers: this.headers,
    });

    if (!response.ok) {
      const body = await response.text().catch(() => '');
      throw new Error(
        `DeploySentry: failed to fetch flags (${response.status}): ${body}`,
      );
    }

    const data: ApiFlagResponse[] = await response.json();
    this.flags.clear();
    for (const raw of data) {
      this.flags.set(raw.key, toFlag(raw));
    }

    this.initialised = true;
    this.emit();
  }

  // ---------------------------------------------------------------------------
  // SSE
  // ---------------------------------------------------------------------------

  private connectSSE(): void {
    if (this.destroyed) return;
    if (typeof EventSource === 'undefined') return; // SSR guard

    const params = this.buildQueryParams();
    const url = `${this.baseURL}/v1/flags/stream?${params.toString()}`;

    // EventSource does not support custom headers natively. We pass the
    // API key as a query parameter for SSE connections.
    const sseUrl = new URL(url);
    sseUrl.searchParams.set('token', this.apiKey);

    const es = new EventSource(sseUrl.toString());

    es.addEventListener('flag.updated', (event: MessageEvent) => {
      this.handleSSEMessage(event);
    });

    es.addEventListener('flag.created', (event: MessageEvent) => {
      this.handleSSEMessage(event);
    });

    es.addEventListener('flag.deleted', (event: MessageEvent) => {
      try {
        const data = JSON.parse(event.data);
        if (data?.flag?.key) {
          this.flags.delete(data.flag.key);
          this.emit();
        }
      } catch {
        // Ignore malformed messages.
      }
    });

    es.addEventListener('message', (event: MessageEvent) => {
      this.handleSSEMessage(event);
    });

    es.onerror = () => {
      this.disconnectSSE();
      this.scheduleReconnect();
    };

    this.eventSource = es;
    this.sseRetryCount = 0;
  }

  private handleSSEMessage(event: MessageEvent): void {
    try {
      const data = JSON.parse(event.data);
      const raw: ApiFlagResponse | undefined = data?.flag ?? data;
      if (raw?.key) {
        this.flags.set(raw.key, toFlag(raw));
        this.emit();
      }
    } catch {
      // Ignore malformed messages.
    }
  }

  private disconnectSSE(): void {
    if (this.eventSource) {
      this.eventSource.close();
      this.eventSource = null;
    }
    if (this.sseRetryTimer !== null) {
      clearTimeout(this.sseRetryTimer);
      this.sseRetryTimer = null;
    }
  }

  private scheduleReconnect(): void {
    if (this.destroyed) return;
    const baseDelay = Math.min(
      1000 * Math.pow(2, this.sseRetryCount),
      DeploySentryClient.SSE_MAX_RETRY_DELAY_MS,
    );
    const jitter = baseDelay * 0.2 * (2 * Math.random() - 1);
    const delay = baseDelay + jitter;
    this.sseRetryCount++;
    this.sseRetryTimer = setTimeout(() => {
      this.sseRetryTimer = null;
      this.connectSSE();
    }, delay);
  }

  // ---------------------------------------------------------------------------
  // Internal
  // ---------------------------------------------------------------------------

  private emit(): void {
    for (const listener of this.listeners) {
      try {
        listener();
      } catch {
        // Listener errors must not break the notification loop.
      }
    }
  }
}
