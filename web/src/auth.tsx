import { useState, useEffect, useCallback, useRef } from 'react';
import { Navigate, useLocation, Outlet } from 'react-router-dom';
import { authApi, type AuthUser } from './api';
import { AuthContext } from './authContext';
import { useAuth } from './authHooks';
import { getTokenExpiryMs } from './authJwt';

const WARNING_LEAD_MS = 60_000;

// Clamp scheduled delays to a signed-32-bit safe range so setTimeout doesn't
// silently fire immediately on tokens with unusually long exp.
function safeDelay(ms: number): number {
  return Math.min(Math.max(ms, 0), 2_147_483_000);
}

/**
 * AuthProvider provides authentication context to the application.
 */
export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<AuthUser | null>(null);
  const [loading, setLoading] = useState(true);
  const [expiresAt, setExpiresAt] = useState<number | null>(null);
  const [expiryWarningOpen, setExpiryWarningOpen] = useState(false);

  const warnTimerRef = useRef<number | null>(null);
  const logoutTimerRef = useRef<number | null>(null);

  function clearTimers() {
    if (warnTimerRef.current !== null) {
      window.clearTimeout(warnTimerRef.current);
      warnTimerRef.current = null;
    }
    if (logoutTimerRef.current !== null) {
      window.clearTimeout(logoutTimerRef.current);
      logoutTimerRef.current = null;
    }
  }

  // Forward-declare logout so the expiry timer can close over it.
  // The actual implementation is assigned below via useCallback.
  const logoutRef = useRef<() => void>(() => {});

  const scheduleExpiry = useCallback((exp: number | null) => {
    clearTimers();
    setExpiresAt(exp);
    setExpiryWarningOpen(false);
    if (exp == null) return;

    const now = Date.now();
    const msUntilExpiry = exp - now;
    if (msUntilExpiry <= 0) {
      // Already expired.
      logoutRef.current();
      return;
    }

    const msUntilWarning = msUntilExpiry - WARNING_LEAD_MS;
    if (msUntilWarning <= 0) {
      // Within the warning window already.
      setExpiryWarningOpen(true);
    } else {
      warnTimerRef.current = window.setTimeout(() => {
        setExpiryWarningOpen(true);
      }, safeDelay(msUntilWarning));
    }

    logoutTimerRef.current = window.setTimeout(() => {
      logoutRef.current();
    }, safeDelay(msUntilExpiry));
  }, []);

  // Check for existing token on mount.
  useEffect(() => {
    const token = localStorage.getItem('ds_token');
    if (!token) {
      setLoading(false);
      return;
    }

    authApi
      .me()
      .then((u) => {
        setUser(u);
        scheduleExpiry(getTokenExpiryMs(token));
      })
      .catch(() => {
        localStorage.removeItem('ds_token');
      })
      .finally(() => setLoading(false));
    return clearTimers;
  }, [scheduleExpiry]);

  const login = useCallback(
    async (email: string, password: string) => {
      const { token, user } = await authApi.login({ email, password });
      localStorage.setItem('ds_token', token);
      setUser(user);
      scheduleExpiry(getTokenExpiryMs(token));
    },
    [scheduleExpiry],
  );

  const register = useCallback(
    async (email: string, password: string, name: string) => {
      const { token, user } = await authApi.register({ email, password, name });
      localStorage.setItem('ds_token', token);
      setUser(user);
      scheduleExpiry(getTokenExpiryMs(token));
    },
    [scheduleExpiry],
  );

  const logout = useCallback(() => {
    clearTimers();
    setExpiryWarningOpen(false);
    setExpiresAt(null);
    authApi.logout();
    setUser(null);
  }, []);
  logoutRef.current = logout;

  const extendSession = useCallback(async () => {
    const { token } = await authApi.extend();
    localStorage.setItem('ds_token', token);
    scheduleExpiry(getTokenExpiryMs(token));
  }, [scheduleExpiry]);

  return (
    <AuthContext.Provider
      value={{
        user,
        loading,
        login,
        register,
        logout,
        expiresAt,
        expiryWarningOpen,
        extendSession,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}


export function RequireAuth() {
  const { user, loading } = useAuth();
  const location = useLocation();

  if (loading) {
    return <div className="page-loading">Loading...</div>;
  }

  if (!user) {
    const next = location.pathname + location.search;
    return <Navigate to={`/login?next=${encodeURIComponent(next)}`} state={{ from: location }} replace />;
  }

  return <Outlet />;
}

export function RedirectIfAuth() {
  const { user, loading } = useAuth();

  if (loading) {
    return <div className="page-loading">Loading...</div>;
  }

  if (user) {
    return <Navigate to="/" replace />;
  }

  return <Outlet />;
}
