import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import { StatusPage } from './StatusPage';
import { setFetch } from '../api';

const FIXTURE = {
  org: { id: '1', slug: 'acme', name: 'Acme' },
  generated_at: '2026-04-24T12:00:00Z',
  projects: [
    {
      project: { id: 'p1', slug: 'payments', name: 'Payments' },
      aggregate_health: 'healthy',
      applications: [
        {
          application: { id: 'a1', slug: 'api', name: 'API', monitoring_links: null },
          environments: [
            {
              environment: { id: 'e1', slug: 'prod', name: 'Production' },
              current_deployment: null,
              health: { state: 'healthy', source: 'agent', staleness: 'fresh' },
              never_deployed: false,
            },
          ],
        },
      ],
    },
  ],
};

describe('StatusPage', () => {
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
          <Route path="/orgs/:orgSlug/status" element={<StatusPage />} />
        </Routes>
      </MemoryRouter>,
    );
  }

  it('renders project cards when the fetch succeeds', async () => {
    fetchMock.mockResolvedValue(new Response(JSON.stringify(FIXTURE), { status: 200 }));
    renderAt('/orgs/acme/status');
    expect(await screen.findByText('Payments')).toBeInTheDocument();
  });

  it('renders an error message when the fetch fails', async () => {
    fetchMock.mockResolvedValue(new Response(JSON.stringify({ error: 'boom' }), { status: 500 }));
    renderAt('/orgs/acme/status');
    expect(await screen.findByText(/boom|failed/i)).toBeInTheDocument();
  });

  it('renders an empty state when the org has zero projects', async () => {
    fetchMock.mockResolvedValue(
      new Response(JSON.stringify({ ...FIXTURE, projects: [] }), { status: 200 }),
    );
    renderAt('/orgs/acme/status');
    expect(await screen.findByText(/no projects/i)).toBeInTheDocument();
  });
});
