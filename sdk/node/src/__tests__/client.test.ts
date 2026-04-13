import { DeploySentryClient } from '../client';

const validOptions = {
  apiKey: 'ds_live_test123',
  environment: 'staging',
  project: 'my-project',
};

describe('DeploySentryClient constructor', () => {
  it('succeeds with valid options', () => {
    const client = new DeploySentryClient(validOptions);
    expect(client).toBeInstanceOf(DeploySentryClient);
  });

  it('throws if apiKey is missing', () => {
    expect(() => {
      new DeploySentryClient({ ...validOptions, apiKey: '' });
    }).toThrow('apiKey is required');
  });

  it('throws if apiKey is undefined', () => {
    expect(() => {
      new DeploySentryClient({
        apiKey: undefined as unknown as string,
        environment: 'staging',
        project: 'my-project',
      });
    }).toThrow('apiKey is required');
  });

  it('throws if environment is missing', () => {
    expect(() => {
      new DeploySentryClient({ ...validOptions, environment: '' });
    }).toThrow('environment is required');
  });

  it('throws if environment is undefined', () => {
    expect(() => {
      new DeploySentryClient({
        apiKey: 'ds_live_test123',
        environment: undefined as unknown as string,
        project: 'my-project',
      });
    }).toThrow('environment is required');
  });

  it('throws if project is missing', () => {
    expect(() => {
      new DeploySentryClient({ ...validOptions, project: '' });
    }).toThrow('project is required');
  });

  it('accepts sessionId without error', () => {
    const client = new DeploySentryClient({
      ...validOptions,
      sessionId: 'sess_abc123',
    });
    expect(client).toBeInstanceOf(DeploySentryClient);
  });

  it('accepts optional baseURL', () => {
    const client = new DeploySentryClient({
      ...validOptions,
      baseURL: 'https://custom.api.example.com',
    });
    expect(client).toBeInstanceOf(DeploySentryClient);
  });

  it('accepts offlineMode', () => {
    const client = new DeploySentryClient({
      ...validOptions,
      offlineMode: true,
    });
    expect(client).toBeInstanceOf(DeploySentryClient);
    expect(client.isInitialized).toBe(false);
  });

  it('is not initialized before calling initialize()', () => {
    const client = new DeploySentryClient(validOptions);
    expect(client.isInitialized).toBe(false);
  });
});

describe('register and dispatch', () => {
  let client: DeploySentryClient;

  beforeEach(() => {
    client = new DeploySentryClient({
      apiKey: 'test-key',
      environment: 'test',
      project: 'test-project',
    });
  });

  function mockFlagEnabled(key: string, enabled: boolean) {
    (client as any).cache.set({
      key,
      enabled,
      value: enabled,
      updatedAt: new Date().toISOString(),
      metadata: { category: 'feature', purpose: '', owners: [], isPermanent: false, tags: [] },
    });
  }

  it('dispatches the flagged handler when flag is on', () => {
    const defaultFn = () => 'default';
    const featFn = () => 'feature';
    client.register('op', featFn, 'my-flag');
    client.register('op', defaultFn);
    mockFlagEnabled('my-flag', true);
    const result = client.dispatch('op')();
    expect(result).toBe('feature');
  });

  it('dispatches the default handler when flag is off', () => {
    const defaultFn = () => 'default';
    const featFn = () => 'feature';
    client.register('op', featFn, 'my-flag');
    client.register('op', defaultFn);
    mockFlagEnabled('my-flag', false);
    const result = client.dispatch('op')();
    expect(result).toBe('default');
  });

  it('returns the first matching handler when multiple flags are on', () => {
    const fn1 = () => 'first';
    const fn2 = () => 'second';
    const defaultFn = () => 'default';
    client.register('op', fn1, 'flag-a');
    client.register('op', fn2, 'flag-b');
    client.register('op', defaultFn);
    mockFlagEnabled('flag-a', true);
    mockFlagEnabled('flag-b', true);
    const result = client.dispatch('op')();
    expect(result).toBe('first');
  });

  it('dispatches the default when only a default is registered', () => {
    const defaultFn = () => 'default';
    client.register('op', defaultFn);
    const result = client.dispatch('op')();
    expect(result).toBe('default');
  });

  it('keeps operations isolated', () => {
    client.register('cart', () => 'cart-default');
    client.register('pay', () => 'pay-default');
    expect(client.dispatch('cart')()).toBe('cart-default');
    expect(client.dispatch('pay')()).toBe('pay-default');
  });

  it('throws on unregistered operation', () => {
    expect(() => client.dispatch('unknown')).toThrow(
      "No handlers registered for operation 'unknown'"
    );
  });

  it('throws when no flag matches and no default exists', () => {
    client.register('op', () => 'feat', 'my-flag');
    mockFlagEnabled('my-flag', false);
    expect(() => client.dispatch('op')).toThrow(
      "No matching handler for operation 'op' and no default registered"
    );
  });

  it('replaces a previous default for the same operation', () => {
    client.register('op', () => 'first-default');
    client.register('op', () => 'second-default');
    expect(client.dispatch('op')()).toBe('second-default');
  });

  it('passes caller args through to the handler', () => {
    client.register('add', (a: number, b: number) => a + b);
    const result = client.dispatch<(a: number, b: number) => number>('add')(3, 4);
    expect(result).toBe(7);
  });
});
