import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import { FlagListPage } from './FlagListPage';
import { setFetch } from '../api';
import type { Flag } from '../types';

const PROJECT = { id: 'proj-1', slug: 'payments', name: 'Payments', org_id: 'org-1' };
const APP = { id: 'app-1', slug: 'api', name: 'API', project_id: 'proj-1' };
const ENVS = [
  { id: 'env-dev', slug: 'dev', name: 'Development' },
  { id: 'env-prd', slug: 'prod', name: 'Production', is_production: true },
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
    expires_at: null,
    default_value: 'false',
    enabled: true,
    archived: false,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    ...partial,
  };
}

const FLAGS: Flag[] = [
  makeFlag({ id: 'flag-1', key: 'checkout_v2', name: 'Checkout v2', category: 'release' }),
  makeFlag({ id: 'flag-2', key: 'new_dashboard', name: 'New Dashboard', category: 'feature' }),
  makeFlag({ id: 'flag-3', key: 'kill_switch_x', name: 'Kill Switch X', category: 'ops' }),
];

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

function defaultHandler(opts: {
  flags?: Flag[];
  apps?: typeof APP[];
  flagsErrorOn?: string;
} = {}): RouteHandler {
  const flags = opts.flags ?? FLAGS;
  const apps = opts.apps ?? [APP];
  return (url) => {
    if (url.includes('/orgs/acme/projects') && url.endsWith('/projects')) {
      return jsonResponse({ projects: [PROJECT] });
    }
    if (url.includes('/orgs/acme/environments')) {
      return jsonResponse({ environments: ENVS });
    }
    if (url.includes('/apps') && url.includes('/projects/payments/apps')) {
      return jsonResponse({ applications: apps });
    }
    if (url.startsWith('/api/v1/flags?')) {
      if (opts.flagsErrorOn === 'list') {
        return jsonResponse({ error: 'Boom' }, 500);
      }
      return jsonResponse({ flags });
    }
    if (url.match(/\/flags\/[^/]+\/environments/)) {
      const id = url.split('/flags/')[1].split('/')[0];
      return jsonResponse({
        environment_states: [
          { flag_id: id, environment_id: 'env-dev', enabled: true },
        ],
      });
    }
    return jsonResponse({ error: `unhandled ${url}` }, 404);
  };
}

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/m/orgs/:orgSlug/flags/:projectSlug" element={<FlagListPage />} />
        <Route
          path="/m/orgs/:orgSlug/flags/:projectSlug/apps/:appSlug"
          element={<FlagListPage />}
        />
      </Routes>
    </MemoryRouter>,
  );
}

describe('FlagListPage', () => {
  let fetchMock: ReturnType<typeof vi.fn<typeof fetch>>;

  beforeEach(() => {
    localStorage.clear();
    localStorage.setItem('ds_token', 'header.payload.sig');
  });

  it('renders loading state initially', () => {
    fetchMock = vi.fn<typeof fetch>(() => new Promise(() => {}));
    setFetch(fetchMock);
    renderAt('/m/orgs/acme/flags/payments');
    expect(screen.getByText(/Loading/i)).toBeInTheDocument();
  });

  it('renders one FlagRow per flag from the API response', async () => {
    fetchMock = makeFetchMock(defaultHandler());
    setFetch(fetchMock);
    renderAt('/m/orgs/acme/flags/payments');
    expect(await screen.findByText('checkout_v2')).toBeInTheDocument();
    expect(screen.getByText('new_dashboard')).toBeInTheDocument();
    expect(screen.getByText('kill_switch_x')).toBeInTheDocument();
    // Detail links use project-scoped path.
    const link = screen.getByRole('link', { name: /checkout_v2/i });
    expect(link).toHaveAttribute('href', '/m/orgs/acme/flags/payments/flag-1');
  });

  it('shows the project name as heading', async () => {
    fetchMock = makeFetchMock(defaultHandler());
    setFetch(fetchMock);
    renderAt('/m/orgs/acme/flags/payments');
    expect(await screen.findByRole('heading', { name: /Payments/ })).toBeInTheDocument();
  });

  it('search input filters rows by key/name (case-insensitive)', async () => {
    fetchMock = makeFetchMock(defaultHandler());
    setFetch(fetchMock);
    renderAt('/m/orgs/acme/flags/payments');
    await screen.findByText('checkout_v2');

    const search = screen.getByPlaceholderText(/Search flags/i);
    await userEvent.type(search, 'CHECKOUT');

    expect(screen.getByText('checkout_v2')).toBeInTheDocument();
    expect(screen.queryByText('new_dashboard')).not.toBeInTheDocument();
    expect(screen.queryByText('kill_switch_x')).not.toBeInTheDocument();
  });

  it('tapping a category chip filters the list to that category', async () => {
    fetchMock = makeFetchMock(defaultHandler());
    setFetch(fetchMock);
    renderAt('/m/orgs/acme/flags/payments');
    await screen.findByText('checkout_v2');

    await userEvent.click(screen.getByRole('button', { name: 'release' }));

    expect(screen.getByText('checkout_v2')).toBeInTheDocument();
    expect(screen.queryByText('new_dashboard')).not.toBeInTheDocument();
    expect(screen.queryByText('kill_switch_x')).not.toBeInTheDocument();
  });

  it('pre-fills filters from URL search params (?q=foo&category=release)', async () => {
    fetchMock = makeFetchMock(defaultHandler());
    setFetch(fetchMock);
    renderAt('/m/orgs/acme/flags/payments?q=checkout&category=release');
    await screen.findByText('checkout_v2');

    const search = screen.getByPlaceholderText(/Search flags/i) as HTMLInputElement;
    expect(search.value).toBe('checkout');
    expect(screen.getByRole('button', { name: 'release' })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
    expect(screen.queryByText('new_dashboard')).not.toBeInTheDocument();
    expect(screen.queryByText('kill_switch_x')).not.toBeInTheDocument();
  });

  it('app-scoped variant calls flagsApi.list with application_id in the query string', async () => {
    fetchMock = makeFetchMock(defaultHandler());
    setFetch(fetchMock);
    renderAt('/m/orgs/acme/flags/payments/apps/api');
    await screen.findByText('checkout_v2');

    const flagsCall = fetchMock.mock.calls
      .map((c) => String(c[0]))
      .find((u) => u.startsWith('/api/v1/flags?'));
    expect(flagsCall).toBeDefined();
    expect(flagsCall).toContain('application_id=app-1');
    // Heading shows project / app form.
    expect(screen.getByRole('heading', { name: /Payments/ })).toHaveTextContent('API');
  });

  it('renders empty state when project has no flags', async () => {
    fetchMock = makeFetchMock(defaultHandler({ flags: [] }));
    setFetch(fetchMock);
    renderAt('/m/orgs/acme/flags/payments');
    expect(await screen.findByText(/No flags in this project/i)).toBeInTheDocument();
  });

  it('surfaces error state when the flags fetch fails', async () => {
    fetchMock = makeFetchMock(defaultHandler({ flagsErrorOn: 'list' }));
    setFetch(fetchMock);
    renderAt('/m/orgs/acme/flags/payments');
    await waitFor(() => expect(screen.getByText(/Boom/)).toBeInTheDocument());
  });

  it('does not render the StaleBadge after a fresh successful fetch', async () => {
    fetchMock = makeFetchMock(defaultHandler());
    setFetch(fetchMock);
    renderAt('/m/orgs/acme/flags/payments');
    expect(await screen.findByText('checkout_v2')).toBeInTheDocument();
    // Fresh data (<30s) → badge renders nothing.
    expect(screen.queryByText(/Showing data from/i)).not.toBeInTheDocument();
  });
});
