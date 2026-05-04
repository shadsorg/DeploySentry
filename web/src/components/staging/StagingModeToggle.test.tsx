import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import StagingModeToggle from './StagingModeToggle';

const mockUseStagingEnabled = vi.fn();
const mockSetStagingEnabled = vi.fn();

vi.mock('@/hooks/useStagingEnabled', () => ({
  useStagingEnabled: (orgSlug?: string) => mockUseStagingEnabled(orgSlug),
  setStagingEnabled: (...args: unknown[]) => mockSetStagingEnabled(...args),
}));

beforeEach(() => {
  mockUseStagingEnabled.mockReset();
  mockSetStagingEnabled.mockReset().mockResolvedValue(undefined);
});

describe('StagingModeToggle', () => {
  it('renders off when the hook returns false', () => {
    mockUseStagingEnabled.mockReturnValue(false);
    render(<StagingModeToggle orgSlug="acme" />);
    expect(screen.getByText(/Staging mode is off/)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Enable staging/ })).toBeInTheDocument();
  });

  it('renders on when the hook returns true', () => {
    mockUseStagingEnabled.mockReturnValue(true);
    render(<StagingModeToggle orgSlug="acme" />);
    expect(screen.getByText(/Staging mode is on/)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Disable staging/ })).toBeInTheDocument();
  });

  it('clicking Enable calls setStagingEnabled(orgSlug, true)', async () => {
    mockUseStagingEnabled.mockReturnValue(false);
    const user = userEvent.setup();
    render(<StagingModeToggle orgSlug="acme" />);
    await user.click(screen.getByRole('button', { name: /Enable staging/ }));
    expect(mockSetStagingEnabled).toHaveBeenCalledWith('acme', true);
  });

  it('clicking Disable calls setStagingEnabled(orgSlug, false)', async () => {
    mockUseStagingEnabled.mockReturnValue(true);
    const user = userEvent.setup();
    render(<StagingModeToggle orgSlug="acme" />);
    await user.click(screen.getByRole('button', { name: /Disable staging/ }));
    expect(mockSetStagingEnabled).toHaveBeenCalledWith('acme', false);
  });
});
