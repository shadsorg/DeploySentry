import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Routes, Route, useLocation } from 'react-router-dom';
import { TabBar } from './TabBar';

function LocationProbe() {
  const loc = useLocation();
  return <div data-testid="loc">{loc.pathname}</div>;
}

describe('TabBar', () => {
  it('renders three tabs and marks Status active on /orgs/:slug/status', () => {
    render(
      <MemoryRouter initialEntries={['/orgs/acme/status']}>
        <Routes>
          <Route
            path="/orgs/:orgSlug/*"
            element={
              <>
                <TabBar />
                <LocationProbe />
              </>
            }
          />
        </Routes>
      </MemoryRouter>,
    );
    expect(screen.getByRole('button', { name: /status/i })).toHaveAttribute('aria-current', 'page');
    expect(screen.getByRole('button', { name: /history/i })).not.toHaveAttribute('aria-current');
    expect(screen.getByRole('button', { name: /flags/i })).not.toHaveAttribute('aria-current');
  });

  it('navigates to history when History tab is clicked', async () => {
    render(
      <MemoryRouter initialEntries={['/orgs/acme/status']}>
        <Routes>
          <Route
            path="/orgs/:orgSlug/*"
            element={
              <>
                <TabBar />
                <LocationProbe />
              </>
            }
          />
        </Routes>
      </MemoryRouter>,
    );
    await userEvent.click(screen.getByRole('button', { name: /history/i }));
    expect(screen.getByTestId('loc').textContent).toBe('/orgs/acme/history');
  });

  it('marks History active on any /history/* drill-down', () => {
    render(
      <MemoryRouter initialEntries={['/orgs/acme/history/deploy-123']}>
        <Routes>
          <Route
            path="/orgs/:orgSlug/*"
            element={
              <>
                <TabBar />
                <LocationProbe />
              </>
            }
          />
        </Routes>
      </MemoryRouter>,
    );
    expect(screen.getByRole('button', { name: /history/i })).toHaveAttribute('aria-current', 'page');
  });
});
