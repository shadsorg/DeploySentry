import { useCallback, useEffect, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { orgDeploymentsApi, projectsApi } from '../api';
import type { OrgDeploymentRow, Project } from '../types';
import { DeploymentRow } from '../components/DeploymentRow';
import { StatusFilterChips } from '../components/StatusFilterChips';
import { ProjectFilterSheet } from '../components/ProjectFilterSheet';
import { StaleBadge } from '../components/StaleBadge';

const PAGE_SIZE = 25;

export function HistoryPage() {
  const { orgSlug } = useParams<{ orgSlug: string }>();
  const nav = useNavigate();

  const [status, setStatus] = useState('');
  const [projectId, setProjectId] = useState('');
  const [projects, setProjects] = useState<Project[]>([]);
  const [projectSheetOpen, setProjectSheetOpen] = useState(false);
  const [rows, setRows] = useState<OrgDeploymentRow[]>([]);
  const [cursor, setCursor] = useState<string | undefined>(undefined);
  const [loading, setLoading] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [lastSuccess, setLastSuccess] = useState<number | null>(null);
  const [refreshing, setRefreshing] = useState(false);

  // Load project list once.
  useEffect(() => {
    if (!orgSlug) return;
    projectsApi
      .list(orgSlug)
      .then((r) => setProjects(r.projects))
      .catch(() => {
        // non-fatal: filter just doesn't show projects
      });
  }, [orgSlug]);

  const fetchPage = useCallback(
    async (reset: boolean) => {
      if (!orgSlug) return;
      const isInitial = reset;
      if (isInitial) {
        setLoading(true);
        setRefreshing(true);
      } else {
        setLoadingMore(true);
      }
      try {
        const res = await orgDeploymentsApi.list(orgSlug, {
          status: status || undefined,
          project_id: projectId || undefined,
          limit: PAGE_SIZE,
          cursor: isInitial ? undefined : cursor,
        });
        setRows((prev) => (isInitial ? res.deployments : [...prev, ...res.deployments]));
        setCursor(res.next_cursor);
        setError(null);
        if (isInitial) setLastSuccess(Date.now());
      } catch (e) {
        setError(e instanceof Error ? e.message : 'Failed to load deployments');
      } finally {
        setLoading(false);
        setLoadingMore(false);
        if (isInitial) setRefreshing(false);
      }
    },
    [orgSlug, status, projectId, cursor],
  );

  // Refetch from scratch whenever the filters change.
  useEffect(() => {
    void fetchPage(true);
    // intentionally exclude `cursor` so a Load more click doesn't loop
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [orgSlug, status, projectId]);

  const projectName =
    projectId === '' ? 'All projects' : projects.find((p) => p.id === projectId)?.name ?? 'Project';

  return (
    <section>
      <h2 style={{ margin: '4px 0 8px' }}>Deploy History</h2>
      <StaleBadge lastSuccess={lastSuccess} inflight={refreshing} />

      <StatusFilterChips value={status} onChange={setStatus} />

      <button
        type="button"
        className="m-button"
        style={{ width: '100%', justifyContent: 'space-between', marginBottom: 12 }}
        onClick={() => setProjectSheetOpen(true)}
      >
        <span style={{ color: 'var(--color-text-muted, #64748b)' }}>Project</span>
        <span>{projectName}</span>
      </button>

      <ProjectFilterSheet
        open={projectSheetOpen}
        projects={projects}
        value={projectId}
        onSelect={setProjectId}
        onClose={() => setProjectSheetOpen(false)}
      />

      {error && (
        <p style={{ color: 'var(--color-danger, #ef4444)', fontSize: 13 }}>{error}</p>
      )}

      {loading && rows.length === 0 ? (
        <div className="m-page-loading" style={{ height: 'auto', padding: 24 }}>Loading…</div>
      ) : rows.length === 0 ? (
        <p style={{ color: 'var(--color-text-muted, #64748b)' }}>No deployments match these filters.</p>
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
          {rows.map((r) => (
            <DeploymentRow
              key={r.id}
              row={r}
              onTap={(row) =>
                nav(`/orgs/${orgSlug}/history/${row.id}`, { state: { row } })
              }
            />
          ))}
          {cursor ? (
            <button
              type="button"
              className="m-button"
              style={{ width: '100%' }}
              disabled={loadingMore}
              onClick={() => void fetchPage(false)}
            >
              {loadingMore ? 'Loading…' : 'Load older'}
            </button>
          ) : null}
        </div>
      )}
    </section>
  );
}
