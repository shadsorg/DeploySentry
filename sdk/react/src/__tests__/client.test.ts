import { DeploySentryClient } from '../client';

function makeClient() {
  return new DeploySentryClient({
    apiKey: 'test-key',
    baseURL: 'https://api.example.com',
    environment: 'test',
    project: 'test-project',
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
