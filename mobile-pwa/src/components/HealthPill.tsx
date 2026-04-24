import type { HealthState } from '../types';

const LABELS: Record<HealthState, string> = {
  healthy: 'HEALTHY',
  degraded: 'DEGRADED',
  unhealthy: 'UNHEALTHY',
  unknown: 'UNKNOWN',
};

export function HealthPill({ state }: { state: HealthState }) {
  return (
    <span className={`m-health-pill m-health-pill-${state}`} data-state={state}>
      {LABELS[state]}
    </span>
  );
}
