import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import { FlagDetailPage } from './FlagDetailPage';
import { setFetch } from '../api';
import type {
  AuditLogEntry,
  Flag,
  FlagEnvironmentState,
  OrgEnvironment,
  RuleEnvironmentState,
  TargetingRule,
} from '../types';

const ENVS: OrgEnvironment[] = [
  { id: 'env-dev', slug: 'dev', name: 'Development', sort_order: 1 },
  { id: 'env-prd', slug: 'prod', name: 'Production', is_production: true, sort_order: 2 },
];

function makeFlag(partial: Partial<Flag> = {}): Flag {
  return {
    id: 'flag-1',
    project_id: 'proj-1',
    application_id: null,
    key: 'checkout_v2',
    name: 'Checkout v2',
    flag_type: 'boolean',
    category: 'release',
    is_permanent: false,
    expires_at: '2026-12-31T00:00:00Z',
    default_value: 'false',
    enabled: true,
    archived: false,
    owners: ['alice', 'bob'],
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    ...partial,
  };
}

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), { status });
}

type RouteHandler = (url: string) => Response | Promise<Response>;

function makeFetchMock(handler: RouteHandler) {
  return vi.fn<typeof fetch>(async (input) => {
    const url = typeof input === 'string' ? input : (input as URL | Request).toString();
    return handler(url);
  });
}

interface DefaultHandlerOpts {
  flag?: Flag;
  rules?: TargetingRule[];
  ruleEnvStates?: RuleEnvironmentState[];
  envStates?: FlagEnvironmentState[];
  envs?: OrgEnvironment[];
  flagError?: boolean;
  auditPages?: AuditLogEntry[][]; // each page returned in order, by offset
}

function makeAuditEntry(idx: number, partial: Partial<AuditLogEntry> = {}): AuditLogEntry {
  return {
    id: `audit-${idx}`,
    resource_type: 'flag',
    resource_id: 'flag-1',
    action: 'flag.updated',
    actor_id: 'u1',
    actor_name: `User ${idx}`,
    created_at: `2026-04-${String(28 - (idx % 28)).padStart(2, '0')}T00:00:00Z`,
    ...partial,
  };
}

function defaultHandler(opts: DefaultHandlerOpts = {}): RouteHandler {
  const flag = opts.flag ?? makeFlag();
  const rules = opts.rules ?? [];
  const ruleEnvStates = opts.ruleEnvStates ?? [];
  const envStates = opts.envStates ?? [];
  const envs = opts.envs ?? ENVS;
  const auditPages = opts.auditPages ?? [[]];

  return (url) => {
    if (url.match(/\/flags\/[^/]+\/rules\/environment-states/)) {
      return jsonResponse({ rule_environment_states: ruleEnvStates });
    }
    if (url.match(/\/flags\/[^/]+\/rules$/)) {
      return jsonResponse({ rules });
    }
    if (url.match(/\/flags\/[^/]+\/environments/)) {
      return jsonResponse({ environment_states: envStates });
    }
    if (url.match(/\/flags\/[^/]+$/)) {
      if (opts.flagError) return jsonResponse({ error: 'Flag fetch boom' }, 500);
      return jsonResponse(flag);
    }
    if (url.includes('/orgs/acme/environments')) {
      return jsonResponse({ environments: envs });
    }
    if (url.startsWith('/api/v1/audit-log?')) {
      const m = url.match(/offset=(\d+)/);
      const offset = m ? parseInt(m[1], 10) : 0;
      const pageIndex = Math.floor(offset / 20);
      const page = auditPages[pageIndex] ?? [];
      return jsonResponse({ entries: page, total: 999 });
    }
    return jsonResponse({ error: `unhandled ${url}` }, 404);
  };
}

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route
          path="/m/orgs/:orgSlug/flags/:projectSlug/:flagId"
          element={<FlagDetailPage />}
        />
        <Route
          path="/m/orgs/:orgSlug/flags/:projectSlug/apps/:appSlug/:flagId"
          element={<FlagDetailPage />}
        />
      </Routes>
    </MemoryRouter>,
  );
}

describe('FlagDetailPage', () => {
  beforeEach(() => {
    localStorage.clear();
    localStorage.setItem('ds_token', 'header.payload.sig');
  });

  it('renders loading state initially', () => {
    setFetch(vi.fn<typeof fetch>(() => new Promise(() => {})));
    renderAt('/m/orgs/acme/flags/payments/flag-1');
    expect(screen.getByText(/Loading/i)).toBeInTheDocument();
  });

  it('renders header with flag.key, flag.name, and category badge', async () => {
    setFetch(makeFetchMock(defaultHandler()));
    renderAt('/m/orgs/acme/flags/payments/flag-1');
    expect(await screen.findByText('checkout_v2')).toBeInTheDocument();
    expect(screen.getByText('Checkout v2')).toBeInTheDocument();
    // Category badge
    expect(screen.getByText('release')).toBeInTheDocument();
  });

  it('renders one collapsible section per environment', async () => {
    setFetch(makeFetchMock(defaultHandler()));
    renderAt('/m/orgs/acme/flags/payments/flag-1');
    await screen.findByText('checkout_v2');

    expect(screen.getByRole('button', { name: /Development/ })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Production/ })).toBeInTheDocument();
  });

  it('section header click toggles expansion (rules visible only when expanded)', async () => {
    const rules: TargetingRule[] = [
      {
        id: 'rule-1',
        flag_id: 'flag-1',
        rule_type: 'percentage',
        percentage: 25,
        value: 'true',
        priority: 1,
        created_at: '',
        updated_at: '',
      },
    ];
    const ruleEnvStates: RuleEnvironmentState[] = [
      { rule_id: 'rule-1', environment_id: 'env-dev', enabled: true },
    ];
    setFetch(makeFetchMock(defaultHandler({ rules, ruleEnvStates })));
    renderAt('/m/orgs/acme/flags/payments/flag-1');
    await screen.findByText('checkout_v2');

    // Collapsed by default — rule summary not visible
    expect(screen.queryByText(/25% rollout/)).not.toBeInTheDocument();

    await userEvent.click(screen.getByRole('button', { name: /Development/ }));
    expect(await screen.findByText(/25% rollout/)).toBeInTheDocument();

    // Collapse again
    await userEvent.click(screen.getByRole('button', { name: /Development/ }));
    expect(screen.queryByText(/25% rollout/)).not.toBeInTheDocument();
  });

  it('only shows rules with RuleEnvironmentState.enabled === true for the env', async () => {
    const rules: TargetingRule[] = [
      {
        id: 'rule-on',
        flag_id: 'flag-1',
        rule_type: 'percentage',
        percentage: 10,
        value: 'true',
        priority: 1,
        created_at: '',
        updated_at: '',
      },
      {
        id: 'rule-off',
        flag_id: 'flag-1',
        rule_type: 'percentage',
        percentage: 90,
        value: 'true',
        priority: 2,
        created_at: '',
        updated_at: '',
      },
    ];
    const ruleEnvStates: RuleEnvironmentState[] = [
      { rule_id: 'rule-on', environment_id: 'env-dev', enabled: true },
      { rule_id: 'rule-off', environment_id: 'env-dev', enabled: false },
    ];
    setFetch(makeFetchMock(defaultHandler({ rules, ruleEnvStates })));
    renderAt('/m/orgs/acme/flags/payments/flag-1');
    await screen.findByText('checkout_v2');

    await userEvent.click(screen.getByRole('button', { name: /Development/ }));
    expect(await screen.findByText('10% rollout')).toBeInTheDocument();
    expect(screen.queryByText('90% rollout')).not.toBeInTheDocument();
  });

  it('renders percentage rule summary as <n>% rollout', async () => {
    const rules: TargetingRule[] = [
      {
        id: 'r1',
        flag_id: 'flag-1',
        rule_type: 'percentage',
        percentage: 50,
        value: 'true',
        priority: 1,
        created_at: '',
        updated_at: '',
      },
    ];
    const ruleEnvStates: RuleEnvironmentState[] = [
      { rule_id: 'r1', environment_id: 'env-dev', enabled: true },
    ];
    setFetch(makeFetchMock(defaultHandler({ rules, ruleEnvStates })));
    renderAt('/m/orgs/acme/flags/payments/flag-1');
    await screen.findByText('checkout_v2');
    await userEvent.click(screen.getByRole('button', { name: /Development/ }));
    expect(await screen.findByText('50% rollout')).toBeInTheDocument();
  });

  it('renders user_target rule summary as <n> user IDs', async () => {
    const rules: TargetingRule[] = [
      {
        id: 'r1',
        flag_id: 'flag-1',
        rule_type: 'user_target',
        user_ids: ['u1', 'u2', 'u3'],
        value: 'true',
        priority: 1,
        created_at: '',
        updated_at: '',
      },
    ];
    const ruleEnvStates: RuleEnvironmentState[] = [
      { rule_id: 'r1', environment_id: 'env-dev', enabled: true },
    ];
    setFetch(makeFetchMock(defaultHandler({ rules, ruleEnvStates })));
    renderAt('/m/orgs/acme/flags/payments/flag-1');
    await screen.findByText('checkout_v2');
    await userEvent.click(screen.getByRole('button', { name: /Development/ }));
    expect(await screen.findByText('3 user IDs')).toBeInTheDocument();
  });

  it('renders attribute rule summary as <attribute> <operator> <value>', async () => {
    const rules: TargetingRule[] = [
      {
        id: 'r1',
        flag_id: 'flag-1',
        rule_type: 'attribute',
        attribute: 'plan',
        operator: 'equals',
        value: 'pro',
        priority: 1,
        created_at: '',
        updated_at: '',
      },
    ];
    const ruleEnvStates: RuleEnvironmentState[] = [
      { rule_id: 'r1', environment_id: 'env-dev', enabled: true },
    ];
    setFetch(makeFetchMock(defaultHandler({ rules, ruleEnvStates })));
    renderAt('/m/orgs/acme/flags/payments/flag-1');
    await screen.findByText('checkout_v2');
    await userEvent.click(screen.getByRole('button', { name: /Development/ }));
    expect(await screen.findByText('plan equals pro')).toBeInTheDocument();
  });

  it('renders compound rule summary as "compound (edit on desktop)" with no other detail', async () => {
    const rules: TargetingRule[] = [
      {
        id: 'r1',
        flag_id: 'flag-1',
        rule_type: 'compound',
        value: 'true',
        priority: 1,
        created_at: '',
        updated_at: '',
      },
    ];
    const ruleEnvStates: RuleEnvironmentState[] = [
      { rule_id: 'r1', environment_id: 'env-dev', enabled: true },
    ];
    setFetch(makeFetchMock(defaultHandler({ rules, ruleEnvStates })));
    renderAt('/m/orgs/acme/flags/payments/flag-1');
    await screen.findByText('checkout_v2');
    await userEvent.click(screen.getByRole('button', { name: /Development/ }));
    expect(await screen.findByText('compound (edit on desktop)')).toBeInTheDocument();
  });

  it('truncates owners >3 to "name1, name2, name3 …+N more"', async () => {
    const flag = makeFlag({
      owners: ['alice', 'bob', 'carol', 'dave', 'eve'],
    });
    setFetch(makeFetchMock(defaultHandler({ flag })));
    renderAt('/m/orgs/acme/flags/payments/flag-1');
    await screen.findByText('checkout_v2');
    expect(screen.getByText(/alice, bob, carol\s*…\+2 more/)).toBeInTheDocument();
  });

  it('initial history fetch renders 20 rows', async () => {
    const page1 = Array.from({ length: 20 }, (_, i) => makeAuditEntry(i + 1));
    setFetch(makeFetchMock(defaultHandler({ auditPages: [page1] })));
    renderAt('/m/orgs/acme/flags/payments/flag-1');
    await screen.findByText('checkout_v2');
    await waitFor(() => {
      expect(screen.getByText('User 1')).toBeInTheDocument();
      expect(screen.getByText('User 20')).toBeInTheDocument();
    });
  });

  it('Load more increments offset by 20 and appends rows; hides when batch < 20', async () => {
    const page1 = Array.from({ length: 20 }, (_, i) => makeAuditEntry(i + 1));
    const page2 = Array.from({ length: 5 }, (_, i) => makeAuditEntry(i + 21));
    const fetchMock = makeFetchMock(defaultHandler({ auditPages: [page1, page2] }));
    setFetch(fetchMock);
    renderAt('/m/orgs/acme/flags/payments/flag-1');
    await screen.findByText('checkout_v2');
    await screen.findByText('User 1');

    const loadMore = await screen.findByRole('button', { name: /Load more/i });
    await userEvent.click(loadMore);

    await waitFor(() => {
      expect(screen.getByText('User 25')).toBeInTheDocument();
    });

    // Verify offset=20 was issued.
    const auditCalls = fetchMock.mock.calls
      .map((c) => String(c[0]))
      .filter((u) => u.includes('/audit-log?'));
    expect(auditCalls.some((u) => u.includes('offset=20'))).toBe(true);

    // Button should be hidden now since the second batch returned fewer than 20 rows.
    expect(screen.queryByRole('button', { name: /Load more/i })).not.toBeInTheDocument();
  });

  it('surfaces an error message when the flag fetch fails', async () => {
    setFetch(makeFetchMock(defaultHandler({ flagError: true })));
    renderAt('/m/orgs/acme/flags/payments/flag-1');
    await waitFor(() => expect(screen.getByText(/Flag fetch boom/)).toBeInTheDocument());
  });

  // ------------------------------------------------------------------
  // Phase 5 — write-path tests (env toggle + per-env default value edit)
  // ------------------------------------------------------------------

  type MethodAwareHandler = (
    url: string,
    init?: RequestInit,
  ) => Response | Promise<Response>;

  function makeFetchMockMethodAware(handler: MethodAwareHandler) {
    return vi.fn<typeof fetch>(async (input, init) => {
      const url = typeof input === 'string' ? input : (input as URL | Request).toString();
      return handler(url, init);
    });
  }

  /**
   * Wrap the existing read-only defaultHandler with PUT support for
   * `/flags/:id/environments/:envId`.
   */
  function defaultHandlerWithPut(
    opts: DefaultHandlerOpts & {
      putStatus?: number;
      putBody?: unknown;
      onPut?: (envId: string, body: unknown) => void;
    } = {},
  ): MethodAwareHandler {
    const readHandler = defaultHandler(opts);
    return (url, init) => {
      const method = (init?.method ?? 'GET').toUpperCase();
      const m = url.match(/\/flags\/([^/]+)\/environments\/([^/?#]+)$/);
      if (method === 'PUT' && m) {
        const envId = m[2];
        let body: unknown = {};
        try {
          body = init?.body ? JSON.parse(String(init.body)) : {};
        } catch {
          body = {};
        }
        opts.onPut?.(envId, body);
        if (opts.putStatus && opts.putStatus !== 200) {
          return jsonResponse(opts.putBody ?? { error: 'boom' }, opts.putStatus);
        }
        const patch = body as { enabled?: boolean; value?: unknown };
        const row: FlagEnvironmentState = {
          flag_id: 'flag-1',
          environment_id: envId,
          enabled: patch.enabled ?? false,
          value: patch.value,
        };
        return jsonResponse(row);
      }
      return readHandler(url);
    };
  }

  it('tapping env toggle on disabled env issues PUT { enabled: true } and reflects state', async () => {
    const calls: Array<{ envId: string; body: unknown }> = [];
    const fetchMock = makeFetchMockMethodAware(
      defaultHandlerWithPut({
        envStates: [],
        onPut: (envId, body) => calls.push({ envId, body }),
      }),
    );
    setFetch(fetchMock);
    renderAt('/m/orgs/acme/flags/payments/flag-1');
    await screen.findByText('checkout_v2');

    const toggle = await screen.findByRole('switch', { name: /Toggle Development/ });
    expect(toggle).not.toBeChecked();
    await userEvent.click(toggle);

    await waitFor(() => {
      expect(calls).toContainEqual({ envId: 'env-dev', body: { enabled: true } });
    });
    await waitFor(() =>
      expect(
        screen.getByRole('switch', { name: /Toggle Development/ }),
      ).toBeChecked(),
    );
  });

  it('tapping env toggle on enabled env issues PUT { enabled: false }', async () => {
    const calls: Array<{ envId: string; body: unknown }> = [];
    const envStates: FlagEnvironmentState[] = [
      { flag_id: 'flag-1', environment_id: 'env-dev', enabled: true },
    ];
    const fetchMock = makeFetchMockMethodAware(
      defaultHandlerWithPut({
        envStates,
        onPut: (envId, body) => calls.push({ envId, body }),
      }),
    );
    setFetch(fetchMock);
    renderAt('/m/orgs/acme/flags/payments/flag-1');
    await screen.findByText('checkout_v2');

    const toggle = await screen.findByRole('switch', { name: /Toggle Development/ });
    await waitFor(() => expect(toggle).toBeChecked());
    await userEvent.click(toggle);

    await waitFor(() => {
      expect(calls).toContainEqual({ envId: 'env-dev', body: { enabled: false } });
    });
  });

  it('boolean flag: editing default value via select issues PUT { value: "true" }', async () => {
    const calls: Array<{ envId: string; body: unknown }> = [];
    const fetchMock = makeFetchMockMethodAware(
      defaultHandlerWithPut({
        flag: makeFlag({ flag_type: 'boolean', default_value: 'false' }),
        onPut: (envId, body) => calls.push({ envId, body }),
      }),
    );
    setFetch(fetchMock);
    renderAt('/m/orgs/acme/flags/payments/flag-1');
    await screen.findByText('checkout_v2');

    await userEvent.click(screen.getByRole('button', { name: /Development/ }));
    const select = (await screen.findAllByRole('combobox'))[0];
    await userEvent.selectOptions(select, 'true');

    await waitFor(() => {
      expect(calls).toContainEqual({ envId: 'env-dev', body: { value: 'true' } });
    });
  });

  it('string flag: editing default value commits on blur', async () => {
    const calls: Array<{ envId: string; body: unknown }> = [];
    const fetchMock = makeFetchMockMethodAware(
      defaultHandlerWithPut({
        flag: makeFlag({ flag_type: 'string', default_value: 'old' }),
        onPut: (envId, body) => calls.push({ envId, body }),
      }),
    );
    setFetch(fetchMock);
    renderAt('/m/orgs/acme/flags/payments/flag-1');
    await screen.findByText('checkout_v2');

    await userEvent.click(screen.getByRole('button', { name: /Development/ }));
    const inputs = await screen.findAllByRole('textbox');
    const input = inputs[0];
    await userEvent.clear(input);
    await userEvent.type(input, 'newval');
    input.blur();

    await waitFor(() => {
      expect(calls).toContainEqual({ envId: 'env-dev', body: { value: 'newval' } });
    });
  });

  it('string flag: editing default value commits on Enter keypress', async () => {
    const calls: Array<{ envId: string; body: unknown }> = [];
    const fetchMock = makeFetchMockMethodAware(
      defaultHandlerWithPut({
        flag: makeFlag({ flag_type: 'string', default_value: 'old' }),
        onPut: (envId, body) => calls.push({ envId, body }),
      }),
    );
    setFetch(fetchMock);
    renderAt('/m/orgs/acme/flags/payments/flag-1');
    await screen.findByText('checkout_v2');

    await userEvent.click(screen.getByRole('button', { name: /Development/ }));
    const inputs = await screen.findAllByRole('textbox');
    const input = inputs[0];
    await userEvent.clear(input);
    await userEvent.type(input, 'newval{Enter}');

    await waitFor(() => {
      expect(calls).toContainEqual({ envId: 'env-dev', body: { value: 'newval' } });
    });
  });

  it('on PUT 500, toggle reverts and an error message renders', async () => {
    const fetchMock = makeFetchMockMethodAware(
      defaultHandlerWithPut({
        envStates: [],
        putStatus: 500,
        putBody: { error: 'server boom' },
      }),
    );
    setFetch(fetchMock);
    renderAt('/m/orgs/acme/flags/payments/flag-1');
    await screen.findByText('checkout_v2');

    const toggle = await screen.findByRole('switch', { name: /Toggle Development/ });
    expect(toggle).not.toBeChecked();
    await userEvent.click(toggle);

    await waitFor(() => {
      expect(
        screen.getByRole('switch', { name: /Toggle Development/ }),
      ).not.toBeChecked();
    });
    expect(await screen.findByText(/couldn't save: server boom/)).toBeInTheDocument();
  });

  it('json flag: renders an "Edit JSON on desktop" deep-link instead of an input', async () => {
    setFetch(
      makeFetchMockMethodAware(
        defaultHandlerWithPut({
          flag: makeFlag({ flag_type: 'json', default_value: '{"a":1}' }),
        }),
      ),
    );
    renderAt('/m/orgs/acme/flags/payments/flag-1');
    await screen.findByText('checkout_v2');

    await userEvent.click(screen.getByRole('button', { name: /Development/ }));
    const link = await screen.findByRole('link', { name: /Edit JSON on desktop/i });
    expect(link).toBeInTheDocument();
    expect(link.getAttribute('href')).toContain('/flags/flag-1');
  });
});
