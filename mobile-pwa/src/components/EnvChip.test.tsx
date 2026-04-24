import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { EnvChip } from './EnvChip';
import type { OrgStatusEnvCell } from '../types';

function cell(partial: Partial<OrgStatusEnvCell> = {}): OrgStatusEnvCell {
  return {
    environment: { id: 'e1', slug: 'prod', name: 'Production' },
    current_deployment: null,
    health: { state: 'healthy', source: 'agent', staleness: 'fresh' },
    never_deployed: false,
    ...partial,
  };
}

describe('EnvChip', () => {
  it('renders the env slug', () => {
    render(<EnvChip cell={cell()} onTap={() => {}} />);
    expect(screen.getByText('prod')).toBeInTheDocument();
  });
  it('shows the never-deployed state when applicable', () => {
    render(<EnvChip cell={cell({ never_deployed: true })} onTap={() => {}} />);
    const chip = screen.getByText(/prod/);
    expect(chip).toHaveAttribute('data-state', 'never');
  });
  it('encodes the health state into data-state', () => {
    render(<EnvChip cell={cell({ health: { state: 'unhealthy', source: 'agent', staleness: 'fresh' } })} onTap={() => {}} />);
    expect(screen.getByText('prod')).toHaveAttribute('data-state', 'unhealthy');
  });
  it('calls onTap when clicked', async () => {
    const onTap = vi.fn();
    render(<EnvChip cell={cell()} onTap={onTap} />);
    await userEvent.click(screen.getByText('prod'));
    expect(onTap).toHaveBeenCalledTimes(1);
  });
});
