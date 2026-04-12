import * as fs from 'fs';
import * as path from 'path';

const TESTDATA_DIR = path.resolve(__dirname, '..', '..', '..', '..', 'testdata');

function loadFixture(filename: string): unknown {
  const raw = fs.readFileSync(path.join(TESTDATA_DIR, filename), 'utf-8');
  return JSON.parse(raw);
}

describe('contract: auth_request fixture', () => {
  const fixture = loadFixture('auth_request.json') as {
    header_name: string;
    header_value_prefix: string;
  };

  it('requires Authorization header', () => {
    expect(fixture.header_name).toBe('Authorization');
  });

  it('uses "ApiKey " prefix', () => {
    expect(fixture.header_value_prefix).toBe('ApiKey ');
  });
});

describe('contract: evaluate_response fixture', () => {
  const fixture = loadFixture('evaluate_response.json') as {
    body: {
      flag_key: string;
      value: unknown;
      enabled: boolean;
      reason: string;
      metadata: Record<string, unknown>;
    };
    valid_reasons: string[];
  };

  it('has a flag_key field', () => {
    expect(fixture.body.flag_key).toBeDefined();
    expect(typeof fixture.body.flag_key).toBe('string');
  });

  it('has a boolean enabled field', () => {
    expect(typeof fixture.body.enabled).toBe('boolean');
  });

  it('has a reason field with a valid value', () => {
    expect(fixture.valid_reasons).toContain(fixture.body.reason);
  });

  it('includes metadata with category', () => {
    expect(fixture.body.metadata).toBeDefined();
    expect(fixture.body.metadata.category).toBeDefined();
  });

  it('parses value correctly', () => {
    expect(fixture.body.value).toBe(true);
  });
});

describe('contract: list_flags_response fixture', () => {
  const fixture = loadFixture('list_flags_response.json') as {
    body: { flags: Array<{ key: string; enabled: boolean; value: unknown; metadata: Record<string, unknown> }> };
  };

  it('has exactly 3 flags', () => {
    expect(fixture.body.flags).toHaveLength(3);
  });

  it('each flag has key, enabled, value, and metadata', () => {
    for (const flag of fixture.body.flags) {
      expect(flag.key).toBeDefined();
      expect(typeof flag.enabled).toBe('boolean');
      expect(flag.value).toBeDefined();
      expect(flag.metadata).toBeDefined();
    }
  });

  it('flag keys match expected values', () => {
    const keys = fixture.body.flags.map((f) => f.key);
    expect(keys).toEqual(['dark-mode', 'new-checkout', 'max-items']);
  });
});

describe('contract: batch_evaluate_response fixture', () => {
  const fixture = loadFixture('batch_evaluate_response.json') as {
    body: { results: Array<{ flag_key: string; value: unknown; reason: string }> };
  };

  it('has exactly 3 results', () => {
    expect(fixture.body.results).toHaveLength(3);
  });

  it('each result has flag_key, value, and reason', () => {
    for (const result of fixture.body.results) {
      expect(result.flag_key).toBeDefined();
      expect(result.value).toBeDefined();
      expect(result.reason).toBeDefined();
    }
  });
});
