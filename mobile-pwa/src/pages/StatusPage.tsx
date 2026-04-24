import { useCallback, useState } from 'react';
import { useParams } from 'react-router-dom';
import { orgStatusApi } from '../api';
import type { OrgStatusEnvCell, OrgStatusResponse } from '../types';
import { useAutoPoll } from '../hooks/useAutoPoll';
import { ProjectCard } from '../components/ProjectCard';

const POLL_MS = 15_000;

export function StatusPage() {
  const { orgSlug } = useParams<{ orgSlug: string }>();
  const [data, setData] = useState<OrgStatusResponse | null>(null);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(() => {
    if (!orgSlug) return;
    orgStatusApi
      .get(orgSlug)
      .then((r) => {
        setData(r);
        setError(null);
      })
      .catch((e) => setError(e instanceof Error ? e.message : 'Failed to load status'));
  }, [orgSlug]);

  useAutoPoll(load, POLL_MS);

  const onEnvTap = (cell: OrgStatusEnvCell) => {
    const dep = cell.current_deployment;
    window.alert(
      dep
        ? `Deployment ${dep.version} (${dep.status})\nDetail screen ships in Phase 3.`
        : 'No deployment yet.',
    );
  };

  if (error && !data) {
    return (
      <section>
        <h2>Status</h2>
        <p style={{ color: 'var(--color-danger, #ef4444)' }}>{error}</p>
      </section>
    );
  }
  if (!data) {
    return <div className="m-page-loading">Loading…</div>;
  }
  if (data.projects.length === 0) {
    return (
      <section>
        <h2>Status</h2>
        <p style={{ color: 'var(--color-text-muted, #64748b)' }}>No projects in this organization yet.</p>
      </section>
    );
  }

  return (
    <section>
      <h2 style={{ margin: '4px 0 12px' }}>Status</h2>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
        {data.projects.map((p) => (
          <ProjectCard key={p.project.id} project={p} onEnvTap={onEnvTap} />
        ))}
      </div>
    </section>
  );
}
