import { useCallback, useEffect, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { stagingApi } from '@/api';
import ConfirmDialog from '@/components/ConfirmDialog';

const POLL_INTERVAL_MS = 30_000;

/**
 * Sticky banner shown whenever the current user has staged dashboard
 * mutations pending in the active org. Polls every 30s and on tab focus.
 * Hidden when the count is zero so it never adds visual noise to clean
 * sessions.
 */
export default function StagingHeaderBanner() {
  const { orgSlug } = useParams();
  const [count, setCount] = useState(0);
  const [confirmDiscard, setConfirmDiscard] = useState(false);
  const [discarding, setDiscarding] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    if (!orgSlug) return;
    try {
      const res = await stagingApi.list(orgSlug);
      setCount(res.count);
      setError(null);
    } catch (e) {
      // Failure here is non-fatal — banner just stays at its last state.
      // We don't want a transient API blip to clobber a real "5 pending"
      // state with "0", so we only zero out on a clean response.
      setError(e instanceof Error ? e.message : 'failed to load pending changes');
    }
  }, [orgSlug]);

  useEffect(() => {
    if (!orgSlug) return;
    refresh();
    const id = setInterval(refresh, POLL_INTERVAL_MS);
    const onFocus = () => refresh();
    window.addEventListener('focus', onFocus);
    return () => {
      clearInterval(id);
      window.removeEventListener('focus', onFocus);
    };
  }, [orgSlug, refresh]);

  const onDiscardAll = useCallback(async () => {
    if (!orgSlug) return;
    setDiscarding(true);
    try {
      await stagingApi.discardAll(orgSlug);
      setCount(0);
      setConfirmDiscard(false);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'discard failed');
    } finally {
      setDiscarding(false);
    }
  }, [orgSlug]);

  if (!orgSlug || count === 0) return null;

  return (
    <>
      <div
        className="staging-banner"
        role="status"
        aria-live="polite"
        data-testid="staging-banner"
      >
        <span className="staging-banner-icon" aria-hidden="true">
          ●
        </span>
        <span className="staging-banner-text">
          You have {count} pending change{count === 1 ? '' : 's'}
        </span>
        {error && <span className="staging-banner-error">({error})</span>}
        <div className="staging-banner-actions">
          <Link
            className="staging-banner-deploy"
            to={`/orgs/${orgSlug}/deploy-changes`}
          >
            Review &amp; Deploy →
          </Link>
          <button
            type="button"
            className="staging-banner-discard"
            onClick={() => setConfirmDiscard(true)}
          >
            Discard all
          </button>
        </div>
      </div>
      <ConfirmDialog
        open={confirmDiscard}
        title="Discard all pending changes?"
        message={`This will permanently drop your ${count} pending change${count === 1 ? '' : 's'} for this organization. Production is not affected.`}
        confirmLabel="Discard all"
        confirmVariant="danger"
        loading={discarding}
        onConfirm={onDiscardAll}
        onCancel={() => setConfirmDiscard(false)}
      />
    </>
  );
}
