import { Flag } from './types';

/** Callback invoked whenever a flag update arrives over SSE. */
export type FlagUpdateHandler = (flags: Flag[]) => void;

/** Callback invoked when the SSE connection encounters an error. */
export type StreamErrorHandler = (error: Error) => void;

const SSE_INITIAL_RETRY_MS = 1_000;
const SSE_MAX_RETRY_MS = 30_000;
const SSE_BACKOFF_MULTIPLIER = 2;
const SSE_JITTER_FRACTION = 0.2;

interface StreamOptions {
  /** Full URL of the SSE endpoint including query parameters. */
  url: string;
  /** Headers to attach to the request (e.g. Authorization). */
  headers: Record<string, string>;
  /** Called when one or more flag updates are received. */
  onUpdate: FlagUpdateHandler;
  /** Called when the stream encounters an error. */
  onError?: StreamErrorHandler;
}

/**
 * Lightweight SSE streaming client that keeps the local flag cache
 * synchronised with the DeploySentry service.
 *
 * Uses the built-in `fetch` readable stream API available in Node 18+
 * to avoid external EventSource dependencies.
 */
export class FlagStreamClient {
  private abortController: AbortController | null = null;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private closed = false;
  private retryMs = SSE_INITIAL_RETRY_MS;

  private readonly url: string;
  private readonly headers: Record<string, string>;
  private readonly onUpdate: FlagUpdateHandler;
  private readonly onError: StreamErrorHandler;

  constructor(options: StreamOptions) {
    this.url = options.url;
    this.headers = options.headers;
    this.onUpdate = options.onUpdate;
    this.onError = options.onError ?? (() => {});
  }

  /** Open the SSE connection and start processing events. */
  async connect(): Promise<void> {
    if (this.closed) return;

    this.abortController = new AbortController();

    try {
      const response = await fetch(this.url, {
        method: 'GET',
        headers: {
          ...this.headers,
          Accept: 'text/event-stream',
          'Cache-Control': 'no-cache',
        },
        signal: this.abortController.signal,
      });

      if (!response.ok) {
        throw new Error(
          `SSE connection failed: ${response.status} ${response.statusText}`,
        );
      }

      if (!response.body) {
        throw new Error('SSE response has no body');
      }

      this.retryMs = SSE_INITIAL_RETRY_MS;
      await this.consumeStream(response.body);
    } catch (err: unknown) {
      if (this.closed) return;

      const error = err instanceof Error ? err : new Error(String(err));
      if (error.name === 'AbortError') return;

      this.onError(error);
      this.scheduleReconnect();
    }
  }

  /** Close the connection and stop reconnection attempts. */
  close(): void {
    this.closed = true;

    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }

    if (this.abortController) {
      this.abortController.abort();
      this.abortController = null;
    }
  }

  // ---------------------------------------------------------------------------
  // Internals
  // ---------------------------------------------------------------------------

  private async consumeStream(body: ReadableStream<Uint8Array>): Promise<void> {
    const reader = body.getReader();
    const decoder = new TextDecoder();
    let buffer = '';

    try {
      while (!this.closed) {
        const { done, value } = await reader.read();
        if (done) break;

        buffer += decoder.decode(value, { stream: true });

        // SSE events are delimited by double newlines.
        const parts = buffer.split('\n\n');
        buffer = parts.pop() ?? '';

        for (const raw of parts) {
          this.processEvent(raw);
        }
      }
    } finally {
      reader.releaseLock();
    }

    // If the stream ended naturally (server closed), attempt reconnect.
    if (!this.closed) {
      this.scheduleReconnect();
    }
  }

  private processEvent(raw: string): void {
    let eventType = 'message';
    let data = '';

    for (const line of raw.split('\n')) {
      if (line.startsWith('event:')) {
        eventType = line.slice(6).trim();
      } else if (line.startsWith('data:')) {
        data += line.slice(5).trim();
      }
    }

    if (eventType === 'flag_change' || eventType === 'flag_update' || eventType === 'message') {
      try {
        const parsed = JSON.parse(data);
        const flags: Flag[] = Array.isArray(parsed) ? parsed : [parsed];
        this.onUpdate(flags);
      } catch {
        // Ignore malformed payloads (e.g. keep-alive comments).
      }
    }
  }

  private scheduleReconnect(): void {
    if (this.closed) return;

    const jitter = this.retryMs * SSE_JITTER_FRACTION * (2 * Math.random() - 1);
    const delay = this.retryMs + jitter;

    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null;
      this.connect();
    }, delay);

    this.retryMs = Math.min(
      this.retryMs * SSE_BACKOFF_MULTIPLIER,
      SSE_MAX_RETRY_MS,
    );
  }
}
