import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Routes, Route, useLocation } from 'react-router-dom';
import { HistoryPage } from './HistoryPage';
import { setFetch } from '../api';

function rowFixture(partial: Record<string, unknown> = {}) {
  return {
    id: 'd1',
    application_id: 'a1',
    environment_id: 'e1',
    version: 'v2.1.0',
    strategy: 'canary',
    status: 'completed',
    traffic_percent: 100,
    created_by: 'u1',
    created_at: new Date(Date.now() - 5 * 60_000).toISOString(),
    updated_at: '',
    completed_at: null,
    application: { id: 'a1', slug: 'api', name: 'API' },
    environment: { id: 'e1', slug: 'prod', name: 'Production' },
    project: { id: 'p1', slug: 'pay', name: 'Payments' },
    ...partial,
  };
}

function LocationProbe() {
  const loc = useLocation();
  return <div data-testid="loc">{loc.pathname}</div>;
}

describe('HistoryPage', () => {
  let fetchMock: ReturnType<typeof vi.fn<typeof fetch>>;
  beforeEach(() => {
    fetchMock = vi.fn<typeof fetch>();
    setFetch(fetchMock);
    localStorage.clear();
    localStorage.setItem('ds_token', 'header.payload.sig');
  });

  function renderAt(path: string) {
    return render(
      <MemoryRouter initialEntries={[path]}>
        <Routes>
          <Route path="/orgs/:orgSlug/history" element={<HistoryPage />} />
          <Route path="/orgs/:orgSlug/history/:deploymentId" element={<LocationProbe />} />
        </Routes>
      </MemoryRouter>,
    );
  }

  it('renders deployment rows from the response', async () => {
    fetchMock.mockImplementation(async (url) => {
      const u = String(url);
      if (u.includes('/projects')) return new Response(JSON.stringify({ projects: [] }), { status: 200 });
      return new Response(JSON.stringify({ deployments: [rowFixture()] }), { status: 200 });
    });
    renderAt('/orgs/acme/history');
    expect(await screen.findByText('v2.1.0')).toBeInTheDocument();
  });

  it('renders an empty state when zero deployments', async () => {
    fetchMock.mockImplementation(async (url) => {
      const u = String(url);
      if (u.includes('/projects')) return new Response(JSON.stringify({ projects: [] }), { status: 200 });
      return new Response(JSON.stringify({ deployments: [] }), { status: 200 });
    });
    renderAt('/orgs/acme/history');
    expect(await screen.findByText(/no deployments/i)).toBeInTheDocument();
  });

  it('navigates to /history/:id when a row is tapped', async () => {
    fetchMock.mockImplementation(async (url) => {
      const u = String(url);
      if (u.includes('/projects')) return new Response(JSON.stringify({ projects: [] }), { status: 200 });
      return new Response(JSON.stringify({ deployments: [rowFixture()] }), { status: 200 });
    });
    renderAt('/orgs/acme/history');
    const row = await screen.findByText('v2.1.0');
    await userEvent.click(row);
    await waitFor(() =>
      expect(screen.getByTestId('loc').textContent).toBe('/orgs/acme/history/d1'),
    );
  });

  it('does not render the StaleBadge after a fresh successful fetch', async () => {
    fetchMock.mockImplementation(async (url) => {
      const u = String(url);
      if (u.includes('/projects')) return new Response(JSON.stringify({ projects: [] }), { status: 200 });
      return new Response(JSON.stringify({ deployments: [rowFixture()] }), { status: 200 });
    });
    renderAt('/orgs/acme/history');
    expect(await screen.findByText('v2.1.0')).toBeInTheDocument();
    // Fresh data (<30s) → badge renders nothing.
    expect(screen.queryByText(/Showing data from/i)).not.toBeInTheDocument();
  });

  it('refetches with status=failed when the Failed chip is tapped', async () => {
    fetchMock.mockImplementation(async (url) => {
      const u = String(url);
      if (u.includes('/projects')) return new Response(JSON.stringify({ projects: [] }), { status: 200 });
      return new Response(JSON.stringify({ deployments: [] }), { status: 200 });
    });
    renderAt('/orgs/acme/history');
    await screen.findByText(/no deployments/i);
    await userEvent.click(screen.getByRole('button', { name: 'Failed' }));
    await waitFor(() => {
      const calls = fetchMock.mock.calls.map((c) => String(c[0]));
      expect(calls.some((u) => u.includes('/orgs/acme/deployments') && u.includes('status=failed'))).toBe(true);
    });
  });
});
