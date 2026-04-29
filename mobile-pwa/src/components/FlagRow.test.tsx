import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { FlagRow } from './FlagRow';
import type { Flag, OrgEnvironment, FlagEnvironmentState } from '../types';

const ENVS: OrgEnvironment[] = [
  { id: 'env-dev', slug: 'dev', name: 'Development' },
  { id: 'env-prd', slug: 'prod', name: 'Production', is_production: true },
];

function makeFlag(partial: Partial<Flag> = {}): Flag {
  return {
    id: 'flag-1',
    project_id: 'proj-1',
    application_id: null,
    key: 'checkout_v2',
    name: 'Checkout v2',
    flag_type: 'boolean',
    category: 'release',
    is_permanent: false,
    expires_at: null,
    default_value: 'false',
    enabled: true,
    archived: false,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    ...partial,
  };
}

describe('FlagRow', () => {
  it('renders flag key, name, category badge, and env strip', () => {
    const flag = makeFlag();
    const states: FlagEnvironmentState[] = [
      { flag_id: 'flag-1', environment_id: 'env-dev', enabled: true },
    ];
    render(
      <MemoryRouter>
        <FlagRow flag={flag} detailHref="/m/orgs/acme/flags/payments/flag-1" environments={ENVS} states={states} />
      </MemoryRouter>,
    );
    expect(screen.getByText('checkout_v2')).toBeInTheDocument();
    expect(screen.getByText('Checkout v2')).toBeInTheDocument();
    // Category badge -- text matches category and data-category attribute set.
    const badge = screen.getByText('release');
    expect(badge).toHaveAttribute('data-category', 'release');
    // Env strip pips with correct data-on values.
    expect(screen.getByText('dev')).toHaveAttribute('data-on', 'true');
    expect(screen.getByText('prod')).toHaveAttribute('data-on', 'false');
  });

  it('wraps content in a link to detailHref', () => {
    const flag = makeFlag();
    render(
      <MemoryRouter>
        <FlagRow flag={flag} detailHref="/m/orgs/acme/flags/payments/flag-1" environments={ENVS} states={[]} />
      </MemoryRouter>,
    );
    const link = screen.getByRole('link');
    expect(link).toHaveAttribute('href', '/m/orgs/acme/flags/payments/flag-1');
  });

  it('renders flag.key in a monospace element', () => {
    const flag = makeFlag({ key: 'mono.key' });
    const { container } = render(
      <MemoryRouter>
        <FlagRow flag={flag} detailHref="/x" environments={[]} states={[]} />
      </MemoryRouter>,
    );
    const keyEl = container.querySelector('.m-flag-key');
    expect(keyEl).not.toBeNull();
    expect(keyEl?.textContent).toBe('mono.key');
  });
});
