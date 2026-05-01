import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import StagingHeaderBanner from './StagingHeaderBanner';

const mockList = vi.fn();
const mockDiscardAll = vi.fn();

vi.mock('@/api', () => ({
  stagingApi: {
    list: (...args: unknown[]) => mockList(...args),
    discardAll: (...args: unknown[]) => mockDiscardAll(...args),
  },
}));

function renderBanner(orgSlug = 'acme') {
  return render(
    <MemoryRouter initialEntries={[`/orgs/${orgSlug}`]}>
      <Routes>
        <Route path="/orgs/:orgSlug" element={<StagingHeaderBanner />} />
      </Routes>
    </MemoryRouter>,
  );
}

beforeEach(() => {
  mockList.mockReset();
  mockDiscardAll.mockReset();
});

describe('StagingHeaderBanner', () => {
  it('renders nothing when there are no pending changes', async () => {
    mockList.mockResolvedValue({ changes: [], count: 0 });
    renderBanner();
    await waitFor(() => expect(mockList).toHaveBeenCalled());
    expect(screen.queryByTestId('staging-banner')).not.toBeInTheDocument();
  });

  it('renders the count and singular phrasing for one row', async () => {
    mockList.mockResolvedValue({ changes: [{}], count: 1 });
    renderBanner();
    expect(await screen.findByTestId('staging-banner')).toBeInTheDocument();
    expect(screen.getByTestId('staging-banner')).toHaveTextContent(
      'You have 1 pending change',
    );
    expect(screen.getByTestId('staging-banner')).not.toHaveTextContent(
      'pending changes',
    );
  });

  it('renders plural phrasing for multiple rows and the deploy link', async () => {
    mockList.mockResolvedValue({ changes: [{}, {}, {}], count: 3 });
    renderBanner('foo');
    expect(await screen.findByText(/3 pending changes/)).toBeInTheDocument();
    const link = screen.getByRole('link', { name: /Review & Deploy/ });
    expect(link).toHaveAttribute('href', '/orgs/foo/deploy-changes');
  });
});
