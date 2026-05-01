import { useCallback, useEffect, useMemo, useState } from 'react';
import { useParams } from 'react-router-dom';
import { stagingApi, type StagedChange, type StagedCommitResult } from '@/api';
import StagedChangeRow from '@/components/staging/StagedChangeRow';
import { resourceTypeLabel } from '@/components/staging/labels';
import { computeConflicts, type ConflictMap } from '@/components/staging/conflicts';

/**
 * /orgs/:orgSlug/deploy-changes — review page.
 *
 * Lists the current user's pending staged_changes for this org, grouped by
 * resource type. Each row exposes a per-row Discard and a checkbox; the
 * page header has bulk Deploy Selected / Discard Selected.
 *
 * Conflict detection is intentionally render-time-only here: when the
 * staged old_value disagrees with the row's prior production value, the
 * row is flagged and unchecked by default. The backend will re-check
 * against current production at commit time as the load-bearing guard.
 */
export default function DeployChangesPage() {
  const { orgSlug } = useParams();
  const [rows, setRows] = useState<StagedChange[]>([]);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [loading, setLoading] = useState(true);
  const [committing, setCommitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [lastCommit, setLastCommit] = useState<StagedCommitResult | null>(null);

  const refresh = useCallback(async () => {
    if (!orgSlug) return;
    setLoading(true);
    try {
      const res = await stagingApi.list(orgSlug);
      setRows(res.changes);
      // Default: all rows checked except those flagged as conflicts.
      const conflicts = computeConflicts(res.changes);
      setSelected(new Set(res.changes.filter((r) => !conflicts[r.id]).map((r) => r.id)));
      setError(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'failed to load pending changes');
    } finally {
      setLoading(false);
    }
  }, [orgSlug]);

  useEffect(() => {
    refresh();
  }, [refresh]);

  const conflicts: ConflictMap = useMemo(() => computeConflicts(rows), [rows]);

  // Bucket rows by resource_type so the page reads as one section per kind.
  const grouped = useMemo(() => {
    const buckets = new Map<string, StagedChange[]>();
    for (const r of rows) {
      const list = buckets.get(r.resource_type) ?? [];
      list.push(r);
      buckets.set(r.resource_type, list);
    }
    return Array.from(buckets.entries()).sort(([a], [b]) => a.localeCompare(b));
  }, [rows]);

  const toggleSelected = useCallback((id: string) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }, []);

  const selectAll = useCallback(() => {
    setSelected(new Set(rows.map((r) => r.id)));
  }, [rows]);

  const selectNone = useCallback(() => setSelected(new Set()), []);

  const onDiscardOne = useCallback(
    async (id: string) => {
      if (!orgSlug) return;
      try {
        await stagingApi.discardOne(orgSlug, id);
        setRows((prev) => prev.filter((r) => r.id !== id));
        setSelected((prev) => {
          const next = new Set(prev);
          next.delete(id);
          return next;
        });
      } catch (e) {
        setError(e instanceof Error ? e.message : 'discard failed');
      }
    },
    [orgSlug],
  );

  const onDiscardSelected = useCallback(async () => {
    if (!orgSlug || selected.size === 0) return;
    // No bulk endpoint yet — issue per-id deletes serially. Phase B+ can
    // optimize if needed; expected size is < 50.
    setCommitting(true);
    try {
      for (const id of selected) {
        await stagingApi.discardOne(orgSlug, id);
      }
      setRows((prev) => prev.filter((r) => !selected.has(r.id)));
      setSelected(new Set());
    } catch (e) {
      setError(e instanceof Error ? e.message : 'bulk discard failed');
    } finally {
      setCommitting(false);
    }
  }, [orgSlug, selected]);

  const onDeploy = useCallback(async () => {
    if (!orgSlug || selected.size === 0) return;
    setCommitting(true);
    setError(null);
    try {
      const res = await stagingApi.commit(orgSlug, Array.from(selected));
      setLastCommit(res);
      await refresh();
      // refresh() clears `error` on success — set the failure message
      // afterwards so it survives the reload.
      if (res.failed_id) {
        setError(`Commit halted at ${res.failed_id}: ${res.failed_reason ?? 'unknown error'}`);
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : 'deploy failed');
    } finally {
      setCommitting(false);
    }
  }, [orgSlug, selected, refresh]);

  if (loading) {
    return (
      <div className="deploy-changes-page">
        <h1>Review &amp; Deploy</h1>
        <p>Loading pending changes…</p>
      </div>
    );
  }

  return (
    <div className="deploy-changes-page" data-testid="deploy-changes-page">
      <header className="deploy-changes-header">
        <div>
          <h1>Review &amp; Deploy</h1>
          <p className="deploy-changes-summary">
            {selected.size} of {rows.length} selected
          </p>
        </div>
        <div className="deploy-changes-actions">
          <button type="button" onClick={selectAll} disabled={rows.length === 0}>
            Select all
          </button>
          <button type="button" onClick={selectNone} disabled={selected.size === 0}>
            Select none
          </button>
          <button
            type="button"
            className="btn-secondary danger"
            onClick={onDiscardSelected}
            disabled={committing || selected.size === 0}
          >
            Discard selected
          </button>
          <button
            type="button"
            className="btn-primary"
            onClick={onDeploy}
            disabled={committing || selected.size === 0}
            data-testid="deploy-button"
          >
            {committing ? 'Deploying…' : `Deploy ${selected.size}`}
          </button>
        </div>
      </header>

      {error && (
        <div className="deploy-changes-error" role="alert">
          {error}
        </div>
      )}

      {lastCommit && lastCommit.committed_ids.length > 0 && !error && (
        <div className="deploy-changes-success" role="status">
          Deployed {lastCommit.committed_ids.length} change
          {lastCommit.committed_ids.length === 1 ? '' : 's'}.
        </div>
      )}

      {rows.length === 0 ? (
        <div className="deploy-changes-empty">No pending changes.</div>
      ) : (
        grouped.map(([resourceType, list]) => (
          <section key={resourceType} className="deploy-changes-section">
            <h2 className="deploy-changes-section-title">
              {resourceTypeLabel(resourceType)} <span>({list.length})</span>
            </h2>
            {list.map((row) => (
              <StagedChangeRow
                key={row.id}
                row={row}
                selected={selected.has(row.id)}
                conflict={conflicts[row.id] ?? false}
                onToggleSelected={toggleSelected}
                onDiscard={onDiscardOne}
              />
            ))}
          </section>
        ))
      )}
    </div>
  );
}
