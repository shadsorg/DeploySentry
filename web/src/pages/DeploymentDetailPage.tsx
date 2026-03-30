import { useParams, Link } from 'react-router-dom';
import ActionBar from '@/components/ActionBar';
import { MOCK_DEPLOYMENT_DETAIL, MOCK_DEPLOYMENT_EVENTS } from '@/mocks/hierarchy';
import type { DeployStatus } from '@/types';

function getDeployActions(status: DeployStatus) {
  const noop = () => {};
  switch (status) {
    case 'pending':
      return { primaryAction: { label: 'Start', onClick: noop }, secondaryActions: [{ label: 'Cancel', onClick: noop }] };
    case 'running':
      return { primaryAction: { label: 'Promote', onClick: noop }, secondaryActions: [{ label: 'Pause', onClick: noop }, { label: 'Rollback', onClick: noop, variant: 'danger' as const }] };
    case 'promoting':
      return { secondaryActions: [{ label: 'Rollback', onClick: noop, variant: 'danger' as const }] };
    case 'paused':
      return { primaryAction: { label: 'Resume', onClick: noop }, secondaryActions: [{ label: 'Rollback', onClick: noop, variant: 'danger' as const }, { label: 'Cancel', onClick: noop }] };
    case 'failed':
      return { primaryAction: { label: 'Rollback', onClick: noop, variant: 'danger' as const } };
    default:
      return {};
  }
}

function formatDateTime(iso: string): string {
  return new Date(iso).toLocaleString('en-US', {
    month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit',
  });
}

function computeDuration(start?: string, end?: string | null): string {
  if (!start) return '—';
  const s = new Date(start).getTime();
  const e = end ? new Date(end).getTime() : Date.now();
  const mins = Math.floor((e - s) / 60000);
  if (mins < 60) return `${mins}m`;
  return `${Math.floor(mins / 60)}h ${mins % 60}m`;
}

function strategyBadgeClass(strategy: string): string {
  switch (strategy) {
    case 'canary': return 'badge badge-experiment';
    case 'blue-green': return 'badge badge-release';
    case 'rolling': return 'badge badge-ops';
    default: return 'badge';
  }
}

function statusBadgeClass(status: string): string {
  switch (status) {
    case 'running': case 'promoting': return 'badge badge-active';
    case 'completed': return 'badge badge-completed';
    case 'failed': return 'badge badge-danger';
    case 'rolled_back': return 'badge badge-rolling-back';
    case 'paused': return 'badge badge-ops';
    case 'pending': return 'badge badge-pending';
    case 'cancelled': return 'badge badge-disabled';
    default: return 'badge';
  }
}

function statusDotColor(status: DeployStatus): string {
  switch (status) {
    case 'running': case 'completed': return 'activity-dot-green';
    case 'paused': case 'promoting': case 'pending': return 'activity-dot-yellow';
    case 'failed': case 'rolled_back': case 'cancelled': return 'activity-dot-red';
    default: return 'activity-dot-gray';
  }
}

function healthColorClass(score: number): string {
  if (score >= 99) return 'text-success';
  if (score >= 95) return 'text-warning';
  return 'text-danger';
}

export default function DeploymentDetailPage() {
  const { orgSlug, projectSlug, appSlug } = useParams();
  const backPath = `/orgs/${orgSlug}/projects/${projectSlug}/apps/${appSlug}/deployments`;

  const dep = MOCK_DEPLOYMENT_DETAIL;
  const events = MOCK_DEPLOYMENT_EVENTS;
  const actions = getDeployActions(dep.status);

  const artifactHostname = dep.artifact ? (() => { try { return new URL(dep.artifact).hostname; } catch { return undefined; } })() : undefined;

  return (
    <div className="page">
      <div className="detail-header">
        <Link to={backPath} className="back-link">← Deployments</Link>
        <div className="detail-header-top">
          <div>
            <h1 className="detail-header-title">{dep.version}</h1>
            <div className="detail-header-subtitle">
              {dep.commit_sha && <span>{dep.commit_sha.slice(0, 7)}</span>}
              {artifactHostname && (
                <>
                  {' · '}
                  <a href={dep.artifact} target="_blank" rel="noopener noreferrer">{artifactHostname}</a>
                </>
              )}
              {' · '}
              <span className={strategyBadgeClass(dep.strategy)}>{dep.strategy}</span>
              {' · '}
              <span>{dep.traffic_percent}% traffic</span>
            </div>
            <div className="detail-header-badges">
              <span className={statusBadgeClass(dep.status)}>{dep.status.replace('_', ' ')}</span>
            </div>
          </div>
          <ActionBar {...actions} />
        </div>
      </div>

      <div className="info-cards">
        <div className="info-card">
          <div className="info-card-label">Traffic</div>
          <div className="info-card-value">{dep.traffic_percent}%</div>
          <div className="info-card-bar">
            <div className="info-card-bar-fill" style={{ width: `${dep.traffic_percent}%` }} />
          </div>
        </div>
        <div className="info-card">
          <div className="info-card-label">Health</div>
          <div className={`info-card-value ${healthColorClass(dep.health_score)}`}>{dep.health_score}%</div>
        </div>
        <div className="info-card">
          <div className="info-card-label">Duration</div>
          <div className="info-card-value">{computeDuration(dep.started_at, dep.completed_at)}</div>
        </div>
        <div className="info-card">
          <div className="info-card-label">Created by</div>
          <div className="info-card-value">{dep.created_by}</div>
        </div>
      </div>

      <div className="activity-log">
        <h2 className="activity-log-title">Activity Log</h2>
        {events.map((event, i) => (
          <div key={i} className="activity-log-entry">
            <span className={`activity-dot ${statusDotColor(event.status)}`} />
            <span className="activity-status">{event.status.replace('_', ' ')}</span>
            <span className="activity-time">{formatDateTime(event.timestamp)}</span>
            <span className="activity-note">{event.note}</span>
          </div>
        ))}
      </div>
    </div>
  );
}
