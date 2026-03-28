import { useParams, Link } from 'react-router-dom';
import ActionBar from '@/components/ActionBar';
import { MOCK_RELEASE_DETAIL, MOCK_RELEASE_FLAG_CHANGES } from '@/mocks/hierarchy';
import type { ReleaseStatus, ReleaseFlagChange } from '@/types';

function getReleaseActions(status: ReleaseStatus) {
  const noop = () => {};
  switch (status) {
    case 'draft':
      return { primaryAction: { label: 'Start Rollout', onClick: noop }, secondaryActions: [{ label: 'Delete', onClick: noop, variant: 'danger' as const }] };
    case 'rolling_out':
      return { primaryAction: { label: 'Promote', onClick: noop }, secondaryActions: [{ label: 'Pause', onClick: noop }, { label: 'Rollback', onClick: noop, variant: 'danger' as const }] };
    case 'paused':
      return { primaryAction: { label: 'Resume', onClick: noop }, secondaryActions: [{ label: 'Rollback', onClick: noop, variant: 'danger' as const }] };
    default:
      return {};
  }
}

function statusBadgeClass(status: string): string {
  switch (status) {
    case 'rolling_out': return 'badge badge-active';
    case 'completed': return 'badge badge-completed';
    case 'rolled_back': return 'badge badge-danger';
    case 'paused': return 'badge badge-ops';
    case 'draft': return 'badge badge-pending';
    default: return 'badge';
  }
}

function formatDateTime(iso: string): string {
  return new Date(iso).toLocaleString('en-US', {
    month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit',
  });
}

function renderEnabledDiff(prev: boolean, next: boolean): JSX.Element | null {
  if (prev === next) return null;
  return (
    <span>
      <span className={prev ? 'diff-disabled' : 'diff-enabled'}>{prev ? 'enabled' : 'disabled'}</span>
      <span className="diff-arrow">{'\u2192'}</span>
      <span className={next ? 'diff-enabled' : 'diff-disabled'}>{next ? 'enabled' : 'disabled'}</span>
    </span>
  );
}

function renderValueDiff(prev: string, next: string): JSX.Element | null {
  if (prev === next) return null;
  return (
    <span>
      <span className="diff-old">{prev}</span>
      <span className="diff-arrow">{'\u2192'}</span>
      <span className="diff-new">{next}</span>
    </span>
  );
}

function renderDiffs(change: ReleaseFlagChange): JSX.Element {
  const enabledDiff = renderEnabledDiff(change.previous_enabled, change.new_enabled);
  const valueDiff = renderValueDiff(change.previous_value, change.new_value);

  if (enabledDiff && valueDiff) {
    return <>{enabledDiff}<br />{valueDiff}</>;
  }
  if (enabledDiff) return enabledDiff;
  if (valueDiff) return valueDiff;
  return <span>—</span>;
}

export default function ReleaseDetailPage() {
  const { orgSlug, projectSlug, appSlug } = useParams();
  const backPath = `/orgs/${orgSlug}/projects/${projectSlug}/apps/${appSlug}/releases`;

  const release = MOCK_RELEASE_DETAIL;
  const flagChanges = MOCK_RELEASE_FLAG_CHANGES;
  const actions = getReleaseActions(release.status);

  return (
    <div className="page">
      <div className="detail-header">
        <Link to={backPath} className="back-link">&larr; Releases</Link>
        <div className="detail-header-top">
          <div>
            <h1 className="detail-header-title">{release.name}</h1>
            {release.description && (
              <p className="detail-description">{release.description}</p>
            )}
            <div className="detail-header-badges">
              <span className={statusBadgeClass(release.status)}>{release.status.replace('_', ' ')}</span>
              <span>{release.traffic_percent}% traffic</span>
              {release.session_sticky && (
                <span className="sticky-badge">&#128274; Session sticky: {release.sticky_header}</span>
              )}
            </div>
          </div>
          <ActionBar {...actions} />
        </div>
      </div>

      <div className="table-section">
        <h2>Flag Changes ({flagChanges.length})</h2>
        {flagChanges.length === 0 ? (
          <p className="empty-state">No flag changes added to this release.</p>
        ) : (
          <table className="data-table">
            <thead>
              <tr>
                <th>Flag Key</th>
                <th>Environment</th>
                <th colSpan={2}>Previous → New</th>
                <th>Applied At</th>
              </tr>
            </thead>
            <tbody>
              {flagChanges.map((change) => (
                  <tr key={change.id}>
                    <td><code>{change.flag_key}</code></td>
                    <td>{change.environment_name}</td>
                    <td colSpan={2}>{renderDiffs(change)}</td>
                    <td>{change.applied_at ? formatDateTime(change.applied_at) : '—'}</td>
                  </tr>
                ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}
