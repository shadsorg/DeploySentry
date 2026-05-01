import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { auditApi } from '@/api';
import type { AuditLogEntry } from '@/types';
import { actionLabel } from '@/components/audit/labels';

interface Props {
  orgSlug: string;
}

function relativeTime(iso: string): string {
  const diff = Date.now() - new Date(iso).getTime();
  if (diff < 60_000) return 'just now';
  if (diff < 3_600_000) return `${Math.floor(diff / 60_000)}m ago`;
  if (diff < 86_400_000) return `${Math.floor(diff / 3_600_000)}h ago`;
  if (diff < 7 * 86_400_000) return `${Math.floor(diff / 86_400_000)}d ago`;
  return new Date(iso).toLocaleDateString();
}

export default function RecentMemberActivity({ orgSlug }: Props) {
  const [entries, setEntries] = useState<AuditLogEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    auditApi
      .query({ entity_type: 'user', limit: 10 })
      .then((res) => {
        if (cancelled) return;
        setEntries(res.entries ?? []);
      })
      .catch((err) => {
        if (cancelled) return;
        setError(err instanceof Error ? err.message : 'Failed to load activity');
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  return (
    <section className="card" style={{ marginBottom: 16 }}>
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          marginBottom: 8,
        }}
      >
        <h3 style={{ margin: 0 }}>Recent member activity</h3>
        <Link
          to={`/orgs/${orgSlug}/audit?entity_type=user`}
          className="btn btn-secondary btn-sm"
          style={{ fontSize: 12 }}
        >
          View full audit log
        </Link>
      </div>

      {loading && (
        <p className="text-muted" style={{ margin: 0 }}>
          Loading…
        </p>
      )}
      {error && !loading && (
        <p style={{ margin: 0, color: 'var(--color-error, #dc2626)' }}>{error}</p>
      )}
      {!loading && !error && entries.length === 0 && (
        <p className="text-muted" style={{ margin: 0 }}>
          No recent member activity.
        </p>
      )}
      {!loading && !error && entries.length > 0 && (
        <ul style={{ listStyle: 'none', padding: 0, margin: 0 }}>
          {entries.map((e) => (
            <li
              key={e.id}
              style={{
                display: 'flex',
                alignItems: 'baseline',
                gap: 8,
                padding: '6px 0',
                borderBottom: '1px solid var(--color-border-subtle, #eee)',
                fontSize: 13,
              }}
            >
              <span className="text-muted" style={{ minWidth: 70 }} title={e.created_at}>
                {relativeTime(e.created_at)}
              </span>
              <span style={{ minWidth: 140 }}>{e.actor_name || '(unknown)'}</span>
              <span>{actionLabel(e)}</span>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
