import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ProjectCard } from './ProjectCard';
import type { OrgStatusProjectNode, OrgStatusApplicationNode } from '../types';

function app(slug: string, envs: string[] = ['prod']): OrgStatusApplicationNode {
  return {
    application: { id: `a-${slug}`, slug, name: slug.toUpperCase(), monitoring_links: null },
    environments: envs.map((e) => ({
      environment: { id: `e-${e}`, slug: e, name: e },
      current_deployment: null,
      health: { state: 'healthy', source: 'agent', staleness: 'fresh' },
      never_deployed: false,
    })),
  };
}

function project(partial: Partial<OrgStatusProjectNode> = {}): OrgStatusProjectNode {
  return {
    project: { id: 'p1', slug: 'payments', name: 'Payments' },
    aggregate_health: 'healthy',
    applications: [app('api'), app('web')],
    ...partial,
  };
}

describe('ProjectCard', () => {
  it('renders project name + aggregate health + app count when collapsed', () => {
    render(<ProjectCard project={project()} onEnvTap={() => {}} />);
    expect(screen.getByText('Payments')).toBeInTheDocument();
    expect(screen.getByText(/HEALTHY/i)).toBeInTheDocument();
    expect(screen.getByText(/2 apps/i)).toBeInTheDocument();
    expect(screen.queryByText('API')).not.toBeInTheDocument();
  });

  it('expands to show apps when tapped', async () => {
    render(<ProjectCard project={project()} onEnvTap={() => {}} />);
    await userEvent.click(screen.getByRole('button', { name: /payments/i }));
    expect(screen.getByText('API')).toBeInTheDocument();
    expect(screen.getByText('WEB')).toBeInTheDocument();
  });

  it('calls onEnvTap with the cell when an env chip is clicked', async () => {
    const onEnvTap = vi.fn();
    render(<ProjectCard project={project({ applications: [app('api', ['prod', 'staging'])] })} onEnvTap={onEnvTap} />);
    await userEvent.click(screen.getByRole('button', { name: /payments/i }));
    await userEvent.click(screen.getByRole('button', { name: 'prod' }));
    expect(onEnvTap).toHaveBeenCalledTimes(1);
    expect(onEnvTap.mock.calls[0][0].environment.slug).toBe('prod');
  });
});
