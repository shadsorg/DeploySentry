import React, { useState, useMemo, useEffect, useCallback } from 'react';
import { useParams } from 'react-router-dom';
import type { Deployment, DeployStrategy, DeployStatus, OrgEnvironment } from '@/types';
import { entitiesApi, deploymentsApi } from '@/api';
import { StrategyPicker } from '@/components/rollout/StrategyPicker';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function strategyBadgeClass(strategy: DeployStrategy): string {
  switch (strategy) {
    case 'canary':
      return 'badge badge-experiment';
    case 'blue-green':
      return 'badge badge-release';
    case 'rolling':
      return 'badge badge-ops';
  }
}

function statusBadgeClass(status: DeployStatus): string {
  switch (status) {
    case 'running':
      return 'badge badge-active';
    case 'promoting':
      return 'badge badge-active';
    case 'completed':
      return 'badge badge-completed';
    case 'failed':
      return 'badge badge-failed';
    case 'rolled_back':
      return 'badge badge-rolling-back';
    case 'paused':
      return 'badge badge-pending';
    case 'pending':
      return 'badge badge-pending';
    case 'cancelled':
      return 'badge badge-disabled';
    default:
      return 'badge';
  }
}

function statusLabel(status: DeployStatus): string {
  switch (status) {
    case 'rolled_back':
      return 'Rolled Back';
    case 'promoting':
      return 'Promoting';
    case 'cancelled':
      return 'Cancelled';
    default:
      return status.charAt(0).toUpperCase() + status.slice(1);
  }
}

function healthColor(score: number): string {
  if (score >= 95) return 'text-success';
  if (score >= 80) return 'text-warning';
  return 'text-danger';
}

function trafficBarColor(score: number): string {
  if (score >= 95) return 'var(--color-success)';
  if (score >= 80) return 'var(--color-warning)';
  return 'var(--color-danger)';
}

function formatTime(iso: string): string {
  const d = new Date(iso);
  return d.toLocaleString('en-US', {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

function computeDuration(start: string, end: string | null): string {
  const s = new Date(start).getTime();
  const e = end ? new Date(end).getTime() : Date.now();
  const mins = Math.round((e - s) / 60000);
  if (mins < 60) return `${mins}m`;
  const hours = Math.floor(mins / 60);
  const rem = mins % 60;
  return `${hours}h ${rem}m`;
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

const DeploymentsPage: React.FC = () => {
  const { orgSlug, projectSlug, appSlug } = useParams();
  const appName = appSlug ?? '';

  const [deployments, setDeployments] = useState<Deployment[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [search, setSearch] = useState('');
  const [strategyFilter, setStrategyFilter] = useState<string>('all');
  const [statusFilter, setStatusFilter] = useState<string>('all');

  // Create modal state
  const [creating, setCreating] = useState(false);
  const [artifact, setArtifact] = useState('');
  const [version, setVersion] = useState('');
  const [strategyName, setStrategyName] = useState('');
  const [applyImmediately, setApplyImmediately] = useState(false);
  const [envId, setEnvId] = useState('');
  const [environments, setEnvironments] = useState<OrgEnvironment[]>([]);
  const [submitting, setSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);

  const load = useCallback(() => {
    if (!orgSlug || !projectSlug || !appSlug) {
      setLoading(false);
      return;
    }
    setLoading(true);
    setError(null);

    entitiesApi
      .getApp(orgSlug, projectSlug, appSlug)
      .then((app) => deploymentsApi.list(app.id))
      .then((result) => setDeployments(result.deployments ?? []))
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, [orgSlug, projectSlug, appSlug]);

  useEffect(() => { load(); }, [load]);

  // Load org environments for the create form
  useEffect(() => {
    if (!orgSlug) return;
    entitiesApi.listOrgEnvironments(orgSlug).then((r) => {
      setEnvironments(r.environments ?? []);
      if (r.environments?.length) setEnvId(r.environments[0].id);
    }).catch(() => {});
  }, [orgSlug]);

  const openCreateModal = () => {
    setArtifact('');
    setVersion('');
    setStrategyName('');
    setApplyImmediately(false);
    setSubmitError(null);
    if (environments.length) setEnvId(environments[0].id);
    setCreating(true);
  };

  const handleCreate = async () => {
    if (!orgSlug || !projectSlug || !appSlug) return;
    if (!artifact.trim() || !version.trim()) {
      setSubmitError('Artifact and version are required.');
      return;
    }
    if (!envId) {
      setSubmitError('Select an environment.');
      return;
    }
    setSubmitting(true);
    setSubmitError(null);
    try {
      const app = await entitiesApi.getApp(orgSlug, projectSlug, appSlug);
      await deploymentsApi.create({
        application_id: app.id,
        environment_id: envId,
        artifact: artifact.trim(),
        version: version.trim(),
        strategy: applyImmediately ? 'rolling' : (strategyName || 'rolling'),
        rollout: applyImmediately
          ? { apply_immediately: true }
          : strategyName
          ? { strategy_name: strategyName }
          : undefined,
      });
      setCreating(false);
      load();
    } catch (err: any) {
      setSubmitError(err.message ?? 'Failed to create deployment.');
    } finally {
      setSubmitting(false);
    }
  };

  const filtered = useMemo(() => {
    // Performance Optimization: Hoist search.toLowerCase() outside the loop
    // to avoid O(N) penalties per render. Also use optional chaining.
    const searchLower = search?.toLowerCase() ?? '';

    return deployments.filter((d) => {
      if (searchLower && !d.version?.toLowerCase().includes(searchLower)) {
        return false;
      }
      if (strategyFilter !== 'all' && d.strategy !== strategyFilter) {
        return false;
      }
      if (statusFilter !== 'all' && d.status !== statusFilter) {
        return false;
      }
      return true;
    });
  }, [deployments, search, strategyFilter, statusFilter]);

  if (!appSlug) {
    return (
      <div>
        <h1 className="page-header">Deployments</h1>
        <p>Select an application to view deployments</p>
      </div>
    );
  }

  if (loading) return <div>Loading...</div>;
  if (error) return <div>Error: {error}</div>;

  return (
    <div>
      {/* Page header */}
      <div className="page-header-row">
        <div className="page-header" style={{ marginBottom: 0 }}>
          <h1>{appName ? `${appName} — Deployments` : 'Deployments'}</h1>
          <p>Monitor and manage application deployments across environments</p>
        </div>
        <button className="btn btn-primary" onClick={openCreateModal}>+ New Deployment</button>
      </div>

      {creating && (
        <div className="modal-backdrop" onClick={() => setCreating(false)}>
          <div className="modal" onClick={(e) => e.stopPropagation()}>
            <h3>New Deployment</h3>
            <div className="form-group">
              <label className="form-label">Artifact URL / Version Ref</label>
              <input
                className="form-input"
                type="text"
                placeholder="e.g. gcr.io/my-project/app:v1.2.3"
                value={artifact}
                onChange={(e) => setArtifact(e.target.value)}
              />
            </div>
            <div className="form-group">
              <label className="form-label">Version</label>
              <input
                className="form-input"
                type="text"
                placeholder="e.g. v1.2.3"
                value={version}
                onChange={(e) => setVersion(e.target.value)}
              />
            </div>
            <div className="form-group">
              <label className="form-label">Environment</label>
              <select
                className="form-select"
                value={envId}
                onChange={(e) => setEnvId(e.target.value)}
              >
                {environments.length === 0 && <option value="">No environments found</option>}
                {environments.map((env) => (
                  <option key={env.id} value={env.id}>{env.name}</option>
                ))}
              </select>
            </div>
            <div className="form-group">
              <label className="form-label">Rollout Strategy</label>
              {orgSlug && (
                <StrategyPicker
                  orgSlug={orgSlug}
                  targetType="deploy"
                  value={strategyName}
                  onChange={setStrategyName}
                  allowImmediate
                  immediate={applyImmediately}
                  onImmediateChange={setApplyImmediately}
                />
              )}
            </div>
            {submitError && <p className="text-danger text-sm">{submitError}</p>}
            <div className="modal-actions">
              <button className="btn" onClick={() => setCreating(false)} disabled={submitting}>
                Cancel
              </button>
              <button className="btn btn-primary" onClick={handleCreate} disabled={submitting}>
                {submitting ? 'Creating…' : 'Create Deployment'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Filter bar */}
      <div className="filter-bar">
        <input
          className="form-input"
          type="text"
          placeholder="Search by version..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
        <select
          className="form-select"
          value={strategyFilter}
          onChange={(e) => setStrategyFilter(e.target.value)}
        >
          <option value="all">All Strategies</option>
          <option value="canary">Canary</option>
          <option value="blue-green">Blue/Green</option>
          <option value="rolling">Rolling</option>
        </select>
        <select
          className="form-select"
          value={statusFilter}
          onChange={(e) => setStatusFilter(e.target.value)}
        >
          <option value="all">All Statuses</option>
          <option value="pending">Pending</option>
          <option value="running">Running</option>
          <option value="promoting">Promoting</option>
          <option value="paused">Paused</option>
          <option value="completed">Completed</option>
          <option value="failed">Failed</option>
          <option value="rolled_back">Rolled Back</option>
          <option value="cancelled">Cancelled</option>
        </select>
      </div>

      {/* Table */}
      <div className="card">
        <div className="table-container">
          <table>
            <thead>
              <tr>
                <th>Version</th>
                <th>Strategy</th>
                <th>Status</th>
                <th>Traffic %</th>
                <th>Health</th>
                <th>Started</th>
                <th>Duration</th>
              </tr>
            </thead>
            <tbody>
              {filtered.length === 0 ? (
                <tr>
                  <td colSpan={7}>
                    <div className="empty-state">
                      <h3>No deployments found</h3>
                      <p>Try adjusting your filters or search term.</p>
                    </div>
                  </td>
                </tr>
              ) : (
                filtered.map((dep) => (
                  <tr key={dep.id}>
                    <td style={{ fontWeight: 500 }}>
                      <code className="font-mono text-sm">{dep.version}</code>
                    </td>
                    <td>
                      <span className={strategyBadgeClass(dep.strategy)}>{dep.strategy}</span>
                    </td>
                    <td>
                      <span className={statusBadgeClass(dep.status)}>
                        {statusLabel(dep.status)}
                      </span>
                    </td>
                    <td>
                      <div className="flex items-center gap-2">
                        <div
                          style={{
                            width: 60,
                            height: 6,
                            borderRadius: 3,
                            background: 'var(--color-bg)',
                            overflow: 'hidden',
                          }}
                        >
                          <div
                            style={{
                              width: `${dep.traffic_percent}%`,
                              height: '100%',
                              borderRadius: 3,
                              background: trafficBarColor(dep.health_score),
                              transition: 'width 0.3s ease',
                            }}
                          />
                        </div>
                        <span className="text-sm">{dep.traffic_percent}%</span>
                      </div>
                    </td>
                    <td>
                      <span className={healthColor(dep.health_score)}>
                        {dep.health_score.toFixed(1)}%
                      </span>
                    </td>
                    <td className="text-sm text-secondary" style={{ whiteSpace: 'nowrap' }}>
                      {formatTime(dep.created_at)}
                    </td>
                    <td className="text-sm text-muted" style={{ whiteSpace: 'nowrap' }}>
                      {computeDuration(dep.created_at, dep.completed_at)}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
};

export default DeploymentsPage;
