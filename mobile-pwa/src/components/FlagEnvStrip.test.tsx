import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { FlagEnvStrip } from './FlagEnvStrip';
import type { OrgEnvironment, FlagEnvironmentState } from '../types';

const ENVS: OrgEnvironment[] = [
  { id: 'env-dev', slug: 'dev', name: 'Development' },
  { id: 'env-stg', slug: 'staging', name: 'Staging' },
  { id: 'env-prd', slug: 'prod', name: 'Production', is_production: true },
];

describe('FlagEnvStrip', () => {
  it('renders one pip per environment', () => {
    render(<FlagEnvStrip environments={ENVS} states={[]} />);
    expect(screen.getByText('dev')).toBeInTheDocument();
    expect(screen.getByText('staging')).toBeInTheDocument();
    expect(screen.getByText('prod')).toBeInTheDocument();
  });

  it('marks data-on="true" when a matching state is enabled', () => {
    const states: FlagEnvironmentState[] = [
      { flag_id: 'f1', environment_id: 'env-dev', enabled: true },
      { flag_id: 'f1', environment_id: 'env-stg', enabled: false },
    ];
    render(<FlagEnvStrip environments={ENVS} states={states} />);
    expect(screen.getByText('dev')).toHaveAttribute('data-on', 'true');
    expect(screen.getByText('staging')).toHaveAttribute('data-on', 'false');
    expect(screen.getByText('prod')).toHaveAttribute('data-on', 'false');
  });

  it('defaults to data-on="false" when no state row exists for an env', () => {
    render(<FlagEnvStrip environments={ENVS} states={[]} />);
    ENVS.forEach((env) => {
      expect(screen.getByText(env.slug)).toHaveAttribute('data-on', 'false');
    });
  });

  it('exposes a non-interactive container with an aria-label', () => {
    const { container } = render(<FlagEnvStrip environments={ENVS} states={[]} />);
    const strip = container.querySelector('.m-flag-env-strip');
    expect(strip).not.toBeNull();
    expect(strip).toHaveAttribute('aria-label', 'Environment states');
    // No buttons -- pips should be plain spans.
    expect(container.querySelectorAll('button').length).toBe(0);
  });
});
