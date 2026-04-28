import type { OrgDeploymentRow } from '../types';
import { StatusPill } from './StatusPill';

function relativeAge(iso: string): string {
  const ms = Date.now() - new Date(iso).getTime();
  if (Number.isNaN(ms) || ms < 0) return '';
  const sec = Math.floor(ms / 1000);
  if (sec < 60) return `${sec}s ago`;
  const min = Math.floor(sec / 60);
  if (min < 60) return `${min}m ago`;
  const hr = Math.floor(min / 60);
  if (hr < 24) return `${hr}h ago`;
  const day = Math.floor(hr / 24);
  return `${day}d ago`;
}

export function DeploymentRow({
  row,
  onTap,
}: {
  row: OrgDeploymentRow;
  onTap: (row: OrgDeploymentRow) => void;
}) {
  return (
    <button type="button" className="m-card m-deploy-row" onClick={() => onTap(row)}>
      <div className="m-deploy-row-head">
        <span className="m-deploy-version">{row.version}</span>
        <StatusPill status={row.status} />
      </div>
      <div className="m-deploy-row-meta">
        <span className="m-app-name">{row.application.name}</span>
        <span className="m-env-chip" data-state="never" style={{ cursor: 'default' }}>
          {row.environment.slug ?? '?'}
        </span>
        <span className="m-deploy-row-age">{relativeAge(row.created_at)}</span>
      </div>
    </button>
  );
}
