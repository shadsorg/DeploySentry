import {
  ClientOptions,
  EvaluationContext,
  EvaluationResult,
  Flag,
  FlagCategory,
  FlagMetadata,
} from './types';
import type { FlagConfig, Registration } from './types';
import { FlagCache } from './cache';
import { FlagStreamClient } from './streaming';
import { loadFlagConfig } from './file-loader';
import { evaluateLocal } from './local-evaluator';

const DEFAULT_BASE_URL = 'https://api.dr-sentry.com';
const DEFAULT_CACHE_TIMEOUT_MS = 60_000;

/**
 * DeploySentry feature-flag client.
 *
 * ```ts
 * const client = new DeploySentryClient({
 *   apiKey: 'ds_live_xxx',
 *   environment: 'production',
 *   project: 'my-app',
 * });
 *
 * await client.initialize();
 *
 * const darkMode = client.boolValue('dark-mode', false, { userId: 'u1' });
 *
 * client.close();
 * ```
 */
export class DeploySentryClient {
  private readonly apiKey: string;
  private readonly baseURL: string;
  private readonly environment: string;
  private readonly project: string;
  private readonly application: string;
  private readonly offlineMode: boolean;
  private readonly sessionId: string | undefined;
  private readonly mode: 'server' | 'file' | 'server-with-fallback';
  private readonly flagFilePath?: string;

  private readonly cache: FlagCache;
  private streamClient: FlagStreamClient | null = null;
  private _initialized = false;
  private flagConfig: FlagConfig | null = null;
  private registry: Map<string, Registration[]> = new Map();

  constructor(options: ClientOptions) {
    if (!options.apiKey) throw new Error('apiKey is required');
    if (!options.environment) throw new Error('environment is required');
    if (!options.project) throw new Error('project is required');
    if (!options.application) throw new Error('application is required');

    this.apiKey = options.apiKey;
    this.baseURL = (options.baseURL ?? DEFAULT_BASE_URL).replace(/\/+$/, '');
    this.environment = options.environment;
    this.project = options.project;
    this.application = options.application;
    this.offlineMode = options.offlineMode ?? false;
    this.sessionId = options.sessionId;
    this.mode = options.mode ?? 'server';
    this.flagFilePath = options.flagFilePath;

    this.cache = new FlagCache(options.cacheTimeout ?? DEFAULT_CACHE_TIMEOUT_MS);
  }

  // ---------------------------------------------------------------------------
  // Lifecycle
  // ---------------------------------------------------------------------------

  /**
   * Fetch the initial flag set and open an SSE connection for real-time
   * updates.  Must be called before evaluating flags.
   */
  async initialize(): Promise<void> {
    if (this.mode === 'file') {
      this.flagConfig = loadFlagConfig(this.flagFilePath);
      this._initialized = true;
      return;
    }

    if (this.offlineMode) {
      this._initialized = true;
      return;
    }

    try {
      // Fetch all flags for the project so the cache is warm.
      const flags = await this.fetchAllFlags();
      this.cache.setMany(flags);

      // Start streaming updates.
      this.streamClient = new FlagStreamClient({
        url: `${this.baseURL}/api/v1/flags/stream?project_id=${enc(this.project)}&environment_id=${enc(this.environment)}&application=${enc(this.application)}`,
        headers: this.authHeaders(),
        onUpdate: (updated) => this.cache.setMany(updated),
        onError: (err) => {
          // Surface errors but do not crash – the cache still serves stale data.
          console.error('[DeploySentry] stream error:', err.message);
        },
      });

      // Fire-and-forget; the stream reconnects automatically.
      this.streamClient.connect();
      this._initialized = true;
    } catch (err) {
      if (this.mode === 'server-with-fallback') {
        console.warn('[DeploySentry] Server unavailable, falling back to flag config file');
        this.flagConfig = loadFlagConfig(this.flagFilePath);
        this._initialized = true;
        return;
      }
      throw err;
    }
  }

  /** Tear down the SSE connection and release resources. */
  close(): void {
    this.streamClient?.close();
    this.streamClient = null;
    this.cache.clear();
    this._initialized = false;
  }

  /**
   * Clear the local cache and re-fetch all flags from the server.
   * Useful when session state may have changed and the client needs
   * fresh evaluations.
   */
  async refreshSession(): Promise<void> {
    if (this.offlineMode) return;

    this.cache.clear();
    const flags = await this.fetchAllFlags();
    this.cache.setMany(flags);
  }

  /** Whether {@link initialize} has been called successfully. */
  get isInitialized(): boolean {
    return this._initialized;
  }

  // ---------------------------------------------------------------------------
  // Typed value helpers
  // ---------------------------------------------------------------------------

  /** Evaluate a flag as a boolean. */
  async boolValue(
    key: string,
    defaultValue: boolean,
    context?: EvaluationContext,
  ): Promise<boolean> {
    const result: unknown = await this.evaluate<boolean>(key, defaultValue, context);
    if (typeof result === 'boolean') return result;
    // The backend stores `default_value` as a string, so the evaluation
    // endpoint commonly returns "true"/"false". Coerce so SDK callers see
    // a real boolean instead of falling back to the literal default.
    if (typeof result === 'string') {
      const lowered = result.toLowerCase();
      if (lowered === 'true') return true;
      if (lowered === 'false') return false;
    }
    return defaultValue;
  }

  /** Evaluate a flag as a string. */
  async stringValue(
    key: string,
    defaultValue: string,
    context?: EvaluationContext,
  ): Promise<string> {
    const result = await this.evaluate<string>(key, defaultValue, context);
    return typeof result === 'string' ? result : defaultValue;
  }

  /** Evaluate a flag as an integer. */
  async intValue(
    key: string,
    defaultValue: number,
    context?: EvaluationContext,
  ): Promise<number> {
    const result = await this.evaluate<number>(key, defaultValue, context);
    return typeof result === 'number' && Number.isInteger(result)
      ? result
      : defaultValue;
  }

  /** Evaluate a flag and return the value as a parsed JSON object. */
  async jsonValue<T = unknown>(
    key: string,
    defaultValue: T,
    context?: EvaluationContext,
  ): Promise<T> {
    const result = await this.evaluate<T>(key, defaultValue, context);
    return result ?? defaultValue;
  }

  // ---------------------------------------------------------------------------
  // Detail evaluation
  // ---------------------------------------------------------------------------

  /**
   * Return the full {@link EvaluationResult} for a flag including metadata,
   * reason, and resolved value.
   */
  async detail(
    key: string,
    context?: EvaluationContext,
  ): Promise<EvaluationResult> {
    if (this.offlineMode) {
      return this.offlineResult(key);
    }

    try {
      const body = await this.post<EvaluationResult>(
        '/api/v1/flags/evaluate',
        {
          project_id: this.project,
          environment_id: this.environment,
          application: this.application,
          flag_key: key,
          context: context ?? {},
          ...(this.sessionId ? { session_id: this.sessionId } : {}),
        },
      );
      return body;
    } catch {
      return this.cachedResult(key);
    }
  }

  // ---------------------------------------------------------------------------
  // Metadata helpers
  // ---------------------------------------------------------------------------

  /** Return all cached flags belonging to a given category. */
  flagsByCategory(category: FlagCategory): Flag[] {
    return this.cache.getAll().filter((f) => f.metadata.category === category);
  }

  /** Return all cached flags whose `expiresAt` is in the past. */
  expiredFlags(): Flag[] {
    const now = new Date().toISOString();
    return this.cache.getAll().filter((f) => {
      return f.metadata.expiresAt && f.metadata.expiresAt < now;
    });
  }

  /** Return the owners array for a given flag key. */
  flagOwners(key: string): string[] {
    const flag = this.cache.get(key);
    return flag?.metadata.owners ?? [];
  }

  /** Return every flag currently held in the local cache. */
  allFlags(): Flag[] {
    return this.cache.getAll();
  }

  // ---------------------------------------------------------------------------
  // Register / Dispatch
  // ---------------------------------------------------------------------------

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
        const flag = this.cache.get(reg.flagKey);
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
  // Private helpers
  // ---------------------------------------------------------------------------

  private async evaluate<T>(
    key: string,
    defaultValue: T,
    context?: EvaluationContext,
  ): Promise<T> {
    if (this.flagConfig) {
      const result = evaluateLocal(this.flagConfig, this.environment, key, context);
      if (result.reason === 'flag_not_found') return defaultValue;
      return result.value as unknown as T;
    }

    if (this.offlineMode) return defaultValue;

    // Prefer server-side evaluation so targeting rules are applied.
    try {
      const result = await this.post<EvaluationResult<T>>(
        '/api/v1/flags/evaluate',
        {
          project_id: this.project,
          environment_id: this.environment,
          application: this.application,
          flag_key: key,
          context: context ?? {},
          ...(this.sessionId ? { session_id: this.sessionId } : {}),
        },
      );
      return result.value;
    } catch {
      // Fall back to the cached flag value.
      const cached = this.cache.get(key);
      return (cached?.value as T) ?? defaultValue;
    }
  }

  private async fetchAllFlags(): Promise<Flag[]> {
    const response = await this.request<{ flags: Flag[] }>(
      'GET',
      `/api/v1/flags?project_id=${enc(this.project)}&application=${enc(this.application)}`,
    );
    return response.flags ?? [];
  }

  private async post<T>(path: string, body: unknown): Promise<T> {
    return this.request<T>('POST', path, body);
  }

  private async request<T>(
    method: string,
    path: string,
    body?: unknown,
  ): Promise<T> {
    const url = `${this.baseURL}${path}`;

    const init: RequestInit = {
      method,
      headers: {
        ...this.authHeaders(),
        'Content-Type': 'application/json',
        Accept: 'application/json',
      },
    };

    if (body !== undefined) {
      init.body = JSON.stringify(body);
    }

    const response = await fetch(url, init);

    if (!response.ok) {
      const text = await response.text().catch(() => '');
      throw new Error(
        `DeploySentry API error: ${response.status} ${response.statusText} – ${text}`,
      );
    }

    return response.json() as Promise<T>;
  }

  private authHeaders(): Record<string, string> {
    const headers: Record<string, string> = {
      Authorization: `ApiKey ${this.apiKey}`,
    };
    if (this.sessionId) {
      headers['X-DeploySentry-Session'] = this.sessionId;
    }
    return headers;
  }

  private offlineResult(key: string): EvaluationResult {
    const flag = this.cache.get(key);
    const metadata: FlagMetadata = flag?.metadata ?? {
      category: 'feature',
      purpose: '',
      owners: [],
      isPermanent: false,
      tags: [],
    };

    return {
      key,
      value: flag?.value ?? null,
      enabled: flag?.enabled ?? false,
      reason: 'OFFLINE',
      metadata,
      evaluatedAt: new Date().toISOString(),
    };
  }

  private cachedResult(key: string): EvaluationResult {
    const flag = this.cache.get(key);
    const metadata: FlagMetadata = flag?.metadata ?? {
      category: 'feature',
      purpose: '',
      owners: [],
      isPermanent: false,
      tags: [],
    };

    return {
      key,
      value: flag?.value ?? null,
      enabled: flag?.enabled ?? false,
      reason: flag ? 'CACHE' : 'ERROR',
      metadata,
      evaluatedAt: new Date().toISOString(),
    };
  }
}

/** Percent-encode a value for use in a URL query parameter. */
function enc(value: string): string {
  return encodeURIComponent(value);
}
