import { useCallback, useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';
import type { Rollout, RolloutEvent } from '@/types';
import { rolloutsApi } from '@/api';
import { RolloutStatusBadge } from '@/components/rollout/RolloutStatusBadge';
import { PhaseTimeline } from '@/components/rollout/PhaseTimeline';
import { ReasonModal } from '@/components/rollout/ReasonModal';

type ReasonAction = 'rollback' | 'force-promote';

export default function RolloutDetailPage() {
  const { orgSlug = '', id = '' } = useParams();
  const [rollout, setRollout] = useState<Rollout | null>(null);
  const [events, setEvents] = useState<RolloutEvent[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [reasonModal, setReasonModal] = useState<ReasonAction | null>(null);
  const [busy, setBusy] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      const [r, e] = await Promise.all([
        rolloutsApi.get(orgSlug, id),
        rolloutsApi.events(orgSlug, id, 100),
      ]);
      setRollout(r);
      // Server emits a nil slice as JSON `null` when no events exist yet;
      // default to [] so render-time .length doesn't crash.
      setEvents(e.items ?? []);
      setError(null);
    } catch (err) {
      setError(String(err));
    } finally {
      setLoading(false);
    }
  }, [orgSlug, id]);

  useEffect(() => {
    load();
  }, [load]);

  // Replace polling with SSE for live rollout updates.
  useEffect(() => {
    const url = rolloutsApi.eventsStreamURL(orgSlug, id);
    const token = localStorage.getItem('ds_token') || '';
    const withAuth = token ? `${url}?token=${encodeURIComponent(token)}` : url;
    const es = new EventSource(withAuth);

    es.addEventListener('snapshot', (e) => {
      try {
        setRollout(JSON.parse((e as MessageEvent).data));
      } catch {
        /* ignore */
      }
    });
    es.addEventListener('update', (e) => {
      try {
        setRollout(JSON.parse((e as MessageEvent).data));
      } catch {
        /* ignore */
      }
    });
    es.addEventListener('event', (e) => {
      try {
        const ev = JSON.parse((e as MessageEvent).data);
        setEvents((prev) => [ev, ...prev]);
      } catch {
        /* ignore */
      }
    });
    es.addEventListener('done', (e) => {
      try {
        setRollout(JSON.parse((e as MessageEvent).data));
      } catch {
        /* ignore */
      }
      es.close();
    });
    es.onerror = () => {
      /* browser auto-reconnects */
    };

    return () => es.close();
  }, [orgSlug, id]);

  async function runAction(name: string, fn: () => Promise<unknown>) {
    setBusy(name);
    try {
      await fn();
      await load();
    } catch (e) {
      alert(`${name} failed: ${e}`);
    } finally {
      setBusy(null);
    }
  }

  if (loading)
    return (
      <div className="empty-state" style={{ padding: '40px 0' }}>
        <span
          className="ms"
          style={{
            fontSize: 32,
            color: 'var(--color-primary)',
            marginBottom: 12,
            display: 'block',
          }}
        >
          sync
        </span>
        Loading rollout…
      </div>
    );
  if (error) return <div className="page-error">{error}</div>;
  if (!rollout) return <div className="page-error">Rollout not found.</div>;

  const canAct = !['succeeded', 'rolled_back', 'aborted', 'superseded'].includes(rollout.status);

  return (
    <div className="page">
      <div className="page-header-row">
        <div className="page-header" style={{ marginBottom: 0 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
            <div
              style={{
                width: 36,
                height: 36,
                borderRadius: 10,
                background: 'var(--color-primary-bg)',
                border: '1px solid rgba(99,102,241,0.25)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                flexShrink: 0,
              }}
            >
              <span className="ms" style={{ fontSize: 18, color: 'var(--color-primary)' }}>
                dynamic_feed
              </span>
            </div>
            <h1 style={{ margin: 0 }}>
              Rollout{' '}
              <span
                style={{ fontFamily: 'monospace', fontSize: 18, color: 'var(--color-primary)' }}
              >
                {rollout.id.slice(0, 8)}
              </span>
            </h1>
          </div>
        </div>
        <RolloutStatusBadge status={rollout.status} />
      </div>

      <section className="card" style={{ marginTop: 20 }}>
        <h3 style={{ fontFamily: 'var(--font-display)', fontWeight: 700, marginBottom: 12 }}>
          Strategy: {rollout.strategy_snapshot.name}
        </h3>
        <dl className="rollout-detail-grid">
          <dt>Target</dt>
          <dd>
            {rollout.target_type === 'deploy'
              ? `Deployment · ${rollout.target_ref.deployment_id?.slice(0, 8) ?? 'unknown'}`
              : `Config · flag ${rollout.target_ref.flag_key ?? rollout.target_ref.rule_id?.slice(0, 8) ?? 'unknown'}${
                  rollout.target_ref.env ? ` · env ${rollout.target_ref.env}` : ''
                }`}
          </dd>
          <dt>Phase</dt>
          <dd>
            {rollout.current_phase_index + 1} /{' '}
            {(rollout.strategy_snapshot.steps ?? []).length || '—'}
          </dd>
          <dt>Created</dt>
          <dd>{new Date(rollout.created_at).toLocaleString()}</dd>
          {rollout.completed_at && (
            <>
              <dt>Completed</dt>
              <dd>{new Date(rollout.completed_at).toLocaleString()}</dd>
            </>
          )}
        </dl>
        {rollout.rollback_reason && (
          <p className="error">Rollback reason: {rollout.rollback_reason}</p>
        )}
        <PhaseTimeline rollout={rollout} />
      </section>

      {canAct && (
        <section className="card">
          <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 12 }}>
            <span className="ms" style={{ fontSize: 16, color: 'var(--color-primary)' }}>
              play_circle
            </span>
            <h3 style={{ fontFamily: 'var(--font-display)', fontWeight: 700, margin: 0 }}>
              Actions
            </h3>
          </div>
          <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
            <button
              className="btn btn-secondary"
              disabled={busy !== null || rollout.status !== 'active'}
              onClick={() => runAction('pause', () => rolloutsApi.pause(orgSlug, rollout.id))}
            >
              <span className="ms" style={{ fontSize: 15 }}>
                pause
              </span>
              Pause
            </button>
            <button
              className="btn btn-secondary"
              disabled={busy !== null || rollout.status !== 'paused'}
              onClick={() => runAction('resume', () => rolloutsApi.resume(orgSlug, rollout.id))}
            >
              <span className="ms" style={{ fontSize: 15 }}>
                play_arrow
              </span>
              Resume
            </button>
            <button
              className="btn btn-primary"
              disabled={busy !== null || !['active', 'paused'].includes(rollout.status)}
              onClick={() => runAction('promote', () => rolloutsApi.promote(orgSlug, rollout.id))}
            >
              <span className="ms" style={{ fontSize: 15 }}>
                fast_forward
              </span>
              Promote
            </button>
            <button
              className="btn btn-primary"
              disabled={busy !== null || rollout.status !== 'awaiting_approval'}
              onClick={() => runAction('approve', () => rolloutsApi.approve(orgSlug, rollout.id))}
            >
              <span className="ms" style={{ fontSize: 15 }}>
                check_circle
              </span>
              Approve
            </button>
            <button
              className="btn"
              style={{ borderColor: 'var(--color-danger)', color: 'var(--color-danger)' }}
              disabled={busy !== null}
              onClick={() => setReasonModal('rollback')}
            >
              <span className="ms" style={{ fontSize: 15 }}>
                undo
              </span>
              Rollback
            </button>
            <button
              className="btn"
              style={{ borderColor: 'var(--color-warning)', color: 'var(--color-warning)' }}
              disabled={busy !== null}
              onClick={() => setReasonModal('force-promote')}
            >
              <span className="ms" style={{ fontSize: 15 }}>
                skip_next
              </span>
              Force Promote
            </button>
          </div>
        </section>
      )}

      <section className="card" style={{ padding: 0, overflow: 'hidden' }}>
        <div
          style={{
            padding: '12px 20px',
            borderBottom: '1px solid var(--color-border)',
            display: 'flex',
            alignItems: 'center',
            gap: 8,
          }}
        >
          <span className="ms" style={{ fontSize: 18, color: 'var(--color-primary)' }}>
            history
          </span>
          <span style={{ fontFamily: 'var(--font-display)', fontWeight: 700, fontSize: 14 }}>
            Event Log
          </span>
          {events.length > 0 && (
            <span
              className="badge"
              style={{
                background: 'var(--color-primary-bg)',
                color: 'var(--color-primary)',
                marginLeft: 4,
              }}
            >
              {events.length}
            </span>
          )}
        </div>
        {(events ?? []).length === 0 ? (
          <div className="empty-state" style={{ padding: '32px 0' }}>
            <span
              className="ms"
              style={{
                fontSize: 32,
                display: 'block',
                marginBottom: 8,
                color: 'var(--color-text-muted)',
              }}
            >
              event_note
            </span>
            No events yet.
          </div>
        ) : (
          <ul style={{ listStyle: 'none', margin: 0, padding: 0 }}>
            {(events ?? []).map((ev) => (
              <li
                key={ev.id}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: 12,
                  padding: '10px 20px',
                  borderBottom: '1px solid var(--color-border)',
                }}
              >
                <span
                  style={{
                    fontSize: 11,
                    color: 'var(--color-text-muted)',
                    fontFamily: 'monospace',
                    minWidth: 160,
                  }}
                >
                  {new Date(ev.occurred_at).toLocaleString()}
                </span>
                <span className="badge badge-ops">{ev.event_type}</span>
                <span style={{ fontSize: 12, color: 'var(--color-text-secondary)' }}>
                  {ev.actor_type}
                  {ev.actor_id ? ` · ${ev.actor_id.slice(0, 8)}` : ''}
                </span>
                {ev.reason && (
                  <span
                    style={{ fontSize: 12, color: 'var(--color-text-muted)', fontStyle: 'italic' }}
                  >
                    "{ev.reason}"
                  </span>
                )}
              </li>
            ))}
          </ul>
        )}
      </section>

      {reasonModal === 'rollback' && (
        <ReasonModal
          title="Rollback Rollout"
          placeholder="Why are you rolling back?"
          required
          onConfirm={async (reason) => {
            setReasonModal(null);
            await runAction('rollback', () => rolloutsApi.rollback(orgSlug, rollout.id, reason));
          }}
          onCancel={() => setReasonModal(null)}
        />
      )}
      {reasonModal === 'force-promote' && (
        <ReasonModal
          title="Force Promote (Override Health)"
          placeholder="Explain why health gates are being overridden"
          required
          onConfirm={async (reason) => {
            setReasonModal(null);
            await runAction('force-promote', () =>
              rolloutsApi.forcePromote(orgSlug, rollout.id, reason),
            );
          }}
          onCancel={() => setReasonModal(null)}
        />
      )}
    </div>
  );
}
