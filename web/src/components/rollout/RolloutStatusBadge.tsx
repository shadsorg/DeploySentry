import type { RolloutStatus } from '@/types';

const CLASS_BY_STATUS: Record<RolloutStatus, string> = {
  pending: 'badge badge-pending',
  active: 'badge badge-active',
  paused: 'badge badge-warning',
  awaiting_approval: 'badge badge-warning',
  succeeded: 'badge badge-completed',
  rolled_back: 'badge badge-rolling-back',
  aborted: 'badge badge-failed',
  superseded: 'badge badge-disabled',
};

const LABEL_BY_STATUS: Record<RolloutStatus, string> = {
  pending: 'Pending',
  active: 'Active',
  paused: 'Paused',
  awaiting_approval: 'Awaiting Approval',
  succeeded: 'Succeeded',
  rolled_back: 'Rolled Back',
  aborted: 'Aborted',
  superseded: 'Superseded',
};

export function RolloutStatusBadge({ status }: { status: RolloutStatus }) {
  return <span className={CLASS_BY_STATUS[status]}>{LABEL_BY_STATUS[status]}</span>;
}
