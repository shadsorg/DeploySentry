import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { StatusPill } from './StatusPill';

describe('StatusPill', () => {
  it('renders the status label uppercased', () => {
    render(<StatusPill status="completed" />);
    expect(screen.getByText('COMPLETED')).toBeInTheDocument();
  });
  it('encodes status into data-state', () => {
    render(<StatusPill status="failed" />);
    expect(screen.getByText('FAILED')).toHaveAttribute('data-state', 'failed');
  });
  it('renders all DeployStatus values as their uppercase form', () => {
    const statuses = ['pending', 'running', 'promoting', 'paused', 'completed', 'failed', 'rolled_back', 'cancelled'] as const;
    statuses.forEach((s) => {
      const { unmount } = render(<StatusPill status={s} />);
      expect(screen.getByText(s.toUpperCase())).toBeInTheDocument();
      unmount();
    });
  });
});
