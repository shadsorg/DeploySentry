import { useState, useEffect, useCallback, useRef } from 'react';
import { Navigate, useLocation, Outlet } from 'react-router-dom';
import { authApi } from './api';
import type { AuthUser } from './types';
import { AuthContext } from './authContext';
import { useAuth } from './authHooks';
import { getTokenExpiryMs } from './authJwt';

const WARNING_LEAD_MS = 60_000;

function safeDelay(ms: number): number {
  return Math.min(Math.max(ms, 0), 2_147_483_000);
}

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<AuthUser | null>(null);
  const [loading, setLoading] = useState(true);
  const [expiresAt, setExpiresAt] = useState<number | null>(null);
  const [expiryWarningOpen, setExpiryWarningOpen] = useState(false);

  const warnTimerRef = useRef<number | null>(null);
  const logoutTimerRef = useRef<number | null>(null);
  const logoutRef = useRef<() => void>(() => {});

  const clearTimers = () => {
    if (warnTimerRef.current !== null) {
      window.clearTimeout(warnTimerRef.current);
      warnTimerRef.current = null;
    }
    if (logoutTimerRef.current !== null) {
      window.clearTimeout(logoutTimerRef.current);
      logoutTimerRef.current = null;
    }
  };

  const scheduleExpiry = useCallback((exp: number | null) => {
    clearTimers();
    setExpiresAt(exp);
    setExpiryWarningOpen(false);
    if (exp == null) return;
    const now = Date.now();
    const msUntilExpiry = exp - now;
    if (msUntilExpiry <= 0) {
      logoutRef.current();
      return;
    }
    const msUntilWarning = msUntilExpiry - WARNING_LEAD_MS;
    if (msUntilWarning <= 0) {
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
      .catch(() => localStorage.removeItem('ds_token'))
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
      value={{ user, loading, login, logout, expiresAt, expiryWarningOpen, extendSession }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export function RequireAuth() {
  const { user, loading } = useAuth();
  const location = useLocation();
  if (loading) return <div className="m-page-loading">Loading…</div>;
  if (!user) {
    const next = location.pathname + location.search;
    return <Navigate to={`/login?next=${encodeURIComponent(next)}`} state={{ from: location }} replace />;
  }
  return <Outlet />;
}

export function RedirectIfAuth() {
  const { user, loading } = useAuth();
  if (loading) return <div className="m-page-loading">Loading…</div>;
  if (user) return <Navigate to="/" replace />;
  return <Outlet />;
}
