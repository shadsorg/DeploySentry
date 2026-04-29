import type { DeployStatus } from '../types';

export function StatusPill({ status }: { status: DeployStatus }) {
  return (
    <span className="m-status-pill" data-state={status}>
      {status.toUpperCase()}
    </span>
  );
}
