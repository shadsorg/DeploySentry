import { useAuth } from '../authHooks';

function formatExpiry(ms: number | null): string {
  if (ms == null) return 'unknown';
  const secs = Math.max(0, Math.floor((ms - Date.now()) / 1000));
  if (secs <= 0) return 'expired';
  const m = Math.floor(secs / 60);
  const s = secs % 60;
  return `${m}m ${s}s`;
}

export function SettingsPage() {
  const { user, expiresAt, logout } = useAuth();
  return (
    <section>
      <h2>Account</h2>
      <div className="m-card" style={{ marginBottom: 16 }}>
        <div className="m-list-row">
          <span style={{ color: 'var(--color-text-muted, #64748b)' }}>Signed in as</span>
          <span>{user?.email ?? '—'}</span>
        </div>
        <div className="m-list-row">
          <span style={{ color: 'var(--color-text-muted, #64748b)' }}>Session expires in</span>
          <span>{formatExpiry(expiresAt)}</span>
        </div>
      </div>
      <button type="button" className="m-button" style={{ width: '100%' }} onClick={logout}>
        Sign out
      </button>
      <p style={{ color: 'var(--color-text-muted, #64748b)', fontSize: 12, marginTop: 24 }}>
        For org / project / member management, open the{' '}
        <a href="/" style={{ color: 'var(--color-primary, #6366f1)' }}>desktop dashboard</a>.
      </p>
    </section>
  );
}
