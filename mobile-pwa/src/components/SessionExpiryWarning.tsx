import { useAuth } from '../authHooks';

export function SessionExpiryWarning() {
  const { expiryWarningOpen, extendSession, logout } = useAuth();
  if (!expiryWarningOpen) return null;

  return (
    <div
      role="dialog"
      aria-modal="true"
      style={{
        position: 'fixed',
        inset: 0,
        background: 'rgba(0,0,0,0.6)',
        display: 'flex',
        alignItems: 'flex-end',
        zIndex: 1000,
      }}
    >
      <div
        style={{
          background: 'var(--color-bg-elevated, #1b2339)',
          border: '1px solid var(--color-border, #1e293b)',
          borderRadius: '16px 16px 0 0',
          padding: '16px 20px',
          width: '100%',
          paddingBottom: 'calc(env(safe-area-inset-bottom) + 16px)',
        }}
      >
        <h3 style={{ margin: '0 0 8px' }}>Session expiring</h3>
        <p style={{ margin: '0 0 16px', color: 'var(--color-text-muted, #64748b)' }}>
          We&apos;re signing you out soon. Stay signed in to keep working.
        </p>
        <div style={{ display: 'flex', gap: 8 }}>
          <button
            type="button"
            className="m-button m-button-primary"
            style={{ flex: 1 }}
            onClick={() => {
              void extendSession();
            }}
          >
            Stay signed in
          </button>
          <button type="button" className="m-button" onClick={logout}>
            Sign out
          </button>
        </div>
      </div>
    </div>
  );
}
