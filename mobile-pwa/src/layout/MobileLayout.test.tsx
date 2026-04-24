import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import { MobileLayout } from './MobileLayout';

describe('MobileLayout', () => {
  it('renders top bar, outlet content, and tab bar', () => {
    render(
      <MemoryRouter initialEntries={['/orgs/acme/status']}>
        <Routes>
          <Route path="/orgs/:orgSlug" element={<MobileLayout />}>
            <Route path="status" element={<div>StatusScreen</div>} />
          </Route>
        </Routes>
      </MemoryRouter>,
    );
    expect(screen.getByText('acme')).toBeInTheDocument();
    expect(screen.getByText('StatusScreen')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /flags/i })).toBeInTheDocument();
  });
});
