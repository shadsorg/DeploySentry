import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import FlagCreatePage from './FlagCreatePage';
import { flagsApi, stagingApi, entitiesApi } from '@/api';
import { isProvisionalId } from '@/lib/provisional';

// Control staging state by mocking the hook directly.
let mockStagingEnabled = false;
vi.mock('@/hooks/useStagingEnabled', () => ({
  useStagingEnabled: () => mockStagingEnabled,
  setStagingEnabled: vi.fn(),
}));

vi.mock('@/api', () => ({
  flagsApi: { create: vi.fn() },
  stagingApi: { stage: vi.fn() },
  entitiesApi: {
    getProject: vi.fn().mockResolvedValue({ id: 'proj-uuid' }),
    getApp: vi.fn().mockResolvedValue({ id: 'app-uuid' }),
  },
}));

const ORG_SLUG = 'acme';
const PROJECT_SLUG = 'web';

function renderPage() {
  return render(
    <MemoryRouter initialEntries={[`/orgs/${ORG_SLUG}/projects/${PROJECT_SLUG}/flags/new`]}>
      <Routes>
        <Route path="/orgs/:orgSlug/projects/:projectSlug/flags/new" element={<FlagCreatePage />} />
        <Route path="/orgs/:orgSlug/projects/:projectSlug/flags" element={<div>List</div>} />
      </Routes>
    </MemoryRouter>,
  );
}

describe('FlagCreatePage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockStagingEnabled = false;
    // Reset mocks to their default resolved values after clearAllMocks
    (entitiesApi.getProject as ReturnType<typeof vi.fn>).mockResolvedValue({ id: 'proj-uuid' });
    (entitiesApi.getApp as ReturnType<typeof vi.fn>).mockResolvedValue({ id: 'app-uuid' });
  });

  it('routes through flagsApi.create when staging is disabled', async () => {
    mockStagingEnabled = false;
    (flagsApi.create as ReturnType<typeof vi.fn>).mockResolvedValue({ id: 'real-uuid' });

    renderPage();
    await screen.findByLabelText(/key/i);
    await userEvent.type(screen.getByLabelText(/key/i), 'my-flag');
    await userEvent.type(screen.getByLabelText(/name/i), 'My Flag');
    // default_value is required by the form
    await userEvent.type(screen.getByLabelText(/default value/i), 'false');
    await userEvent.click(screen.getByRole('button', { name: /create/i }));

    await waitFor(() => expect(flagsApi.create).toHaveBeenCalledTimes(1));
    expect(stagingApi.stage).not.toHaveBeenCalled();
  });

  it('routes through stagingApi.stage when staging is enabled', async () => {
    mockStagingEnabled = true;
    (stagingApi.stage as ReturnType<typeof vi.fn>).mockResolvedValue({ id: 'staged-row' });

    renderPage();
    await screen.findByLabelText(/key/i);
    await userEvent.type(screen.getByLabelText(/key/i), 'my-flag');
    await userEvent.type(screen.getByLabelText(/name/i), 'My Flag');
    // default_value is required by the form
    await userEvent.type(screen.getByLabelText(/default value/i), 'false');
    await userEvent.click(screen.getByRole('button', { name: /create/i }));

    await waitFor(() => expect(stagingApi.stage).toHaveBeenCalledTimes(1));
    expect(flagsApi.create).not.toHaveBeenCalled();

    const [orgArg, body] = (stagingApi.stage as ReturnType<typeof vi.fn>).mock.calls[0];
    expect(orgArg).toBe(ORG_SLUG);
    expect(body.resource_type).toBe('flag');
    expect(body.action).toBe('create');
    expect(typeof body.provisional_id).toBe('string');
    expect(isProvisionalId(body.provisional_id)).toBe(true);
    expect(body.new_value).toMatchObject({ key: 'my-flag', name: 'My Flag' });
  });
});
