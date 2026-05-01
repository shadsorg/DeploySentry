import { describe, it, expect, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import StagingModeToggle from './StagingModeToggle';

beforeEach(() => {
  localStorage.clear();
});

describe('StagingModeToggle', () => {
  it('renders the off state by default and offers to enable', () => {
    render(<StagingModeToggle orgSlug="acme" />);
    expect(screen.getByText(/Staging mode is off/)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Enable staging/ })).toBeInTheDocument();
  });

  it('clicking Enable flips the on/off label and the button intent', async () => {
    const user = userEvent.setup();
    render(<StagingModeToggle orgSlug="acme" />);
    await user.click(screen.getByRole('button', { name: /Enable staging/ }));
    expect(screen.getByText(/Staging mode is on/)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Disable staging/ })).toBeInTheDocument();
  });

  it('reads the per-org localStorage key on mount', () => {
    localStorage.setItem('ds_staging_enabled:acme', 'true');
    render(<StagingModeToggle orgSlug="acme" />);
    expect(screen.getByText(/Staging mode is on/)).toBeInTheDocument();
  });

  it('persists the new state in the per-org localStorage key', async () => {
    const user = userEvent.setup();
    render(<StagingModeToggle orgSlug="acme" />);
    await user.click(screen.getByRole('button', { name: /Enable staging/ }));
    expect(localStorage.getItem('ds_staging_enabled:acme')).toBe('true');
    await user.click(screen.getByRole('button', { name: /Disable staging/ }));
    expect(localStorage.getItem('ds_staging_enabled:acme')).toBeNull();
  });
});
