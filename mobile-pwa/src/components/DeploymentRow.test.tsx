import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { DeploymentRow } from './DeploymentRow';
import type { OrgDeploymentRow } from '../types';

function row(partial: Partial<OrgDeploymentRow> = {}): OrgDeploymentRow {
  return {
    id: 'd1',
    application_id: 'a1',
    environment_id: 'e1',
    version: 'v2.1.0',
    strategy: 'canary',
    status: 'completed',
    traffic_percent: 100,
    created_by: 'u1',
    created_at: new Date(Date.now() - 5 * 60_000).toISOString(),
    updated_at: '',
    completed_at: null,
    application: { id: 'a1', slug: 'api', name: 'API' },
    environment: { id: 'e1', slug: 'prod', name: 'Production' },
    project: { id: 'p1', slug: 'payments', name: 'Payments' },
    ...partial,
  };
}

describe('DeploymentRow', () => {
  it('renders version, env slug, app name, status', () => {
    render(<DeploymentRow row={row()} onTap={() => {}} />);
    expect(screen.getByText('v2.1.0')).toBeInTheDocument();
    expect(screen.getByText('prod')).toBeInTheDocument();
    expect(screen.getByText('API')).toBeInTheDocument();
    expect(screen.getByText('COMPLETED')).toBeInTheDocument();
  });

  it('renders relative age when created within the last hour', () => {
    render(<DeploymentRow row={row()} onTap={() => {}} />);
    expect(screen.getByText(/5m ago/i)).toBeInTheDocument();
  });

  it('calls onTap with the row when clicked', async () => {
    const onTap = vi.fn();
    render(<DeploymentRow row={row()} onTap={onTap} />);
    await userEvent.click(screen.getByRole('button'));
    expect(onTap).toHaveBeenCalledTimes(1);
    expect(onTap.mock.calls[0][0].id).toBe('d1');
  });
});
