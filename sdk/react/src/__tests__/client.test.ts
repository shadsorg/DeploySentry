import { DeploySentryClient } from '../client';

function makeClient() {
  return new DeploySentryClient({
    apiKey: 'test-key',
    baseURL: 'https://api.example.com',
    environment: 'test',
    project: 'test-project',
    application: 'test-app',
  });
}

describe('DeploySentryClient', () => {
  describe('boolValue', () => {
    it('returns default when flag is missing', () => {
      const client = makeClient();
      expect(client.boolValue('missing-flag', true)).toBe(true);
      expect(client.boolValue('missing-flag', false)).toBe(false);
    });
  });

  describe('stringValue', () => {
    it('returns default when flag is missing', () => {
      const client = makeClient();
      expect(client.stringValue('missing-flag', 'fallback')).toBe('fallback');
    });
  });

  describe('numberValue', () => {
    it('returns default when flag is missing', () => {
      const client = makeClient();
      expect(client.numberValue('missing-flag', 42)).toBe(42);
    });
  });

  describe('jsonValue', () => {
    it('returns default when flag is missing', () => {
      const client = makeClient();
      const fallback = { theme: 'dark' };
      expect(client.jsonValue('missing-flag', fallback)).toBe(fallback);
    });
  });

  describe('detail', () => {
    it('returns not-found shape for missing flag', () => {
      const client = makeClient();
      const result = client.detail('missing-flag');

      expect(result.value).toBeUndefined();
      expect(result.enabled).toBe(false);
      expect(result.loading).toBe(true); // not initialised yet
      expect(result.metadata).toEqual({
        category: 'feature',
        purpose: '',
        owners: [],
        isPermanent: false,
        tags: [],
      });
    });
  });

  describe('isInitialised', () => {
    it('is false before init()', () => {
      const client = makeClient();
      expect(client.isInitialised).toBe(false);
    });
  });

  describe('getAllFlags', () => {
    it('returns empty array before init', () => {
      const client = makeClient();
      expect(client.getAllFlags()).toEqual([]);
    });
  });
});

describe('register and dispatch', () => {
  let client: DeploySentryClient;

  beforeEach(() => {
    client = new DeploySentryClient({
      apiKey: 'test-key',
      baseURL: 'http://localhost',
      environment: 'test',
      project: 'test-project',
      application: 'test-app',
    });
  });

  function mockFlagEnabled(key: string, enabled: boolean) {
    (client as any).flags.set(key, {
      key,
      enabled,
      value: enabled,
      name: key,
      category: 'feature',
      purpose: '',
      owners: [],
      isPermanent: false,
      tags: [],
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
      "No handlers registered for operation 'unknown'",
    );
  });

  it('throws when no flag matches and no default exists', () => {
    client.register('op', () => 'feat', 'my-flag');
    mockFlagEnabled('my-flag', false);
    expect(() => client.dispatch('op')).toThrow(
      "No matching handler for operation 'op' and no default registered",
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
