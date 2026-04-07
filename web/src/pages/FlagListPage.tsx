import { useState, useEffect, useMemo } from 'react';
import { Link, useParams } from 'react-router-dom';
import type { Flag, FlagCategory } from '@/types';
import { entitiesApi, flagsApi } from '@/api';

type StatusFilter = 'all' | 'enabled' | 'disabled' | 'archived';

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  });
}

export default function FlagListPage() {
  const { orgSlug, projectSlug, appSlug } = useParams();
  const contextName = appSlug ?? projectSlug ?? '';
  const heading = appSlug
    ? `${contextName} — Flags`
    : contextName
      ? `${contextName} — Feature Flags`
      : 'Feature Flags';

  const createPath = appSlug
    ? `/orgs/${orgSlug}/projects/${projectSlug}/apps/${appSlug}/flags/new`
    : `/orgs/${orgSlug}/projects/${projectSlug}/flags/new`;

  const flagDetailPath = (flagId: string) =>
    appSlug
      ? `/orgs/${orgSlug}/projects/${projectSlug}/apps/${appSlug}/flags/${flagId}`
      : `/orgs/${orgSlug}/projects/${projectSlug}/flags/${flagId}`;

  const [flags, setFlags] = useState<Flag[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [search, setSearch] = useState('');
  const [categoryFilter, setCategoryFilter] = useState<'all' | FlagCategory>('all');
  const [statusFilter, setStatusFilter] = useState<StatusFilter>('all');

  useEffect(() => {
    if (!orgSlug || !projectSlug) return;
    setLoading(true);
    setError(null);

    entitiesApi
      .getProject(orgSlug, projectSlug)
      .then((project) => flagsApi.list(project.id))
      .then((result) => setFlags(result.flags))
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, [orgSlug, projectSlug]);

  // ⚡ Bolt Optimization: Wrapped in useMemo to prevent unnecessary recalculations on every render.
  // ⚡ Bolt Optimization: Hoisted search term lowercasing outside the loop to prevent O(N) operations.
  // ⚡ Bolt Optimization: Added optional chaining/nullish coalescing to avoid runtime crashes.
  const filtered = useMemo(() => {
    const q = search?.toLowerCase() ?? '';
    return flags.filter((flag) => {
      if (q) {
        if (!flag.name?.toLowerCase().includes(q) && !flag.key?.toLowerCase().includes(q)) {
          return false;
        }
      }
      if (categoryFilter !== 'all' && flag.category !== categoryFilter) {
        return false;
      }
      if (statusFilter === 'enabled' && (!flag.enabled || flag.archived)) return false;
      if (statusFilter === 'disabled' && (flag.enabled || flag.archived)) return false;
      if (statusFilter === 'archived' && !flag.archived) return false;
      return true;
    });
  }, [flags, search, categoryFilter, statusFilter]);

  if (loading) return <div>Loading...</div>;
  if (error) return <div>Error: {error}</div>;

  return (
    <div>
      <div className="page-header-row">
        <h1 className="page-header">{heading}</h1>
        <Link to={createPath} className="btn btn-primary">
          Create Flag
        </Link>
      </div>

      <div className="filter-bar">
        <input
          type="text"
          className="form-input"
          placeholder="Search flags..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
        <select
          className="form-select"
          value={categoryFilter}
          onChange={(e) => setCategoryFilter(e.target.value as 'all' | FlagCategory)}
        >
          <option value="all">All Categories</option>
          <option value="release">Release</option>
          <option value="feature">Feature</option>
          <option value="experiment">Experiment</option>
          <option value="ops">Ops</option>
          <option value="permission">Permission</option>
        </select>
        <select
          className="form-select"
          value={statusFilter}
          onChange={(e) => setStatusFilter(e.target.value as StatusFilter)}
        >
          <option value="all">All Statuses</option>
          <option value="enabled">Enabled</option>
          <option value="disabled">Disabled</option>
          <option value="archived">Archived</option>
        </select>
      </div>

      <div className="card">
        <table>
          <thead>
            <tr>
              <th>Name / Key</th>
              <th>Category</th>
              <th>Status</th>
              <th>Owners</th>
              <th>Expires</th>
              <th>Updated</th>
            </tr>
          </thead>
          <tbody>
            {filtered.map((flag) => (
              <tr key={flag.id}>
                <td>
                  <Link to={flagDetailPath(flag.id)}>{flag.name}</Link>
                  <div className="font-mono text-muted">{flag.key}</div>
                </td>
                <td>
                  <span className={`badge badge-${flag.category}`}>{flag.category}</span>
                </td>
                <td>
                  {flag.archived ? (
                    <span className="badge badge-archived">archived</span>
                  ) : flag.enabled ? (
                    <span className="badge badge-enabled">enabled</span>
                  ) : (
                    <span className="badge badge-disabled">disabled</span>
                  )}
                </td>
                <td>{flag.owners.join(', ')}</td>
                <td>
                  {flag.is_permanent
                    ? 'Permanent'
                    : flag.expires_at
                      ? formatDate(flag.expires_at)
                      : '\u2014'}
                </td>
                <td>{formatDate(flag.updated_at)}</td>
              </tr>
            ))}
            {filtered.length === 0 && (
              <tr>
                <td colSpan={6} style={{ textAlign: 'center' }}>
                  No flags match your filters.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}
