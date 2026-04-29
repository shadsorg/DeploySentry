import { useEffect, useState } from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { projectsApi } from '../api';
import type { Project } from '../types';

export function FlagProjectPickerPage() {
  const { orgSlug } = useParams();
  const navigate = useNavigate();
  const [projects, setProjects] = useState<Project[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!orgSlug) return;
    projectsApi
      .list(orgSlug)
      .then((res) => setProjects(res.projects))
      .catch((err) =>
        setError(err instanceof Error ? err.message : 'Failed to load projects'),
      );
  }, [orgSlug]);

  useEffect(() => {
    if (projects && projects.length === 1 && orgSlug) {
      navigate(`/m/orgs/${orgSlug}/flags/${projects[0].slug}`, { replace: true });
    }
  }, [projects, orgSlug, navigate]);

  if (error) {
    return (
      <section style={{ padding: 20 }}>
        <p style={{ color: 'var(--color-danger, #ef4444)' }}>{error}</p>
      </section>
    );
  }
  if (projects === null) {
    return (
      <section style={{ padding: 20 }}>
        <p>Loading projects…</p>
      </section>
    );
  }
  if (projects.length === 0) {
    return (
      <section style={{ padding: 20 }}>
        <h2>Flags</h2>
        <p style={{ color: 'var(--color-text-muted, #64748b)' }}>
          No projects in this org yet.
        </p>
      </section>
    );
  }

  return (
    <section style={{ padding: 20 }}>
      <h2 style={{ margin: '8px 0 16px' }}>Flags</h2>
      <ul style={{ listStyle: 'none', padding: 0, margin: 0 }}>
        {projects.map((p) => (
          <li key={p.id} className="m-list-row">
            <Link
              to={`/m/orgs/${orgSlug}/flags/${p.slug}`}
              className="m-button"
              style={{
                width: '100%',
                textAlign: 'left',
                justifyContent: 'flex-start',
                display: 'inline-flex',
                textDecoration: 'none',
              }}
            >
              {p.name}
              <span
                style={{
                  color: 'var(--color-text-muted, #64748b)',
                  marginLeft: 8,
                  fontSize: 12,
                }}
              >
                {p.slug}
              </span>
            </Link>
          </li>
        ))}
      </ul>
    </section>
  );
}
