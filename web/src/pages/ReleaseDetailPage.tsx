import { useState, useEffect } from 'react';
import { useParams, Link } from 'react-router-dom';
import ActionBar from '@/components/ActionBar';
import { releasesApi } from '@/api';
import type { Release, ReleaseStatus } from '@/types';

function getReleaseActions(status: ReleaseStatus) {
  const noop = () => {};
  switch (status) {
    case 'draft':
      return {
        primaryAction: { label: 'Start Rollout', onClick: noop },
        secondaryActions: [{ label: 'Delete', onClick: noop, variant: 'danger' as const }],
      };
    case 'rolling_out':
      return {
        primaryAction: { label: 'Promote', onClick: noop },
        secondaryActions: [
          { label: 'Pause', onClick: noop },
          { label: 'Rollback', onClick: noop, variant: 'danger' as const },
        ],
      };
    case 'paused':
      return {
        primaryAction: { label: 'Resume', onClick: noop },
        secondaryActions: [{ label: 'Rollback', onClick: noop, variant: 'danger' as const }],
      };
    default:
      return {};
  }
}

function statusBadgeClass(status: string): string {
  switch (status) {
    case 'rolling_out':
      return 'badge badge-active';
    case 'completed':
      return 'badge badge-completed';
    case 'rolled_back':
      return 'badge badge-danger';
    case 'paused':
      return 'badge badge-ops';
    case 'draft':
      return 'badge badge-pending';
    default:
      return 'badge';
  }
}

export default function ReleaseDetailPage() {
  const { id, orgSlug, projectSlug, appSlug } = useParams();
  const backPath = `/orgs/${orgSlug}/projects/${projectSlug}/apps/${appSlug}/releases`;

  const [release, setRelease] = useState<Release | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!id) return;
    setLoading(true);
    setError(null);

    releasesApi
      .get(id)
      .then((data) => setRelease(data))
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, [id]);

  if (loading) return (
    <div className="empty-state" style={{ padding: '40px 0' }}>
      <span className="ms" style={{ fontSize: 32, color: 'var(--color-primary)', marginBottom: 12, display: 'block' }}>sync</span>
      Loading release…
    </div>
  );
  if (error) return <div className="page-error">Error: {error}</div>;
  if (!release) return <div className="page-error">Release not found.</div>;

  const actions = getReleaseActions(release.status);

  return (
    <div className="page">
      <div className="detail-header">
        <Link to={backPath} className="back-link">
          &larr; Releases
        </Link>
        <div className="detail-header-top">
          <div>
            <h1 className="detail-header-title">{release.name}</h1>
            {release.description && <p className="detail-description">{release.description}</p>}
            <div className="detail-header-badges">
              <span className={statusBadgeClass(release.status)}>
                {release.status.replace('_', ' ')}
              </span>
              <span>{release.traffic_percent}% traffic</span>
              {release.session_sticky && (
                <span className="sticky-badge">
                  &#128274; Session sticky: {release.sticky_header}
                </span>
              )}
            </div>
          </div>
          <ActionBar {...actions} />
        </div>
      </div>

      <div className="table-section">
        <h2>Flag Changes</h2>
        <p style={{ textAlign: 'center', padding: '2rem 0', color: 'var(--color-text-muted)' }}>
          No flag changes data available
        </p>
      </div>
    </div>
  );
}
