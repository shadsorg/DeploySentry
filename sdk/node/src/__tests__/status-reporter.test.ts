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
