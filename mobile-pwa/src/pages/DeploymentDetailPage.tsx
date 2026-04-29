import { Link, useLocation, useParams } from 'react-router-dom';
import type { OrgDeploymentRow } from '../types';
import { StatusPill } from '../components/StatusPill';

interface LocationState {
  row?: OrgDeploymentRow;
}

function fmt(iso: string | null | undefined): string {
  if (!iso) return '—';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '—';
  return d.toLocaleString();
}

export function DeploymentDetailPage() {
  const { orgSlug } = useParams<{ orgSlug: string }>();
  const loc = useLocation();
  const row = (loc.state as LocationState | null)?.row;

  if (!row) {
    return (
      <section>
        <h2>Deployment</h2>
        <p style={{ color: 'var(--color-text-muted, #64748b)' }}>
          Detail not available — open this deployment from the history list.
        </p>
        <Link to={`/orgs/${orgSlug}/history`} className="m-button" style={{ width: '100%' }}>
          Return to history
        </Link>
      </section>
    );
  }

  return (
    <section>
      <header style={{ marginBottom: 12 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <h2 style={{ margin: 0, fontFamily: 'var(--font-mono, monospace)' }}>{row.version}</h2>
          <StatusPill status={row.status} />
        </div>
        {row.commit_sha ? (
          <p style={{ margin: '4px 0 0', color: 'var(--color-text-muted, #64748b)', fontFamily: 'var(--font-mono, monospace)', fontSize: 12 }}>
            {row.commit_sha.slice(0, 7)}
          </p>
        ) : null}
      </header>

      <div className="m-card" style={{ marginBottom: 12 }}>
        <div className="m-list-row">
          <span style={{ color: 'var(--color-text-muted)' }}>Project</span>
          <span>{row.project.name}</span>
        </div>
        <div className="m-list-row">
          <span style={{ color: 'var(--color-text-muted)' }}>Application</span>
          <span>{row.application.name}</span>
        </div>
        <div className="m-list-row">
          <span style={{ color: 'var(--color-text-muted)' }}>Environment</span>
          <span>{row.environment.slug ?? row.environment.name ?? '—'}</span>
        </div>
        <div className="m-list-row">
          <span style={{ color: 'var(--color-text-muted)' }}>Strategy</span>
          <span>{row.strategy}</span>
        </div>
        {row.mode ? (
          <div className="m-list-row">
            <span style={{ color: 'var(--color-text-muted)' }}>Mode</span>
            <span>{row.mode}</span>
          </div>
        ) : null}
        {row.source ? (
          <div className="m-list-row">
            <span style={{ color: 'var(--color-text-muted)' }}>Source</span>
            <span>{row.source}</span>
          </div>
        ) : null}
      </div>

      <div className="m-card">
        <div className="m-list-row">
          <span style={{ color: 'var(--color-text-muted)' }}>Created</span>
          <span>{fmt(row.created_at)}</span>
        </div>
        <div className="m-list-row">
          <span style={{ color: 'var(--color-text-muted)' }}>Started</span>
          <span>{fmt(row.started_at)}</span>
        </div>
        <div className="m-list-row">
          <span style={{ color: 'var(--color-text-muted)' }}>Completed</span>
          <span>{fmt(row.completed_at)}</span>
        </div>
      </div>

      <p style={{ color: 'var(--color-text-muted, #64748b)', fontSize: 12, marginTop: 16 }}>
        Phase timeline + rollback / promote ship in a later phase. Open in the desktop dashboard
        for full controls.
      </p>
    </section>
  );
}
