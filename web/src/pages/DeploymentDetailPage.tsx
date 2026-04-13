import { useState, useEffect } from 'react';
import { useParams, Link } from 'react-router-dom';
import ActionBar from '@/components/ActionBar';
import ConfirmDialog from '@/components/ConfirmDialog';
import { deploymentsApi } from '@/api';
import type { Deployment, DeployStatus, DeploymentPhase, PhaseStatus } from '@/types';

type ActionKey = 'promote' | 'rollback' | 'pause' | 'cancel' | 'advance' | 'resume';

const confirmConfig: Record<
  ActionKey,
  { title: string; message: string; variant: 'primary' | 'danger' }
> = {
  promote: {
    title: 'Promote Deployment',
    message: 'Route 100% traffic to this deployment?',
    variant: 'primary',
  },
  rollback: {
    title: 'Rollback Deployment',
    message: 'Roll back and route traffic to the previous version? This cannot be undone.',
    variant: 'danger',
  },
  pause: {
    title: 'Pause Deployment',
    message: 'Pause canary progression? You can resume later.',
    variant: 'primary',
  },
  cancel: {
    title: 'Cancel Deployment',
    message: 'Cancel this deployment? This cannot be undone.',
    variant: 'danger',
  },
  advance: {
    title: 'Advance Past Gate',
    message: 'Advance to the next phase?',
    variant: 'primary',
  },
  resume: {
    title: 'Resume Deployment',
    message: 'Resume the paused deployment?',
    variant: 'primary',
  },
};

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
    case 'canary':
      return 'badge badge-experiment';
    case 'blue-green':
      return 'badge badge-release';
    case 'rolling':
      return 'badge badge-ops';
    default:
      return 'badge';
  }
}

function statusBadgeClass(status: string): string {
  switch (status) {
    case 'running':
    case 'promoting':
      return 'badge badge-active';
    case 'completed':
      return 'badge badge-completed';
    case 'failed':
      return 'badge badge-danger';
    case 'rolled_back':
      return 'badge badge-rolling-back';
    case 'paused':
      return 'badge badge-ops';
    case 'pending':
      return 'badge badge-pending';
    case 'cancelled':
      return 'badge badge-disabled';
    default:
      return 'badge';
  }
}

function healthColorClass(score: number): string {
  if (score >= 99) return 'text-success';
  if (score >= 95) return 'text-warning';
  return 'text-danger';
}

function phaseStatusIcon(status: PhaseStatus): string {
  switch (status) {
    case 'passed':
      return '✓';
    case 'active':
      return '▶';
    case 'failed':
      return '✗';
    case 'skipped':
      return '⊘';
    default:
      return '○';
  }
}

export default function DeploymentDetailPage() {
  const { id, orgSlug, projectSlug, appSlug } = useParams();
  const backPath = `/orgs/${orgSlug}/projects/${projectSlug}/apps/${appSlug}/deployments`;

  const [dep, setDep] = useState<Deployment | null>(null);
  const [phases, setPhases] = useState<DeploymentPhase[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [confirmAction, setConfirmAction] = useState<ActionKey | null>(null);
  const [actionLoading, setActionLoading] = useState(false);

  function fetchDeployment() {
    if (!id) return;
    return Promise.all([
      deploymentsApi.get(id),
      deploymentsApi.phases(id).catch(() => ({ phases: [] as DeploymentPhase[] })),
    ]).then(([depData, phasesData]) => {
      setDep(depData);
      setPhases(phasesData.phases ?? []);
    });
  }

  useEffect(() => {
    if (!id) return;
    setLoading(true);
    setError(null);

    fetchDeployment()
      ?.catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, [id]);

  async function executeAction() {
    if (!dep || !confirmAction) return;
    setActionLoading(true);
    try {
      switch (confirmAction) {
        case 'promote':
          await deploymentsApi.promote(dep.id);
          break;
        case 'rollback':
          await deploymentsApi.rollback(dep.id);
          break;
        case 'pause':
          await deploymentsApi.pause(dep.id);
          break;
        case 'resume':
          await deploymentsApi.resume(dep.id);
          break;
        case 'cancel':
          await deploymentsApi.cancel(dep.id);
          break;
        case 'advance':
          await deploymentsApi.advance(dep.id);
          break;
      }
      setConfirmAction(null);
      await fetchDeployment();
    } catch (err: any) {
      setError(err.message);
      setConfirmAction(null);
    } finally {
      setActionLoading(false);
    }
  }

  function getDeployActions(status: DeployStatus) {
    const trigger = (action: ActionKey) => () => setConfirmAction(action);
    switch (status) {
      case 'pending':
        return {
          primaryAction: { label: 'Start', onClick: trigger('advance') },
          secondaryActions: [{ label: 'Cancel', onClick: trigger('cancel') }],
        };
      case 'running':
        return {
          primaryAction: { label: 'Promote', onClick: trigger('promote') },
          secondaryActions: [
            { label: 'Pause', onClick: trigger('pause') },
            { label: 'Advance', onClick: trigger('advance') },
            { label: 'Rollback', onClick: trigger('rollback'), variant: 'danger' as const },
          ],
        };
      case 'promoting':
        return {
          secondaryActions: [{ label: 'Rollback', onClick: trigger('rollback'), variant: 'danger' as const }],
        };
      case 'paused':
        return {
          primaryAction: { label: 'Resume', onClick: trigger('resume') },
          secondaryActions: [
            { label: 'Advance', onClick: trigger('advance') },
            { label: 'Rollback', onClick: trigger('rollback'), variant: 'danger' as const },
            { label: 'Cancel', onClick: trigger('cancel') },
          ],
        };
      case 'failed':
        return {
          primaryAction: { label: 'Rollback', onClick: trigger('rollback'), variant: 'danger' as const },
        };
      default:
        return {};
    }
  }

  // Poll for updates while deployment is active
  useEffect(() => {
    if (!id || !dep) return;

    const terminalStatuses = ['completed', 'failed', 'rolled_back', 'cancelled'];
    if (terminalStatuses.includes(dep.status)) return;

    const interval = setInterval(() => {
      deploymentsApi.get(id).then(setDep).catch(() => {});
    }, 5000);

    return () => clearInterval(interval);
  }, [id, dep?.status]);

  if (loading) return <div>Loading...</div>;
  if (error) return <div>Error: {error}</div>;
  if (!dep) return <div>Deployment not found.</div>;

  const actions = getDeployActions(dep.status);
  const artifactHostname = dep.artifact
    ? (() => {
        try {
          return new URL(dep.artifact).hostname;
        } catch {
          return undefined;
        }
      })()
    : undefined;

  return (
    <div className="page">
      <div className="detail-header">
        <Link to={backPath} className="back-link">
          ← Deployments
        </Link>
        <div className="detail-header-top">
          <div>
            <h1 className="detail-header-title">{dep.version}</h1>
            <div className="detail-header-subtitle">
              {dep.commit_sha && <span>{dep.commit_sha.slice(0, 7)}</span>}
              {artifactHostname && (
                <>
                  {' · '}
                  <a href={dep.artifact} target="_blank" rel="noopener noreferrer">
                    {artifactHostname}
                  </a>
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
          <div className={`info-card-value ${healthColorClass(dep.health_score)}`}>
            {dep.health_score}%
          </div>
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

      {phases.length > 0 && (
        <div className="phases-section">
          <h2 className="phases-title">Deployment Phases</h2>
          <div className="phases-list">
            {phases
              .slice()
              .sort((a, b) => a.sort_order - b.sort_order)
              .map((phase) => (
                <div
                  key={phase.id}
                  className={`phase-item phase-${phase.status}`}
                >
                  <div className="phase-header">
                    <span>{phaseStatusIcon(phase.status)}</span>
                    <span className="phase-name">{phase.name}</span>
                    <span className={statusBadgeClass(phase.status === 'active' ? 'running' : phase.status === 'passed' ? 'completed' : phase.status === 'failed' ? 'failed' : 'pending')}>
                      {phase.status}
                    </span>
                  </div>
                  <div className="phase-details">
                    <span>{phase.traffic_percent}% traffic</span>
                    <span>{phase.duration_seconds}s duration</span>
                    <span>{phase.auto_promote ? 'Auto-promote' : 'Manual gate'}</span>
                  </div>
                  {phase.status === 'active' && (
                    <div className="phase-progress">
                      <div
                        className="phase-progress-fill"
                        style={{
                          width:
                            phase.started_at && phase.duration_seconds > 0
                              ? `${Math.min(
                                  100,
                                  ((Date.now() - new Date(phase.started_at).getTime()) /
                                    (phase.duration_seconds * 1000)) *
                                    100,
                                )}%`
                              : '100%',
                        }}
                      />
                    </div>
                  )}
                </div>
              ))}
          </div>
        </div>
      )}

      <div className="activity-log">
        <h2 className="activity-log-title">Activity Log</h2>
        <p style={{ textAlign: 'center', padding: '2rem 0', color: 'var(--color-text-muted)' }}>
          No events data available
        </p>
      </div>

      {confirmAction && (
        <ConfirmDialog
          open={!!confirmAction}
          title={confirmConfig[confirmAction].title}
          message={confirmConfig[confirmAction].message}
          confirmLabel={confirmConfig[confirmAction].title.split(' ')[0]}
          confirmVariant={confirmConfig[confirmAction].variant}
          onConfirm={executeAction}
          onCancel={() => setConfirmAction(null)}
          loading={actionLoading}
        />
      )}
    </div>
  );
}
