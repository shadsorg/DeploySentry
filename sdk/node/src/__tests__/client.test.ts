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
