import React, { useState, useMemo, useEffect } from 'react';
import { useParams } from 'react-router-dom';
import type { Release, ReleaseStatus } from '@/types';
import { entitiesApi, releasesApi } from '@/api';

const TABS = ['all', 'draft', 'rolling_out', 'paused', 'completed', 'rolled_back'] as const;

type TabKey = (typeof TABS)[number];

function tabLabel(tab: TabKey): string {
  switch (tab) {
    case 'all':
      return 'All';
    case 'draft':
      return 'Draft';
    case 'rolling_out':
      return 'Rolling Out';
    case 'paused':
      return 'Paused';
    case 'completed':
      return 'Completed';
    case 'rolled_back':
      return 'Rolled Back';
  }
}

function statusBadgeClass(status: ReleaseStatus): string {
  switch (status) {
    case 'draft':
      return 'badge badge-pending';
    case 'rolling_out':
      return 'badge badge-active';
    case 'paused':
      return 'badge badge-ops';
    case 'completed':
      return 'badge badge-completed';
    case 'rolled_back':
      return 'badge badge-danger';
  }
}

function statusLabel(status: ReleaseStatus): string {
  switch (status) {
    case 'rolling_out':
      return 'Rolling Out';
    case 'rolled_back':
      return 'Rolled Back';
    default:
      return status.charAt(0).toUpperCase() + status.slice(1);
  }
}

function formatDate(iso: string): string {
  const d = new Date(iso);
  return d.toLocaleString('en-US', {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

const ReleasesPage: React.FC = () => {
  const { orgSlug, projectSlug, appSlug } = useParams();
  const appName = appSlug ?? '';

  const [releases, setReleases] = useState<Release[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<TabKey>('all');

  useEffect(() => {
    if (!orgSlug || !projectSlug || !appSlug) {
      setLoading(false);
      return;
    }
    setLoading(true);
    setError(null);

    entitiesApi
      .getApp(orgSlug, projectSlug, appSlug)
      .then((app) => releasesApi.list(app.id))
      .then((result) => setReleases(result.releases ?? []))
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, [orgSlug, projectSlug, appSlug]);

  const filtered = useMemo(() => {
    if (activeTab === 'all') return releases;
    return releases.filter((r) => r.status === activeTab);
  }, [releases, activeTab]);

  if (!appSlug) {
    return (
      <div>
        <h1 className="page-header">Releases</h1>
        <p>Select an application to view releases</p>
      </div>
    );
  }

  if (loading) return (
    <div className="empty-state" style={{ padding: '40px 0' }}>
      <span className="ms" style={{ fontSize: 32, color: 'var(--color-primary)', marginBottom: 12, display: 'block' }}>sync</span>
      Loading releases…
    </div>
  );
  if (error) return <div className="page-error">Error: {error}</div>;

  return (
    <div>
      <div className="page-header-row">
        <div className="page-header" style={{ marginBottom: 0 }}>
          <h1>{appName ? `${appName} — Releases` : 'Releases'}</h1>
          <p>Coordinate flag changes across environments with managed releases</p>
        </div>
        <button className="btn btn-primary">
          <span className="ms" style={{ fontSize: 16 }}>add</span>
          Create Release
        </button>
      </div>

      <div className="tabs">
        {TABS.map((tab) => (
          <button
            key={tab}
            className={`tab${activeTab === tab ? ' active' : ''}`}
            onClick={() => setActiveTab(tab)}
          >
            {tabLabel(tab)}
          </button>
        ))}
      </div>

      <div className="card" style={{ padding: 0, overflow: 'hidden' }}>
        <div style={{ padding: '12px 20px', borderBottom: '1px solid var(--color-border)', display: 'flex', alignItems: 'center', gap: 8 }}>
          <span className="ms" style={{ fontSize: 18, color: 'var(--color-primary)' }}>local_shipping</span>
          <span style={{ fontFamily: 'var(--font-display)', fontWeight: 700, fontSize: 14 }}>Releases</span>
          <span className="badge" style={{ background: 'var(--color-primary-bg)', color: 'var(--color-primary)', marginLeft: 4 }}>{filtered.length}</span>
        </div>
        <div className="table-container">
          <table>
            <thead>
              <tr>
                <th>Name</th>
                <th>Status</th>
                <th>Traffic %</th>
                <th>Description</th>
                <th>Created By</th>
                <th>Created</th>
              </tr>
            </thead>
            <tbody>
              {filtered.length === 0 ? (
                <tr>
                  <td colSpan={6}>
                    <div className="empty-state" style={{ padding: '40px 0' }}>
                      <span className="ms" style={{ fontSize: 36, display: 'block', marginBottom: 12, color: 'var(--color-text-muted)' }}>local_shipping</span>
                      <h3>No releases found</h3>
                      <p>There are no releases matching the selected filter.</p>
                    </div>
                  </td>
                </tr>
              ) : (
                filtered.map((rel) => (
                  <tr key={rel.id}>
                    <td style={{ fontWeight: 500 }}>{rel.name}</td>
                    <td>
                      <span className={statusBadgeClass(rel.status)}>
                        {statusLabel(rel.status)}
                      </span>
                    </td>
                    <td>
                      <span className="text-sm">{rel.traffic_percent}%</span>
                    </td>
                    <td className="text-sm" style={{ maxWidth: 300 }}>
                      <span className="truncate" style={{ display: 'block' }}>
                        {rel.description}
                      </span>
                    </td>
                    <td className="text-sm text-secondary">{rel.created_by}</td>
                    <td className="text-sm text-secondary" style={{ whiteSpace: 'nowrap' }}>
                      {formatDate(rel.created_at)}
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

export default ReleasesPage;
