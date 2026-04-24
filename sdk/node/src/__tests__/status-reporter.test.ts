import { StatusReporter, resolveVersion } from '../status-reporter';

describe('resolveVersion', () => {
  const savedEnv = { ...process.env };
  afterEach(() => {
    // Reset env between tests
    for (const k of Object.keys(process.env)) {
      if (!(k in savedEnv)) delete process.env[k];
    }
    Object.assign(process.env, savedEnv);
  });

  it('prefers the explicit override', () => {
    process.env.APP_VERSION = '0.0.1';
    expect(resolveVersion('9.9.9')).toBe('9.9.9');
  });

  it('falls back to env vars in priority order', () => {
    delete process.env.APP_VERSION;
    process.env.GIT_SHA = 'abc123';
    expect(resolveVersion()).toBe('abc123');
  });

  it('returns "unknown" when nothing is available', () => {
    delete process.env.APP_VERSION;
    delete process.env.GIT_SHA;
    delete process.env.GIT_COMMIT;
    delete process.env.SOURCE_COMMIT;
    delete process.env.RAILWAY_GIT_COMMIT_SHA;
    delete process.env.RENDER_GIT_COMMIT;
    delete process.env.VERCEL_GIT_COMMIT_SHA;
    delete process.env.HEROKU_SLUG_COMMIT;
    delete process.env.npm_package_version;
    expect(resolveVersion()).toBe('unknown');
  });
});

describe('StatusReporter', () => {
  it('rejects negative intervals', () => {
    expect(
      () =>
        new StatusReporter({
          baseURL: 'http://x',
          apiKey: 'k',
          applicationId: 'a',
          intervalMs: -1,
        }),
    ).toThrow('intervalMs must be >= 0');
  });

  it('POSTs to the right URL with ApiKey auth and JSON body', async () => {
    const fetchImpl = jest.fn().mockResolvedValue({ ok: true, status: 201, statusText: 'Created' });
    const reporter = new StatusReporter({
      baseURL: 'https://api.example.com/',
      apiKey: 'ds_test',
      applicationId: 'f47ac10b-58cc-4372-a567-0e02b2c3d479',
      intervalMs: 0,
      version: '1.4.2',
      commitSha: 'abc123',
      deploySlot: 'canary',
      tags: { region: 'us-east' },
      fetchImpl: fetchImpl as unknown as typeof fetch,
    });

    await reporter.reportOnce();

    expect(fetchImpl).toHaveBeenCalledTimes(1);
    const [url, init] = fetchImpl.mock.calls[0] as [string, RequestInit];
    expect(url).toBe(
      'https://api.example.com/api/v1/applications/f47ac10b-58cc-4372-a567-0e02b2c3d479/status',
    );
    expect(init.method).toBe('POST');
    const headers = init.headers as Record<string, string>;
    expect(headers.Authorization).toBe('ApiKey ds_test');
    const body = JSON.parse(init.body as string);
    expect(body.version).toBe('1.4.2');
    expect(body.health).toBe('healthy');
    expect(body.commit_sha).toBe('abc123');
    expect(body.deploy_slot).toBe('canary');
    expect(body.tags).toEqual({ region: 'us-east' });
  });

  it('awaits the health provider and reflects its output', async () => {
    const fetchImpl = jest.fn().mockResolvedValue({ ok: true, status: 201, statusText: 'OK' });
    const reporter = new StatusReporter({
      baseURL: 'http://x',
      apiKey: 'k',
      applicationId: 'a',
      intervalMs: 0,
      version: '1',
      healthProvider: async () => ({ state: 'degraded', score: 0.8, reason: 'db slow' }),
      fetchImpl: fetchImpl as unknown as typeof fetch,
    });

    await reporter.reportOnce();
    const body = JSON.parse((fetchImpl.mock.calls[0] as [string, RequestInit])[1].body as string);
    expect(body.health).toBe('degraded');
    expect(body.health_score).toBe(0.8);
    expect(body.health_reason).toBe('db slow');
  });

  it('reports unknown when the health provider throws', async () => {
    const fetchImpl = jest.fn().mockResolvedValue({ ok: true, status: 201, statusText: 'OK' });
    const reporter = new StatusReporter({
      baseURL: 'http://x',
      apiKey: 'k',
      applicationId: 'a',
      intervalMs: 0,
      version: '1',
      healthProvider: () => {
        throw new Error('boom');
      },
      fetchImpl: fetchImpl as unknown as typeof fetch,
    });

    await reporter.reportOnce();
    const body = JSON.parse((fetchImpl.mock.calls[0] as [string, RequestInit])[1].body as string);
    expect(body.health).toBe('unknown');
    expect(body.health_reason).toContain('boom');
  });

  it('throws for non-2xx responses', async () => {
    const fetchImpl = jest.fn().mockResolvedValue({ ok: false, status: 500, statusText: 'ouch' });
    const reporter = new StatusReporter({
      baseURL: 'http://x',
      apiKey: 'k',
      applicationId: 'a',
      intervalMs: 0,
      version: '1',
      fetchImpl: fetchImpl as unknown as typeof fetch,
    });

    await expect(reporter.reportOnce()).rejects.toThrow(/500/);
  });

  it('start() in interval=0 mode only fires once and then stops', async () => {
    const fetchImpl = jest.fn().mockResolvedValue({ ok: true, status: 201, statusText: 'OK' });
    const reporter = new StatusReporter({
      baseURL: 'http://x',
      apiKey: 'k',
      applicationId: 'a',
      intervalMs: 0,
      version: '1',
      fetchImpl: fetchImpl as unknown as typeof fetch,
    });

    reporter.start();
    // flush microtasks so the initial tick resolves
    await new Promise((r) => setImmediate(r));
    reporter.stop();

    expect(fetchImpl).toHaveBeenCalledTimes(1);
  });
});

// ---------------------------------------------------------------------------
// Backoff ladder: ensure a stuck-at-max reporter doesn't miss server recovery
// by more than one normal interval.
// ---------------------------------------------------------------------------

describe('StatusReporter backoff ladder', () => {
  // The ladder doubles from MIN_BACKOFF_MS (1s) up to MAX_BACKOFF_MS (5m).
  const EXPECTED_LADDER_MS = [
    1_000,
    2_000,
    4_000,
    8_000,
    16_000,
    32_000,
    64_000,
    128_000,
    256_000,
    300_000, // clamps at MAX_BACKOFF_MS
    300_000, // still clamped
  ];

  // Drive the reporter manually by repeatedly calling its internal tick via
  // start()+stop(). We capture the delay the reporter *would have* scheduled
  // by spying on setTimeout instead of letting real timers fire.
  async function runOneTick(reporter: StatusReporter, setTimeoutSpy: jest.SpyInstance) {
    setTimeoutSpy.mockClear();
    reporter.start();
    // Let reportOnce() and the follow-up backoff math resolve.
    await new Promise((r) => setImmediate(r));
    reporter.stop();
    // Prior start() schedules exactly one setTimeout after the tick resolves.
    // Record its delay, then return it for the assertion.
    const delays = setTimeoutSpy.mock.calls
      .map((c) => c[1] as number)
      .filter((ms) => typeof ms === 'number');
    return delays[delays.length - 1];
  }

  function makeReporter(
    fetchImpl: jest.Mock,
    warn: jest.Mock,
    intervalMs = 30_000,
  ): StatusReporter {
    return new StatusReporter({
      baseURL: 'http://x',
      apiKey: 'k',
      applicationId: 'a',
      intervalMs,
      version: '1',
      fetchImpl: fetchImpl as unknown as typeof fetch,
      warn,
    });
  }

  it('climbs 1s, 2s, 4s, …, 300s on consecutive failures', async () => {
    const fetchImpl = jest
      .fn()
      .mockResolvedValue({ ok: false, status: 422, statusText: 'FK violation' });
    const warn = jest.fn();
    const reporter = makeReporter(fetchImpl, warn);
    const spy = jest.spyOn(global, 'setTimeout');
    try {
      for (let i = 0; i < 9; i++) {
        const delay = await runOneTick(reporter, spy);
        expect(delay).toBe(EXPECTED_LADDER_MS[i]);
      }
    } finally {
      spy.mockRestore();
    }
  });

  it('resets to intervalMs on the tick after reaching MAX_BACKOFF_MS', async () => {
    const fetchImpl = jest
      .fn()
      .mockResolvedValue({ ok: false, status: 422, statusText: 'FK violation' });
    const warn = jest.fn();
    const reporter = makeReporter(fetchImpl, warn, 30_000);
    const spy = jest.spyOn(global, 'setTimeout');
    try {
      // Walk to MAX_BACKOFF_MS. Ladder is 1,2,4,8,16,32,64,128,256,300 →
      // 10 failed ticks lands the backoff at 300_000.
      let lastDelay = 0;
      for (let i = 0; i < 10; i++) {
        lastDelay = await runOneTick(reporter, spy);
      }
      expect(lastDelay).toBe(300_000);
      // The next failed tick should *not* stay at 300_000; it should fall
      // back to intervalMs so the SDK discovers recovery within one cycle.
      const nextDelay = await runOneTick(reporter, spy);
      expect(nextDelay).toBe(30_000);
      // Operators should see a log indicating the reporter is re-probing.
      expect(warn).toHaveBeenCalledWith(
        expect.stringContaining('backoff reset; probing every 30000ms'),
      );
    } finally {
      spy.mockRestore();
    }
  });

  it('clears backoff to zero on success and uses intervalMs thereafter', async () => {
    const warn = jest.fn();
    const failing = { ok: false, status: 422, statusText: 'FK' };
    const success = { ok: true, status: 201, statusText: 'Created' };
    const fetchImpl = jest
      .fn()
      .mockResolvedValueOnce(failing)
      .mockResolvedValueOnce(failing)
      .mockResolvedValueOnce(success);
    const reporter = makeReporter(fetchImpl, warn, 30_000);
    const spy = jest.spyOn(global, 'setTimeout');
    try {
      expect(await runOneTick(reporter, spy)).toBe(1_000); // fail 1
      expect(await runOneTick(reporter, spy)).toBe(2_000); // fail 2
      expect(await runOneTick(reporter, spy)).toBe(30_000); // success → intervalMs
    } finally {
      spy.mockRestore();
    }
  });

  it('restarts the ladder from MIN_BACKOFF_MS on the next failure after a reset', async () => {
    const warn = jest.fn();
    const failing = { ok: false, status: 422, statusText: 'FK' };
    const fetchImpl = jest.fn().mockResolvedValue(failing);
    const reporter = makeReporter(fetchImpl, warn, 30_000);
    const spy = jest.spyOn(global, 'setTimeout');
    try {
      // Drive to MAX, then let the reset fire once.
      for (let i = 0; i < 10; i++) await runOneTick(reporter, spy); // reaches 300_000
      const afterReset = await runOneTick(reporter, spy);
      expect(afterReset).toBe(30_000);
      // The very next failure restarts at 1s, not jumps back to 300s.
      const nextFailure = await runOneTick(reporter, spy);
      expect(nextFailure).toBe(1_000);
    } finally {
      spy.mockRestore();
    }
  });
});
