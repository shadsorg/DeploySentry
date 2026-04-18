import { evaluateLocal } from './local-evaluator';
import type {
  ApiFlagResponse,
  EvaluationContext,
  Flag,
  FlagConfig,
  FlagDetail,
  FlagMetadata,
  Registration,
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
  private readonly application: string;
  private readonly sessionId: string | undefined;
  private readonly mode: 'server' | 'file' | 'server-with-fallback';
  private flagConfig: FlagConfig | null = null;
  private user: UserContext | undefined;

  /** In-memory flag store keyed by flag key. */
  private readonly flags = new Map<string, Flag>();

  /** Registry of operation handlers for the register/dispatch pattern. */
  private registry: Map<string, Registration[]> = new Map();

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
    application: string;
    user?: UserContext;
    sessionId?: string;
    mode?: 'server' | 'file' | 'server-with-fallback';
    flagConfig?: FlagConfig;
  }) {
    this.apiKey = options.apiKey;
    this.baseURL = options.baseURL.replace(/\/+$/, '');
    this.environment = options.environment;
    this.project = options.project;
    this.application = options.application;
    this.sessionId = options.sessionId;
    this.mode = options.mode ?? 'server';
    this.flagConfig = options.flagConfig ?? null;
    this.user = options.user;
  }

  // ---------------------------------------------------------------------------
  // Public API
  // ---------------------------------------------------------------------------

  /** Fetch all flags from the API and start listening for SSE updates. */
  async init(): Promise<void> {
    if (this.flagConfig) {
      // Populate the in-memory store from the pre-loaded config.
      const envName = this.environment;
      for (const fc of this.flagConfig.flags) {
        const envState = fc.environments[envName];
        this.flags.set(fc.key, {
          key: fc.key,
          name: fc.name,
          category: fc.category as any,
          purpose: '',
          owners: [],
          isPermanent: fc.is_permanent,
          expiresAt: fc.expires_at,
          enabled: envState?.enabled ?? false,
          value: envState?.value ?? fc.default_value,
          tags: [],
        });
      }
      this.initialised = true;
      this.emit();
      return;
    }

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
  // Register / dispatch
  // ---------------------------------------------------------------------------

  /**
   * Register a handler for the given operation name.
   *
   * When `flagKey` is provided the handler is used only when that flag is
   * enabled. Omit `flagKey` to register the default (fallback) handler.
   */
  register<T extends (...args: any[]) => any>(
    operation: string,
    handler: T,
    flagKey?: string,
  ): void {
    let list = this.registry.get(operation);
    if (!list) {
      list = [];
      this.registry.set(operation, list);
    }
    if (flagKey === undefined) {
      const idx = list.findIndex((r) => r.flagKey === undefined);
      if (idx !== -1) list[idx] = { handler };
      else list.push({ handler });
    } else {
      list.push({ handler, flagKey });
    }
  }

  /**
   * Dispatch the appropriate handler for the given operation.
   *
   * Returns the first registered handler whose flag is enabled. Falls back
   * to the default (no-flagKey) handler if no flagged handler matches.
   *
   * @throws If no handlers are registered for the operation.
   * @throws If no flagged handler matches and no default is registered.
   */
  dispatch<T extends (...args: any[]) => any>(
    operation: string,
    _context?: EvaluationContext,
  ): T {
    const list = this.registry.get(operation);
    if (!list || list.length === 0) {
      throw new Error(
        `No handlers registered for operation '${operation}'. Call register() before dispatch().`,
      );
    }
    for (const reg of list) {
      if (reg.flagKey !== undefined) {
        const flag = this.flags.get(reg.flagKey);
        if (flag && flag.enabled) return reg.handler as T;
      }
    }
    const defaultReg = list.find((r) => r.flagKey === undefined);
    if (!defaultReg) {
      throw new Error(
        `No matching handler for operation '${operation}' and no default registered. Register a default handler (no flagKey) as the last registration.`,
      );
    }
    return defaultReg.handler as T;
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
    if (this.flagConfig) {
      const ctx = this.user ? { attributes: this.user.attributes } : undefined;
      const result = evaluateLocal(this.flagConfig, this.environment, key, ctx);
      if (result.reason === 'flag_not_found') return defaultValue;
      if (result.value === 'true') return true;
      if (result.value === 'false') return false;
      return defaultValue;
    }
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
    if (this.flagConfig) {
      const ctx = this.user ? { attributes: this.user.attributes } : undefined;
      const result = evaluateLocal(this.flagConfig, this.environment, key, ctx);
      if (result.reason === 'flag_not_found') return defaultValue;
      return result.value;
    }
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
    if (this.flagConfig) {
      const ctx = this.user ? { attributes: this.user.attributes } : undefined;
      const result = evaluateLocal(this.flagConfig, this.environment, key, ctx);
      if (result.reason === 'flag_not_found') return defaultValue;
      const parsed = Number(result.value);
      if (!Number.isNaN(parsed)) return parsed;
      return defaultValue;
    }
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
    if (this.flagConfig) {
      const ctx = this.user ? { attributes: this.user.attributes } : undefined;
      const result = evaluateLocal(this.flagConfig, this.environment, key, ctx);
      if (result.reason === 'flag_not_found') return defaultValue;
      try {
        return JSON.parse(result.value) as T;
      } catch {
        return defaultValue;
      }
    }
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
      project_id: this.project,
      environment_id: this.environment,
      application: this.application,
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
    // Match the backend `listFlags` endpoint, which only requires a project_id.
    const url = `${this.baseURL}/api/v1/flags?project_id=${encodeURIComponent(this.project)}&application=${encodeURIComponent(this.application)}&environment_id=${encodeURIComponent(this.environment)}`;

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

  private async fetchSingleFlag(flagId: string): Promise<void> {
    const url = `${this.baseURL}/api/v1/flags/${encodeURIComponent(flagId)}?environment_id=${encodeURIComponent(this.environment)}`;

    const response = await fetch(url, {
      method: 'GET',
      headers: this.headers,
    });

    if (!response.ok) {
      throw new Error(
        `DeploySentry: failed to fetch flag ${flagId} (${response.status})`,
      );
    }

    const raw: ApiFlagResponse = await response.json();
    const flag = toFlag(raw);
    this.flags.set(flag.key, flag);
    this.emit();
  }

  // ---------------------------------------------------------------------------
  // SSE
  // ---------------------------------------------------------------------------

  private connectSSE(): void {
    if (this.destroyed) return;
    if (typeof EventSource === 'undefined') return; // SSR guard

    const params = this.buildQueryParams();
    const url = `${this.baseURL}/api/v1/flags/stream?${params.toString()}`;

    // EventSource does not support custom headers natively. We pass the
    // API key as a query parameter for SSE connections.
    const sseUrl = new URL(url);
    sseUrl.searchParams.set('token', this.apiKey);

    const es = new EventSource(sseUrl.toString());

    // The backend sends all flag change events with SSE event type
    // "flag_change". The JSON payload's inner "event" field distinguishes
    // the specific action (flag.updated, flag.toggled, flag.deleted, etc.).
    es.addEventListener('flag_change', (event: MessageEvent) => {
      try {
        const outer = JSON.parse(event.data);
        // event.data is a double-encoded JSON string from SSEvent():
        // the outer layer is the SSE data field, inner is the SSEEvent struct.
        const data = typeof outer === 'string' ? JSON.parse(outer) : outer;
        if (data?.event === 'flag.deleted' && data?.flag_key) {
          this.flags.delete(data.flag_key);
          this.emit();
        } else if (data?.flag_id) {
          // Fetch only the changed flag instead of the full set.
          this.fetchSingleFlag(data.flag_id).catch(() => this.fetchFlags());
        } else {
          // No flag_id available — fall back to a full refresh.
          this.fetchFlags();
        }
      } catch {
        // Malformed event — trigger a full refresh as a fallback.
        this.fetchFlags();
      }
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
