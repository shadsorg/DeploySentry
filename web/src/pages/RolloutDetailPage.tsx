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
      setEvents(e.items);
      setError(null);
    } catch (err) {
      setError(String(err));
    } finally {
      setLoading(false);
    }
  }, [orgSlug, id]);

  useEffect(() => { load(); }, [load]);

  // Poll while the rollout is non-terminal.
  useEffect(() => {
    if (!rollout) return;
    const terminal = ['succeeded', 'rolled_back', 'aborted', 'superseded'];
    if (terminal.includes(rollout.status)) return;
    const t = setInterval(load, 3000);
    return () => clearInterval(t);
  }, [rollout, load]);

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

  if (loading) return <div className="page"><p>Loading…</p></div>;
  if (error) return <div className="page"><p className="error">{error}</p></div>;
  if (!rollout) return <div className="page"><p>Rollout not found.</p></div>;

  const canAct = !['succeeded', 'rolled_back', 'aborted', 'superseded'].includes(rollout.status);

  return (
    <div className="page">
      <header className="page-header">
        <h1>Rollout {rollout.id.slice(0, 8)}</h1>
        <div>
          <RolloutStatusBadge status={rollout.status} />
        </div>
      </header>

      <section className="card">
        <h3>Strategy: {rollout.strategy_snapshot.name}</h3>
        <p>Target type: {rollout.target_type}</p>
        <p>Phase: {rollout.current_phase_index + 1} / {rollout.strategy_snapshot.steps.length}</p>
        {rollout.rollback_reason && <p className="error">Rollback reason: {rollout.rollback_reason}</p>}
        <PhaseTimeline rollout={rollout} />
      </section>

      {canAct && (
        <section className="card">
          <h3>Actions</h3>
          <div className="action-bar">
            <button
              disabled={busy !== null || rollout.status !== 'active'}
              onClick={() => runAction('pause', () => rolloutsApi.pause(orgSlug, rollout.id))}
            >
              Pause
            </button>
            <button
              disabled={busy !== null || rollout.status !== 'paused'}
              onClick={() => runAction('resume', () => rolloutsApi.resume(orgSlug, rollout.id))}
            >
              Resume
            </button>
            <button
              disabled={busy !== null || !['active', 'paused'].includes(rollout.status)}
              onClick={() => runAction('promote', () => rolloutsApi.promote(orgSlug, rollout.id))}
            >
              Promote
            </button>
            <button
              disabled={busy !== null || rollout.status !== 'awaiting_approval'}
              onClick={() => runAction('approve', () => rolloutsApi.approve(orgSlug, rollout.id))}
            >
              Approve
            </button>
            <button
              disabled={busy !== null}
              className="btn-danger"
              onClick={() => setReasonModal('rollback')}
            >
              Rollback
            </button>
            <button
              disabled={busy !== null}
              className="btn-warning"
              onClick={() => setReasonModal('force-promote')}
            >
              Force Promote
            </button>
          </div>
        </section>
      )}

      <section className="card">
        <h3>Event Log</h3>
        {events.length === 0 ? (
          <p className="empty-state">No events yet.</p>
        ) : (
          <ul className="event-list">
            {events.map((ev) => (
              <li key={ev.id}>
                <span className="event-time">{new Date(ev.occurred_at).toLocaleString()}</span>
                <span className="event-type">{ev.event_type}</span>
                <span className="event-actor">{ev.actor_type}{ev.actor_id ? ` · ${ev.actor_id.slice(0, 8)}` : ''}</span>
                {ev.reason && <span className="event-reason">"{ev.reason}"</span>}
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
            await runAction('force-promote', () => rolloutsApi.forcePromote(orgSlug, rollout.id, reason));
          }}
          onCancel={() => setReasonModal(null)}
        />
      )}
    </div>
  );
}
