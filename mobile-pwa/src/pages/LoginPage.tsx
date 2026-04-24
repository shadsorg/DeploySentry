import { useState } from 'react';
import { useAuth } from '../authHooks';

export function LoginPage() {
  const { login } = useAuth();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setBusy(true);
    try {
      await login(email, password);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Sign-in failed');
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="m-screen" style={{ padding: '24px 20px' }}>
      <h1 style={{ fontSize: 22, margin: '24px 0 4px' }}>Deploy Sentry</h1>
      <p style={{ color: 'var(--color-text-muted, #64748b)', marginTop: 0 }}>Sign in to continue.</p>

      <form onSubmit={onSubmit} style={{ display: 'flex', flexDirection: 'column', gap: 12, marginTop: 24 }}>
        <label>
          <span style={{ fontSize: 12, color: 'var(--color-text-muted, #64748b)' }}>Email</span>
          <input
            className="m-input"
            type="email"
            autoComplete="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            required
          />
        </label>
        <label>
          <span style={{ fontSize: 12, color: 'var(--color-text-muted, #64748b)' }}>Password</span>
          <input
            className="m-input"
            type="password"
            autoComplete="current-password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
          />
        </label>
        {error && <div style={{ color: 'var(--color-danger, #ef4444)', fontSize: 13 }}>{error}</div>}
        <button type="submit" className="m-button m-button-primary" disabled={busy}>
          {busy ? 'Signing in…' : 'Sign in'}
        </button>
      </form>

      <div style={{ textAlign: 'center', margin: '24px 0 12px', color: 'var(--color-text-muted, #64748b)', fontSize: 12 }}>
        or
      </div>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
        <a className="m-button" href="/api/v1/auth/oauth/github" style={{ textAlign: 'center' }}>
          Sign in with GitHub
        </a>
        <a className="m-button" href="/api/v1/auth/oauth/google" style={{ textAlign: 'center' }}>
          Sign in with Google
        </a>
      </div>
    </div>
  );
}
