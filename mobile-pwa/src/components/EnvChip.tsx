import type { OrgStatusEnvCell } from '../types';

export function EnvChip({ cell, onTap }: { cell: OrgStatusEnvCell; onTap: (cell: OrgStatusEnvCell) => void }) {
  const slug = cell.environment.slug ?? '?';
  const dataState = cell.never_deployed ? 'never' : cell.health.state;
  const stale = !cell.never_deployed && cell.health.staleness === 'stale';
  return (
    <button
      type="button"
      className="m-env-chip"
      data-state={dataState}
      data-stale={stale || undefined}
      onClick={() => onTap(cell)}
    >
      {slug}
    </button>
  );
}
