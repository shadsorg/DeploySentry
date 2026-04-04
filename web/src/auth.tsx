import { createContext, useContext, useState, useEffect, useCallback } from 'react';
import { Navigate, useLocation, Outlet } from 'react-router-dom';
import { authApi, type AuthUser } from './api';
import { type AuthContextValue } from './authTypes';

const AuthContext = createContext<AuthContextValue | null>(null);

/**
 * AuthProvider provides authentication context to the application.
 */
export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<AuthUser | null>(null);
  const [loading, setLoading] = useState(true);

  // Check for existing token on mount
  useEffect(() => {
    const token = localStorage.getItem('ds_token');
    if (!token) {
      setLoading(false);
      return;
    }

    authApi.me()
      .then(setUser)
      .catch(() => {
        localStorage.removeItem('ds_token');
      })
      .finally(() => setLoading(false));
  }, []);

  const login = useCallback(async (email: string, password: string) => {
    const { token, user } = await authApi.login({ email, password });
    localStorage.setItem('ds_token', token);
    setUser(user);
  }, []);

  const register = useCallback(async (email: string, password: string, name: string) => {
    const { token, user } = await authApi.register({ email, password, name });
    localStorage.setItem('ds_token', token);
    setUser(user);
  }, []);

  const logout = useCallback(() => {
    authApi.logout();
    setUser(null);
  }, []);

  return (
    <AuthContext.Provider value={{ user, loading, login, register, logout }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error('useAuth must be used within AuthProvider');
  return ctx;
}

export function RequireAuth() {
  const { user, loading } = useAuth();
  const location = useLocation();

  if (loading) {
    return <div className="page-loading">Loading...</div>;
  }

  if (!user) {
    return <Navigate to="/login" state={{ from: location }} replace />;
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