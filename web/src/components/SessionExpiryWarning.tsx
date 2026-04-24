import { useEffect, useState } from 'react';
import { useAuth } from '@/authHooks';

/**
 * Modal shown in the last 60 seconds before the access token expires.
 * Lets the user extend the session without re-entering credentials, or
 * sign out immediately.
 */
export default function SessionExpiryWarning() {
  const { expiryWarningOpen, expiresAt, extendSession, logout } = useAuth();
  const [now, setNow] = useState(Date.now());
  const [extending, setExtending] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!expiryWarningOpen) return;
    const id = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(id);
  }, [expiryWarningOpen]);

  if (!expiryWarningOpen || expiresAt == null) return null;

  const secondsLeft = Math.max(0, Math.ceil((expiresAt - now) / 1000));

  async function handleExtend() {
    setExtending(true);
    setError(null);
    try {
      await extendSession();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Could not refresh session');
    } finally {
      setExtending(false);
    }
  }

  return (
    <div className="modal-backdrop" role="dialog" aria-modal="true" aria-labelledby="session-expiry-title">
      <div className="modal" style={{ maxWidth: 440 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 12 }}>
          <div style={{
            width: 36, height: 36, borderRadius: 10, flexShrink: 0,
            background: 'var(--color-warning-bg)', border: '1px solid rgba(245,158,11,0.3)',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
          }}>
            <span className="ms" style={{ fontSize: 20, color: 'var(--color-warning)' }}>schedule</span>
          </div>
          <h3 id="session-expiry-title" style={{ margin: 0 }}>Your session is about to expire</h3>
        </div>

        <p style={{ color: 'var(--color-text-secondary)', fontSize: 14, marginBottom: 16 }}>
          You&apos;ll be signed out in{' '}
          <strong style={{ color: 'var(--color-text)', fontVariantNumeric: 'tabular-nums' }}>
            {secondsLeft}s
          </strong>
          . Stay signed in to keep working.
        </p>

        {error && <div className="form-error" style={{ marginBottom: 12 }}>{error}</div>}

        <div className="modal-actions">
          <button type="button" className="btn btn-secondary" onClick={logout} disabled={extending}>
            Sign out
          </button>
          <button type="button" className="btn btn-primary" onClick={handleExtend} disabled={extending}>
            <span className="ms" style={{ fontSize: 16 }}>
              {extending ? 'hourglass_empty' : 'refresh'}
            </span>
            {extending ? 'Refreshing…' : 'Stay signed in'}
          </button>
        </div>
      </div>
    </div>
  );
}
