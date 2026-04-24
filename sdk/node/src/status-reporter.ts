import type { HealthReport } from './types';

/** Environment variable names probed (in order) for the running version. */
const VERSION_ENV_CHAIN = [
  'APP_VERSION',
  'GIT_SHA',
  'GIT_COMMIT',
  'SOURCE_COMMIT',
  'RAILWAY_GIT_COMMIT_SHA',
  'RENDER_GIT_COMMIT',
  'VERCEL_GIT_COMMIT_SHA',
  'HEROKU_SLUG_COMMIT',
] as const;

const MIN_BACKOFF_MS = 1_000;
const MAX_BACKOFF_MS = 5 * 60_000;

/** Options passed to StatusReporter. Internal to the SDK. */
export interface StatusReporterOptions {
  baseURL: string;
  apiKey: string;
  applicationId: string;
  intervalMs?: number;
  version?: string;
  commitSha?: string;
  deploySlot?: string;
  tags?: Record<string, string>;
  healthProvider?: () => HealthReport | Promise<HealthReport>;
  /** Injection seam for tests. Defaults to global fetch. */
  fetchImpl?: typeof fetch;
  /** Injection seam for tests. */
  warn?: (msg: string) => void;
}

/**
 * Resolve the version the app should report, in this preference order:
 * explicit config → standard env vars → `npm_package_version` → "unknown".
 */
export function resolveVersion(explicit?: string): string {
  if (explicit && explicit.trim()) return explicit;
  const env = typeof process !== 'undefined' && process.env ? process.env : {};
  for (const name of VERSION_ENV_CHAIN) {
    const v = env[name];
    if (v && v.trim()) return v;
  }
  if (env.npm_package_version) return env.npm_package_version;
  return 'unknown';
}

/**
 * StatusReporter posts periodic `POST /applications/:id/status` updates to
 * DeploySentry on behalf of a DeploySentryClient. Failures are swallowed
 * (logged via `warn`) so flag evaluation is never blocked by reporting.
 */
export class StatusReporter {
  private readonly opts: Required<
    Pick<StatusReporterOptions, 'baseURL' | 'apiKey' | 'applicationId' | 'intervalMs' | 'warn'>
  > &
    Omit<StatusReporterOptions, 'baseURL' | 'apiKey' | 'applicationId' | 'intervalMs' | 'warn'>;

  private timer: ReturnType<typeof setTimeout> | null = null;
  private stopped = false;
  private backoff = 0;

  constructor(options: StatusReporterOptions) {
    if (!options.baseURL) throw new Error('baseURL is required');
    if (!options.apiKey) throw new Error('apiKey is required');
    if (!options.applicationId) throw new Error('applicationId is required');
    const intervalMs = options.intervalMs ?? 30_000;
    if (intervalMs < 0) throw new Error('intervalMs must be >= 0');

    this.opts = {
      ...options,
      baseURL: options.baseURL.replace(/\/+$/, ''),
      apiKey: options.apiKey,
      applicationId: options.applicationId,
      intervalMs,
      warn: options.warn ?? ((msg) => console.warn('[DeploySentry] ' + msg)),
    };
  }

  /** Fire one immediate report and (if intervalMs > 0) schedule repeats. */
  start(): void {
    this.stopped = false;
    void this.tick();
  }

  /** Stop scheduling further reports. In-flight reports still resolve. */
  stop(): void {
    this.stopped = true;
    if (this.timer) {
      clearTimeout(this.timer);
      this.timer = null;
    }
  }

  /** Send exactly one report. Public for tests and explicit callers. */
  async reportOnce(): Promise<void> {
    const version = resolveVersion(this.opts.version);
    let health: HealthReport = { state: 'healthy' };
    if (this.opts.healthProvider) {
      try {
        health = await this.opts.healthProvider();
      } catch (err) {
        health = { state: 'unknown', reason: String((err as Error)?.message ?? err) };
      }
    }

    const body: Record<string, unknown> = {
      version,
      health: health.state,
    };
    if (health.score !== undefined) body.health_score = health.score;
    if (health.reason) body.health_reason = health.reason;
    if (this.opts.commitSha) body.commit_sha = this.opts.commitSha;
    if (this.opts.deploySlot) body.deploy_slot = this.opts.deploySlot;
    if (this.opts.tags && Object.keys(this.opts.tags).length) body.tags = this.opts.tags;

    const fetchImpl = this.opts.fetchImpl ?? fetch;
    const url = `${this.opts.baseURL}/api/v1/applications/${encodeURIComponent(
      this.opts.applicationId,
    )}/status`;

    const res = await fetchImpl(url, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `ApiKey ${this.opts.apiKey}`,
      },
      body: JSON.stringify(body),
    });
    if (!res.ok) {
      throw new Error(`status report failed: ${res.status} ${res.statusText}`);
    }
  }

  private async tick(): Promise<void> {
    if (this.stopped) return;
    try {
      await this.reportOnce();
      this.backoff = 0; // success — clear backoff
    } catch (err) {
      this.opts.warn(`status report error: ${(err as Error).message}`);
      if (this.backoff >= MAX_BACKOFF_MS) {
        // Already at the ceiling. Reset so the next schedule falls back to
        // intervalMs — otherwise a server that recovers mid-outage takes up
        // to MAX_BACKOFF_MS (5m) to be noticed, regardless of how tight the
        // operator configured intervalMs. On the next failure the 1s ladder
        // restarts, so the short-outage behavior (quickly re-probe while
        // the server is still down) is preserved.
        this.backoff = 0;
        this.opts.warn(
          `status reporter backoff reset; probing every ${this.opts.intervalMs}ms`,
        );
      } else {
        this.backoff = this.backoff === 0 ? MIN_BACKOFF_MS : Math.min(this.backoff * 2, MAX_BACKOFF_MS);
      }
    }
    if (this.stopped) return;
    if (this.opts.intervalMs === 0) return; // startup-only
    const nextDelay = this.backoff > 0 ? this.backoff : this.opts.intervalMs;
    this.timer = setTimeout(() => void this.tick(), nextDelay);
    // Allow Node to exit even when the timer is pending.
    if (typeof (this.timer as { unref?: () => void }).unref === 'function') {
      (this.timer as { unref?: () => void }).unref?.();
    }
  }
}
