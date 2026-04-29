import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import { FlagProjectPickerPage } from './FlagProjectPickerPage';
import { setFetch } from '../api';

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/m/orgs/:orgSlug/flags" element={<FlagProjectPickerPage />} />
        <Route
          path="/m/orgs/:orgSlug/flags/:projectSlug"
          element={<div data-testid="flag-list">flag list</div>}
        />
      </Routes>
    </MemoryRouter>,
  );
}

describe('FlagProjectPickerPage', () => {
  let fetchMock: ReturnType<typeof vi.fn<typeof fetch>>;
  beforeEach(() => {
    fetchMock = vi.fn<typeof fetch>();
    setFetch(fetchMock);
    localStorage.clear();
    localStorage.setItem('ds_token', 'header.payload.sig');
  });

  it('renders loading state initially', () => {
    fetchMock.mockImplementation(() => new Promise(() => {}));
    renderAt('/m/orgs/acme/flags');
    expect(screen.getByText(/Loading projects/i)).toBeInTheDocument();
  });

  it('auto-redirects to flag list when org has exactly one project', async () => {
    fetchMock.mockResolvedValue(
      new Response(
        JSON.stringify({
          projects: [
            { id: 'p1', name: 'Web', slug: 'web', org_id: 'o1' },
          ],
        }),
        { status: 200 },
      ),
    );
    renderAt('/m/orgs/acme/flags');
    expect(await screen.findByTestId('flag-list')).toBeInTheDocument();
  });

  it('renders one link per project when multiple projects exist', async () => {
    fetchMock.mockResolvedValue(
      new Response(
        JSON.stringify({
          projects: [
            { id: 'p1', name: 'Web', slug: 'web', org_id: 'o1' },
            { id: 'p2', name: 'API', slug: 'api', org_id: 'o1' },
          ],
        }),
        { status: 200 },
      ),
    );
    renderAt('/m/orgs/acme/flags');
    const webLink = await screen.findByRole('link', { name: /Web/i });
    const apiLink = await screen.findByRole('link', { name: /API/i });
    expect(webLink).toHaveAttribute('href', '/m/orgs/acme/flags/web');
    expect(apiLink).toHaveAttribute('href', '/m/orgs/acme/flags/api');
  });

  it('renders empty state when org has no projects', async () => {
    fetchMock.mockResolvedValue(
      new Response(JSON.stringify({ projects: [] }), { status: 200 }),
    );
    renderAt('/m/orgs/acme/flags');
    expect(await screen.findByText(/No projects in this org yet/i)).toBeInTheDocument();
  });

  it('surfaces error message when fetch fails', async () => {
    fetchMock.mockResolvedValue(
      new Response(JSON.stringify({ error: 'Boom' }), { status: 500 }),
    );
    renderAt('/m/orgs/acme/flags');
    await waitFor(() => expect(screen.getByText(/Boom/)).toBeInTheDocument());
  });
});
