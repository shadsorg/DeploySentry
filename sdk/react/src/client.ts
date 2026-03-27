import type {
  ApiFlagResponse,
  Flag,
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
  private user: UserContext | undefined;

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
  }) {
    this.apiKey = options.apiKey;
    this.baseURL = options.baseURL.replace(/\/+$/, '');
    this.environment = options.environment;
    this.project = options.project;
    this.user = options.user;
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

  /** Update the user context and re-fetch flags. */
  async identify(user: UserContext | undefined): Promise<void> {
    this.user = user;
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
  // HTTP
  // ---------------------------------------------------------------------------

  private get headers(): Record<string, string> {
    return {
      Authorization: `ApiKey ${this.apiKey}`,
      'Content-Type': 'application/json',
      Accept: 'application/json',
    };
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
    const delay = Math.min(
      1000 * Math.pow(2, this.sseRetryCount),
      DeploySentryClient.SSE_MAX_RETRY_DELAY_MS,
    );
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
