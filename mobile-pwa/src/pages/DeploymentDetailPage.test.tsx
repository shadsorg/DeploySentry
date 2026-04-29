import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import { DeploymentDetailPage } from './DeploymentDetailPage';
import type { OrgDeploymentRow } from '../types';

function row(): OrgDeploymentRow {
  return {
    id: 'd1',
    application_id: 'a1',
    environment_id: 'e1',
    version: 'v2.1.0',
    commit_sha: 'abc1234',
    strategy: 'canary',
    status: 'completed',
    mode: 'orchestrate',
    source: 'github-actions',
    traffic_percent: 100,
    created_by: 'u1',
    created_at: '2026-04-26T12:00:00Z',
    updated_at: '2026-04-26T12:05:00Z',
    started_at: '2026-04-26T12:01:00Z',
    completed_at: '2026-04-26T12:05:00Z',
    application: { id: 'a1', slug: 'api', name: 'API' },
    environment: { id: 'e1', slug: 'prod', name: 'Production' },
    project: { id: 'p1', slug: 'pay', name: 'Payments' },
  };
}

describe('DeploymentDetailPage', () => {
  it('renders all key fields when state is provided', () => {
    render(
      <MemoryRouter
        initialEntries={[{ pathname: '/orgs/acme/history/d1', state: { row: row() } }]}
      >
        <Routes>
          <Route path="/orgs/:orgSlug/history/:deploymentId" element={<DeploymentDetailPage />} />
        </Routes>
      </MemoryRouter>,
    );
    expect(screen.getByText('v2.1.0')).toBeInTheDocument();
    expect(screen.getByText('COMPLETED')).toBeInTheDocument();
    expect(screen.getByText(/abc1234/i)).toBeInTheDocument();
    expect(screen.getByText('Payments')).toBeInTheDocument();
    expect(screen.getByText('API')).toBeInTheDocument();
    expect(screen.getByText('prod')).toBeInTheDocument();
    expect(screen.getByText(/orchestrate/i)).toBeInTheDocument();
    expect(screen.getByText(/github-actions/i)).toBeInTheDocument();
  });

  it('falls back gracefully when no row is in state', () => {
    render(
      <MemoryRouter initialEntries={['/orgs/acme/history/d1']}>
        <Routes>
          <Route path="/orgs/:orgSlug/history/:deploymentId" element={<DeploymentDetailPage />} />
        </Routes>
      </MemoryRouter>,
    );
    expect(screen.getByText(/return to history/i)).toBeInTheDocument();
  });
});
