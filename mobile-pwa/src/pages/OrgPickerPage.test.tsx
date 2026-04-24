import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Routes, Route, useLocation } from 'react-router-dom';
import { OrgPickerPage } from './OrgPickerPage';
import { setFetch } from '../api';

function LocationProbe() {
  const loc = useLocation();
  return <div data-testid="loc">{loc.pathname}</div>;
}

describe('OrgPickerPage', () => {
  let fetchMock: ReturnType<typeof vi.fn<typeof fetch>>;
  beforeEach(() => {
    fetchMock = vi.fn<typeof fetch>();
    setFetch(fetchMock);
    localStorage.clear();
    localStorage.setItem('ds_token', 'header.payload.sig');
  });

  it('auto-redirects to /orgs/:slug/status when user has exactly one org', async () => {
    fetchMock.mockResolvedValue(
      new Response(
        JSON.stringify({
          organizations: [{ id: '1', name: 'Acme', slug: 'acme', created_at: '', updated_at: '' }],
        }),
        { status: 200 },
      ),
    );
    render(
      <MemoryRouter initialEntries={['/orgs']}>
        <Routes>
          <Route path="/orgs" element={<OrgPickerPage />} />
          <Route path="/orgs/:orgSlug/status" element={<LocationProbe />} />
        </Routes>
      </MemoryRouter>,
    );
    await waitFor(() => expect(screen.getByTestId('loc').textContent).toBe('/orgs/acme/status'));
  });

  it('renders picker when user has multiple orgs', async () => {
    fetchMock.mockResolvedValue(
      new Response(
        JSON.stringify({
          organizations: [
            { id: '1', name: 'Acme', slug: 'acme', created_at: '', updated_at: '' },
            { id: '2', name: 'Beta', slug: 'beta', created_at: '', updated_at: '' },
          ],
        }),
        { status: 200 },
      ),
    );
    render(
      <MemoryRouter initialEntries={['/orgs']}>
        <Routes>
          <Route path="/orgs" element={<OrgPickerPage />} />
          <Route path="/orgs/:orgSlug/status" element={<LocationProbe />} />
        </Routes>
      </MemoryRouter>,
    );
    expect(await screen.findByText('Acme')).toBeInTheDocument();
    expect(screen.getByText('Beta')).toBeInTheDocument();
    await userEvent.click(screen.getByText('Beta'));
    await waitFor(() => expect(screen.getByTestId('loc').textContent).toBe('/orgs/beta/status'));
    expect(localStorage.getItem('ds_active_org')).toBe('beta');
  });
});
