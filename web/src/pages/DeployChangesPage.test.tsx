import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import DeployChangesPage from './DeployChangesPage';
import { computeConflicts } from '@/components/staging/conflicts';
import type { StagedChange } from '@/api';

const mockList = vi.fn();
const mockCommit = vi.fn();
const mockDiscardOne = vi.fn();

vi.mock('@/api', () => ({
  stagingApi: {
    list: (...args: unknown[]) => mockList(...args),
    commit: (...args: unknown[]) => mockCommit(...args),
    discardOne: (...args: unknown[]) => mockDiscardOne(...args),
  },
}));

function row(overrides: Partial<StagedChange>): StagedChange {
  return {
    id: 'r1',
    user_id: 'u1',
    org_id: 'o1',
    resource_type: 'flag',
    resource_id: 'f1',
    action: 'toggle',
    new_value: { enabled: true },
    created_at: '2026-05-01T10:00:00Z',
    updated_at: '2026-05-01T10:00:00Z',
    ...overrides,
  };
}

function renderPage(orgSlug = 'acme') {
  return render(
    <MemoryRouter initialEntries={[`/orgs/${orgSlug}/deploy-changes`]}>
      <Routes>
        <Route path="/orgs/:orgSlug/deploy-changes" element={<DeployChangesPage />} />
      </Routes>
    </MemoryRouter>,
  );
}

beforeEach(() => {
  mockList.mockReset();
  mockCommit.mockReset();
  mockDiscardOne.mockReset();
});

describe('DeployChangesPage', () => {
  it('shows the empty state when no rows are pending', async () => {
    mockList.mockResolvedValue({ changes: [], count: 0 });
    renderPage();
    await waitFor(() => expect(mockList).toHaveBeenCalled());
    expect(screen.getByText(/No pending changes/)).toBeInTheDocument();
  });

  it('groups rows by resource_type and selects all by default', async () => {
    mockList.mockResolvedValue({
      changes: [
        row({ id: 'a', resource_type: 'flag', resource_id: 'f1' }),
        row({ id: 'b', resource_type: 'setting', resource_id: 'g1', action: 'update' }),
        row({ id: 'c', resource_type: 'flag', resource_id: 'f2' }),
      ],
      count: 3,
    });
    renderPage();
    expect(await screen.findByText(/Feature flags/)).toBeInTheDocument();
    expect(screen.getByText(/Settings/)).toBeInTheDocument();
    expect(screen.getByText(/3 of 3 selected/)).toBeInTheDocument();
  });

  it('Deploy button posts the selected ids and refreshes', async () => {
    mockList
      .mockResolvedValueOnce({ changes: [row({ id: 'a' })], count: 1 })
      .mockResolvedValueOnce({ changes: [], count: 0 });
    mockCommit.mockResolvedValue({ committed_ids: ['a'] });
    const user = userEvent.setup();
    renderPage();
    await screen.findByTestId('deploy-button');
    await user.click(screen.getByTestId('deploy-button'));
    await waitFor(() => expect(mockCommit).toHaveBeenCalledWith('acme', ['a']));
    expect(await screen.findByText(/Deployed 1 change/)).toBeInTheDocument();
  });

  it('per-row Discard button removes the row from the list', async () => {
    mockList.mockResolvedValue({
      changes: [row({ id: 'a' }), row({ id: 'b', resource_id: 'f2' })],
      count: 2,
    });
    mockDiscardOne.mockResolvedValue({ discarded: 'a' });
    const user = userEvent.setup();
    renderPage();
    await screen.findByTestId('staging-row-a');
    const discardA = screen.getAllByText('Discard')[0];
    await user.click(discardA);
    await waitFor(() => expect(mockDiscardOne).toHaveBeenCalledWith('acme', 'a'));
    await waitFor(() => expect(screen.queryByTestId('staging-row-a')).not.toBeInTheDocument());
  });

  it('flags partial-failure commit and surfaces the reason', async () => {
    mockList.mockResolvedValue({ changes: [row({ id: 'a' })], count: 1 });
    mockCommit.mockResolvedValue({
      committed_ids: [],
      failed_id: 'a',
      failed_reason: 'flag.toggle commit: ToggleFlag boom',
    });
    const user = userEvent.setup();
    renderPage();
    await user.click(await screen.findByTestId('deploy-button'));
    expect(
      await screen.findByText(/Commit halted at a/),
    ).toBeInTheDocument();
  });
});

describe('computeConflicts', () => {
  it('returns empty when no two rows target the same resource+field', () => {
    const conflicts = computeConflicts([
      row({ id: 'a', resource_id: 'f1', field_path: 'name' }),
      row({ id: 'b', resource_id: 'f2', field_path: 'name' }),
    ]);
    expect(conflicts).toEqual({});
  });

  it('flags a later row whose old_value disagrees with the prior row', () => {
    const conflicts = computeConflicts([
      row({
        id: 'first',
        resource_id: 'f1',
        field_path: 'color',
        old_value: 'red',
        new_value: 'blue',
      }),
      row({
        id: 'second',
        resource_id: 'f1',
        field_path: 'color',
        old_value: 'green', // disagrees with first row's new_value=blue
        new_value: 'orange',
      }),
    ]);
    expect(conflicts).toEqual({ second: true });
  });
});
