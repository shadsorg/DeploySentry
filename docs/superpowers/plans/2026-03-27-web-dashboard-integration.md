# Web Dashboard Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Connect the React dashboard to the real backend API — add login flow, replace all mock data with API calls, and add SSE real-time updates.

**Architecture:** AuthProvider manages JWT tokens. ProtectedRoute gates access. Each page uses the existing api.ts client. useSSE hook provides real-time flag updates.

**Tech Stack:** React 18, TypeScript, React Router v6, Vite

**Spec:** `docs/superpowers/specs/2026-03-27-deploysentry-production-readiness-design.md` (Section 7b)

---

## Current State

- `web/src/api.ts` already has a full API client with `flagsApi`, `deploymentsApi`, `releasesApi`, `apiKeysApi`, and `healthApi`. It reads `ds_token` from `localStorage` and sends `Bearer` or `ApiKey` headers automatically.
- `web/src/types.ts` has all TypeScript interfaces matching the backend models.
- All 8 pages (`DashboardPage`, `FlagListPage`, `FlagCreatePage`, `FlagDetailPage`, `DeploymentsPage`, `ReleasesPage`, `SDKsPage`, `SettingsPage`) use hardcoded mock data.
- `web/src/App.tsx` has flat routing under a `<Layout />` wrapper with no auth gating.
- `web/src/main.tsx` renders `<BrowserRouter><App /></BrowserRouter>` with no providers.

---

## File Map

| File | Action | Responsibility |
|------|--------|---------------|
| `web/src/types.ts` | Modify | Add auth-related types |
| `web/src/api.ts` | Modify | Add auth API methods, add 401 interception |
| `web/src/components/AuthProvider.tsx` | Create | React context for auth state, token management |
| `web/src/components/ProtectedRoute.tsx` | Create | Route guard, redirects unauthenticated users |
| `web/src/pages/LoginPage.tsx` | Create | Email/password login form |
| `web/src/pages/RegisterPage.tsx` | Create | Registration form |
| `web/src/App.tsx` | Modify | Add auth routes, wrap protected routes |
| `web/src/main.tsx` | Modify | Wrap app with AuthProvider |
| `web/src/hooks/useSSE.ts` | Create | SSE connection hook for real-time flag updates |
| `web/src/components/ErrorBoundary.tsx` | Create | Per-page error boundary |
| `web/src/components/LoadingSkeleton.tsx` | Create | Reusable loading skeleton |
| `web/src/components/ErrorToast.tsx` | Create | Global error toast for network failures |
| `web/src/pages/DashboardPage.tsx` | Modify | Replace mock data with API calls |
| `web/src/pages/FlagListPage.tsx` | Modify | Replace mock data with API calls |
| `web/src/pages/FlagCreatePage.tsx` | Modify | Wire form to POST /api/v1/flags |
| `web/src/pages/FlagDetailPage.tsx` | Modify | Replace mock data with API calls |
| `web/src/pages/DeploymentsPage.tsx` | Modify | Replace mock data with API calls |
| `web/src/pages/ReleasesPage.tsx` | Modify | Replace mock data with API calls |
| `web/src/pages/SettingsPage.tsx` | Modify | Replace mock data with API calls |

---

### Task 1: Auth Types and API Methods

**Files:**
- Modify: `web/src/types.ts`
- Modify: `web/src/api.ts`

- [ ] **Step 1: Add auth types to `types.ts`**

Add these types at the end of `web/src/types.ts`:

```typescript
export interface AuthUser {
  id: string;
  email: string;
}

export interface LoginRequest {
  email: string;
  password: string;
}

export interface RegisterRequest {
  email: string;
  password: string;
  name: string;
}

export interface AuthResponse {
  token: string;
  user: AuthUser;
}
```

- [ ] **Step 2: Add auth API and 401 interception to `api.ts`**

Add a 401 interception to the existing `request()` function. When a 401 is returned, clear the token from localStorage and redirect to `/login`:

```typescript
// Inside the request() function, after the res.ok check:
if (res.status === 401) {
  localStorage.removeItem('ds_token');
  window.location.href = '/login';
  throw new Error('Session expired');
}
```

Add a new `authApi` export at the top of the API exports:

```typescript
export const authApi = {
  login: (data: LoginRequest) =>
    request<AuthResponse>('/auth/login', {
      method: 'POST',
      body: JSON.stringify(data),
    }),
  register: (data: RegisterRequest) =>
    request<AuthResponse>('/auth/register', {
      method: 'POST',
      body: JSON.stringify(data),
    }),
};
```

Note: The `authApi` calls must NOT include the Bearer/ApiKey header (user is not logged in yet). Modify the `request()` function to skip the Authorization header when `token` is empty:

```typescript
const headers: Record<string, string> = {
  'Content-Type': 'application/json',
  ...init?.headers as Record<string, string>,
};
if (token) {
  headers.Authorization = token.startsWith('ds_') ? `ApiKey ${token}` : `Bearer ${token}`;
}
```

**Commit message:** `feat(web): add auth types and API methods with 401 interception`

---

### Task 2: AuthProvider Context

**Files:**
- Create: `web/src/components/AuthProvider.tsx`

- [ ] **Step 1: Create AuthProvider with React context**

```typescript
// web/src/components/AuthProvider.tsx
import { createContext, useContext, useState, useCallback, useEffect, type ReactNode } from 'react';
import type { AuthUser } from '@/types';

interface AuthContextValue {
  user: AuthUser | null;
  token: string | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  login: (token: string, user: AuthUser) => void;
  logout: () => void;
}

const AuthContext = createContext<AuthContextValue | null>(null);

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error('useAuth must be used within AuthProvider');
  return ctx;
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [token, setToken] = useState<string | null>(null);
  const [user, setUser] = useState<AuthUser | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  // Restore token from localStorage on mount
  useEffect(() => {
    const stored = localStorage.getItem('ds_token');
    const storedUser = localStorage.getItem('ds_user');
    if (stored && storedUser) {
      setToken(stored);
      try {
        setUser(JSON.parse(storedUser));
      } catch {
        localStorage.removeItem('ds_token');
        localStorage.removeItem('ds_user');
      }
    }
    setIsLoading(false);
  }, []);

  const login = useCallback((newToken: string, newUser: AuthUser) => {
    localStorage.setItem('ds_token', newToken);
    localStorage.setItem('ds_user', JSON.stringify(newUser));
    setToken(newToken);
    setUser(newUser);
  }, []);

  const logout = useCallback(() => {
    localStorage.removeItem('ds_token');
    localStorage.removeItem('ds_user');
    setToken(null);
    setUser(null);
  }, []);

  return (
    <AuthContext.Provider
      value={{
        user,
        token,
        isAuthenticated: !!token,
        isLoading,
        login,
        logout,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}
```

Key design decisions:
- Stores both `ds_token` and `ds_user` in localStorage for session persistence across refreshes.
- `isLoading` is true during initial hydration from localStorage, preventing a flash of the login page.
- `login()` and `logout()` are memoized callbacks to avoid re-renders.

**Commit message:** `feat(web): add AuthProvider context for JWT auth state management`

---

### Task 3: LoginPage and RegisterPage

**Files:**
- Create: `web/src/pages/LoginPage.tsx`
- Create: `web/src/pages/RegisterPage.tsx`

- [ ] **Step 1: Create LoginPage**

```typescript
// web/src/pages/LoginPage.tsx
import { useState, type FormEvent } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { authApi } from '@/api';
import { useAuth } from '@/components/AuthProvider';

export default function LoginPage() {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const { login } = useAuth();
  const navigate = useNavigate();

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setError('');
    setLoading(true);
    try {
      const res = await authApi.login({ email, password });
      login(res.token, res.user);
      navigate('/');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Login failed');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="auth-page">
      <div className="auth-card">
        <div className="auth-header">
          <div className="sidebar-logo">DS</div>
          <h1>Sign in to DeploySentry</h1>
        </div>
        {error && <div className="alert alert-error">{error}</div>}
        <form onSubmit={handleSubmit}>
          <div className="form-group">
            <label className="form-label" htmlFor="email">Email</label>
            <input
              id="email"
              className="form-input"
              type="email"
              required
              autoComplete="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
            />
          </div>
          <div className="form-group">
            <label className="form-label" htmlFor="password">Password</label>
            <input
              id="password"
              className="form-input"
              type="password"
              required
              autoComplete="current-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
          </div>
          <button type="submit" className="btn btn-primary" style={{ width: '100%' }} disabled={loading}>
            {loading ? 'Signing in...' : 'Sign in'}
          </button>
        </form>
        <p className="auth-footer">
          Don't have an account? <Link to="/register">Register</Link>
        </p>
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Create RegisterPage**

```typescript
// web/src/pages/RegisterPage.tsx
import { useState, type FormEvent } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { authApi } from '@/api';
import { useAuth } from '@/components/AuthProvider';

export default function RegisterPage() {
  const [name, setName] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const { login } = useAuth();
  const navigate = useNavigate();

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setError('');
    setLoading(true);
    try {
      const res = await authApi.register({ email, password, name });
      login(res.token, res.user);
      navigate('/');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Registration failed');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="auth-page">
      <div className="auth-card">
        <div className="auth-header">
          <div className="sidebar-logo">DS</div>
          <h1>Create your account</h1>
        </div>
        {error && <div className="alert alert-error">{error}</div>}
        <form onSubmit={handleSubmit}>
          <div className="form-group">
            <label className="form-label" htmlFor="name">Name</label>
            <input
              id="name"
              className="form-input"
              type="text"
              required
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
          </div>
          <div className="form-group">
            <label className="form-label" htmlFor="email">Email</label>
            <input
              id="email"
              className="form-input"
              type="email"
              required
              autoComplete="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
            />
          </div>
          <div className="form-group">
            <label className="form-label" htmlFor="password">Password</label>
            <input
              id="password"
              className="form-input"
              type="password"
              required
              minLength={8}
              autoComplete="new-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
            <span className="form-hint">Minimum 8 characters</span>
          </div>
          <button type="submit" className="btn btn-primary" style={{ width: '100%' }} disabled={loading}>
            {loading ? 'Creating account...' : 'Create account'}
          </button>
        </form>
        <p className="auth-footer">
          Already have an account? <Link to="/login">Sign in</Link>
        </p>
      </div>
    </div>
  );
}
```

- [ ] **Step 3: Add auth page styles to `globals.css`**

Append to `web/src/styles/globals.css`:

```css
/* Auth pages */
.auth-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--color-bg);
}

.auth-card {
  width: 100%;
  max-width: 400px;
  padding: 2rem;
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: 8px;
}

.auth-header {
  text-align: center;
  margin-bottom: 1.5rem;
}

.auth-header h1 {
  font-size: 1.25rem;
  margin-top: 0.75rem;
}

.auth-footer {
  text-align: center;
  margin-top: 1rem;
  font-size: 0.875rem;
  color: var(--color-text-muted);
}

.alert-error {
  padding: 0.75rem;
  margin-bottom: 1rem;
  border-radius: 6px;
  background: rgba(var(--color-danger-rgb, 239, 68, 68), 0.1);
  color: var(--color-danger);
  font-size: 0.875rem;
  border: 1px solid rgba(var(--color-danger-rgb, 239, 68, 68), 0.2);
}
```

**Commit message:** `feat(web): add LoginPage and RegisterPage with auth forms`

---

### Task 4: ProtectedRoute and App.tsx Routing

**Files:**
- Create: `web/src/components/ProtectedRoute.tsx`
- Modify: `web/src/App.tsx`
- Modify: `web/src/main.tsx`

- [ ] **Step 1: Create ProtectedRoute**

```typescript
// web/src/components/ProtectedRoute.tsx
import { Navigate, useLocation } from 'react-router-dom';
import { useAuth } from './AuthProvider';

export default function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, isLoading } = useAuth();
  const location = useLocation();

  if (isLoading) {
    return <div className="page-loading">Loading...</div>;
  }

  if (!isAuthenticated) {
    return <Navigate to="/login" state={{ from: location }} replace />;
  }

  return <>{children}</>;
}
```

- [ ] **Step 2: Update `App.tsx` to add auth routes and wrap protected routes**

Replace the entire content of `web/src/App.tsx`:

```typescript
import { Routes, Route } from 'react-router-dom';
import Layout from './components/Layout';
import ProtectedRoute from './components/ProtectedRoute';
import LoginPage from './pages/LoginPage';
import RegisterPage from './pages/RegisterPage';
import DashboardPage from './pages/DashboardPage';
import FlagListPage from './pages/FlagListPage';
import FlagDetailPage from './pages/FlagDetailPage';
import FlagCreatePage from './pages/FlagCreatePage';
import DeploymentsPage from './pages/DeploymentsPage';
import ReleasesPage from './pages/ReleasesPage';
import SDKsPage from './pages/SDKsPage';
import SettingsPage from './pages/SettingsPage';

export default function App() {
  return (
    <Routes>
      {/* Public routes */}
      <Route path="/login" element={<LoginPage />} />
      <Route path="/register" element={<RegisterPage />} />

      {/* Protected routes */}
      <Route
        element={
          <ProtectedRoute>
            <Layout />
          </ProtectedRoute>
        }
      >
        <Route index element={<DashboardPage />} />
        <Route path="flags" element={<FlagListPage />} />
        <Route path="flags/new" element={<FlagCreatePage />} />
        <Route path="flags/:id" element={<FlagDetailPage />} />
        <Route path="deployments" element={<DeploymentsPage />} />
        <Route path="releases" element={<ReleasesPage />} />
        <Route path="sdks" element={<SDKsPage />} />
        <Route path="settings" element={<SettingsPage />} />
      </Route>
    </Routes>
  );
}
```

- [ ] **Step 3: Wrap app with AuthProvider in `main.tsx`**

Replace the entire content of `web/src/main.tsx`:

```typescript
import React from 'react';
import ReactDOM from 'react-dom/client';
import { BrowserRouter } from 'react-router-dom';
import { AuthProvider } from './components/AuthProvider';
import App from './App';
import './styles/globals.css';

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <BrowserRouter>
      <AuthProvider>
        <App />
      </AuthProvider>
    </BrowserRouter>
  </React.StrictMode>,
);
```

**Commit message:** `feat(web): add ProtectedRoute guard and wire auth routing`

---

### Task 5: Error Boundary, Loading Skeleton, and Error Toast

**Files:**
- Create: `web/src/components/ErrorBoundary.tsx`
- Create: `web/src/components/LoadingSkeleton.tsx`
- Create: `web/src/components/ErrorToast.tsx`

- [ ] **Step 1: Create ErrorBoundary**

```typescript
// web/src/components/ErrorBoundary.tsx
import { Component, type ErrorInfo, type ReactNode } from 'react';

interface Props {
  children: ReactNode;
  fallback?: ReactNode;
}

interface State {
  hasError: boolean;
  error: Error | null;
}

export default class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props);
    this.state = { hasError: false, error: null };
  }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('ErrorBoundary caught:', error, info.componentStack);
  }

  render() {
    if (this.state.hasError) {
      if (this.props.fallback) return this.props.fallback;
      return (
        <div className="card" style={{ textAlign: 'center', padding: '3rem' }}>
          <h2>Something went wrong</h2>
          <p className="text-muted">{this.state.error?.message ?? 'An unexpected error occurred.'}</p>
          <button
            className="btn btn-secondary"
            onClick={() => this.setState({ hasError: false, error: null })}
            style={{ marginTop: '1rem' }}
          >
            Try again
          </button>
        </div>
      );
    }
    return this.props.children;
  }
}
```

- [ ] **Step 2: Create LoadingSkeleton**

```typescript
// web/src/components/LoadingSkeleton.tsx

interface Props {
  rows?: number;
  type?: 'table' | 'card' | 'stat';
}

export default function LoadingSkeleton({ rows = 5, type = 'table' }: Props) {
  if (type === 'stat') {
    return (
      <div className="stat-grid">
        {Array.from({ length: 4 }).map((_, i) => (
          <div key={i} className="stat-card skeleton-pulse">
            <div className="skeleton-line" style={{ width: '60%', height: 12 }} />
            <div className="skeleton-line" style={{ width: '40%', height: 28, marginTop: 8 }} />
            <div className="skeleton-line" style={{ width: '80%', height: 10, marginTop: 8 }} />
          </div>
        ))}
      </div>
    );
  }

  if (type === 'card') {
    return (
      <div className="card skeleton-pulse">
        {Array.from({ length: rows }).map((_, i) => (
          <div key={i} className="skeleton-line" style={{ width: `${70 + Math.random() * 30}%`, marginBottom: 12 }} />
        ))}
      </div>
    );
  }

  return (
    <div className="card">
      <table>
        <tbody>
          {Array.from({ length: rows }).map((_, i) => (
            <tr key={i}>
              <td colSpan={6}>
                <div className="skeleton-line skeleton-pulse" style={{ width: `${50 + Math.random() * 50}%` }} />
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
```

- [ ] **Step 3: Create ErrorToast**

```typescript
// web/src/components/ErrorToast.tsx
import { useState, useEffect, createContext, useContext, useCallback, type ReactNode } from 'react';

interface ToastContextValue {
  showError: (message: string) => void;
}

const ToastContext = createContext<ToastContextValue>({ showError: () => {} });

export function useToast(): ToastContextValue {
  return useContext(ToastContext);
}

export function ToastProvider({ children }: { children: ReactNode }) {
  const [message, setMessage] = useState<string | null>(null);

  const showError = useCallback((msg: string) => {
    setMessage(msg);
  }, []);

  useEffect(() => {
    if (!message) return;
    const timer = setTimeout(() => setMessage(null), 5000);
    return () => clearTimeout(timer);
  }, [message]);

  return (
    <ToastContext.Provider value={{ showError }}>
      {children}
      {message && (
        <div className="toast toast-error">
          <span>{message}</span>
          <button className="toast-close" onClick={() => setMessage(null)}>
            x
          </button>
        </div>
      )}
    </ToastContext.Provider>
  );
}
```

- [ ] **Step 4: Add skeleton and toast styles to `globals.css`**

Append to `web/src/styles/globals.css`:

```css
/* Loading skeletons */
.skeleton-line {
  height: 14px;
  border-radius: 4px;
  background: var(--color-border);
}

@keyframes skeletonPulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.5; }
}

.skeleton-pulse {
  animation: skeletonPulse 1.5s ease-in-out infinite;
}

/* Toast notifications */
.toast {
  position: fixed;
  bottom: 1.5rem;
  right: 1.5rem;
  padding: 0.75rem 1rem;
  border-radius: 8px;
  font-size: 0.875rem;
  display: flex;
  align-items: center;
  gap: 0.75rem;
  z-index: 9999;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
}

.toast-error {
  background: var(--color-danger);
  color: #fff;
}

.toast-close {
  background: none;
  border: none;
  color: inherit;
  cursor: pointer;
  font-size: 1rem;
  padding: 0;
  opacity: 0.8;
}

.toast-close:hover {
  opacity: 1;
}

/* Page loading */
.page-loading {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 200px;
  color: var(--color-text-muted);
}
```

**Commit message:** `feat(web): add ErrorBoundary, LoadingSkeleton, and ErrorToast components`

---

### Task 6: Connect DashboardPage to API

**Files:**
- Modify: `web/src/pages/DashboardPage.tsx`

- [ ] **Step 1: Replace mock data with API calls**

Rewrite `DashboardPage.tsx` to fetch real data. Remove all `MOCK_*` constants, the local `Deployment` interface, and `ActivityEvent`/`ExpiringFlag` interfaces. Replace with state fetched from API.

Key changes:
- Add `useState` + `useEffect` to load flags, deployments, and releases on mount.
- Compute dashboard stats from real data: total flags by category, active deployments (status `running`), flags with upcoming expiration.
- Show `LoadingSkeleton type="stat"` while data loads.
- Wrap the component in an `ErrorBoundary`.

```typescript
import { useState, useEffect } from 'react';
import { flagsApi, deploymentsApi, releasesApi } from '@/api';
import type { Flag, Deployment, Release } from '@/types';
import LoadingSkeleton from '@/components/LoadingSkeleton';
import ErrorBoundary from '@/components/ErrorBoundary';

// project_id is hardcoded for v1 (single-project mode)
const PROJECT_ID = 'proj-1';
```

Fetch pattern (used across all page conversions):

```typescript
const [flags, setFlags] = useState<Flag[]>([]);
const [deployments, setDeployments] = useState<Deployment[]>([]);
const [releases, setReleases] = useState<Release[]>([]);
const [loading, setLoading] = useState(true);
const [error, setError] = useState<string | null>(null);

useEffect(() => {
  async function load() {
    try {
      const [flagRes, depRes, relRes] = await Promise.all([
        flagsApi.list(PROJECT_ID),
        deploymentsApi.list(PROJECT_ID),
        releasesApi.list(PROJECT_ID),
      ]);
      setFlags(flagRes.flags ?? []);
      setDeployments(depRes.deployments ?? []);
      setReleases(relRes.releases ?? []);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load dashboard');
    } finally {
      setLoading(false);
    }
  }
  load();
}, []);
```

Compute stats from real data:

```typescript
const categories = ['release', 'feature', 'experiment', 'ops', 'permission'] as const;
const flagsByCategory = categories.map((cat) => ({
  key: cat,
  label: cat.charAt(0).toUpperCase() + cat.slice(1),
  count: flags.filter((f) => f.category === cat && !f.archived).length,
}));
const totalFlags = flags.filter((f) => !f.archived).length;
const activeDeployments = deployments.filter((d) => d.status === 'running');
const expiringSoon = flags
  .filter((f) => f.expires_at && !f.archived)
  .map((f) => ({ ...f, daysLeft: Math.ceil((new Date(f.expires_at!).getTime() - Date.now()) / 86400000) }))
  .filter((f) => f.daysLeft > 0 && f.daysLeft <= 14)
  .sort((a, b) => a.daysLeft - b.daysLeft);
```

- Keep the existing JSX structure and CSS classes (stat-grid, stat-card, card, table, etc.).
- Replace hardcoded numbers with computed values.
- Replace the "Recent Activity" section with the 5 most recently updated flags.
- Replace the "Active Deployments" section with real running deployments.
- Show loading skeleton when `loading` is true.
- Show error message when `error` is set.

**Commit message:** `feat(web): connect DashboardPage to live API data`

---

### Task 7: Connect FlagListPage to API

**Files:**
- Modify: `web/src/pages/FlagListPage.tsx`

- [ ] **Step 1: Replace mock data with API call**

Remove the entire `MOCK_FLAGS` array (lines 5-216). Replace with API fetch:

```typescript
import { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { flagsApi } from '@/api';
import type { Flag, FlagCategory } from '@/types';
import LoadingSkeleton from '@/components/LoadingSkeleton';

const PROJECT_ID = 'proj-1';

export default function FlagListPage() {
  const [flags, setFlags] = useState<Flag[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [search, setSearch] = useState('');
  const [categoryFilter, setCategoryFilter] = useState<'all' | FlagCategory>('all');
  const [statusFilter, setStatusFilter] = useState<StatusFilter>('all');

  useEffect(() => {
    async function load() {
      try {
        const params: { category?: string; archived?: boolean } = {};
        if (categoryFilter !== 'all') params.category = categoryFilter;
        if (statusFilter === 'archived') params.archived = true;
        const res = await flagsApi.list(PROJECT_ID, params);
        setFlags(res.flags ?? []);
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to load flags');
      } finally {
        setLoading(false);
      }
    }
    load();
  }, [categoryFilter, statusFilter]);
```

- Keep the existing filter bar JSX and table JSX unchanged.
- Apply `search` filter client-side on the fetched `flags` array (same logic as before).
- Apply `statusFilter` for enabled/disabled client-side (the API only supports `archived` filter).
- Show `<LoadingSkeleton />` when `loading` is true.
- Show an error message card when `error` is set.

**Commit message:** `feat(web): connect FlagListPage to GET /api/v1/flags`

---

### Task 8: Connect FlagCreatePage to API

**Files:**
- Modify: `web/src/pages/FlagCreatePage.tsx`

- [ ] **Step 1: Wire form submission to POST /api/v1/flags**

Replace the `console.log` in `handleSubmit` with an actual API call:

```typescript
import { useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { flagsApi } from '@/api';
import type { FlagType, FlagCategory } from '@/types';

// Inside the component:
const navigate = useNavigate();
const [submitting, setSubmitting] = useState(false);
const [error, setError] = useState('');

const handleSubmit = async (e: React.FormEvent) => {
  e.preventDefault();
  setError('');
  setSubmitting(true);
  try {
    const flag = await flagsApi.create({
      project_id: 'proj-1',
      environment_id: 'env-prod',
      key: form.key,
      name: form.name,
      description: form.description || undefined,
      flag_type: form.flag_type,
      category: form.category,
      purpose: form.purpose || undefined,
      owners: form.owners.split(',').map((s) => s.trim()).filter(Boolean),
      is_permanent: form.is_permanent,
      expires_at: form.is_permanent ? undefined : form.expires_at || undefined,
      default_value: form.default_value,
    });
    navigate(`/flags/${flag.id}`);
  } catch (err) {
    setError(err instanceof Error ? err.message : 'Failed to create flag');
  } finally {
    setSubmitting(false);
  }
};
```

- Display `error` in an alert above the form.
- Disable the submit button while `submitting` is true.
- On success, navigate to the new flag's detail page.

**Commit message:** `feat(web): wire FlagCreatePage form to POST /api/v1/flags`

---

### Task 9: Connect FlagDetailPage to API

**Files:**
- Modify: `web/src/pages/FlagDetailPage.tsx`

- [ ] **Step 1: Replace mock data with API calls**

Remove `MOCK_FLAG` and `MOCK_RULES`. Fetch the flag and its rules on mount using the `id` param:

```typescript
import { useState, useEffect } from 'react';
import { useParams, Link } from 'react-router-dom';
import { flagsApi } from '@/api';
import type { Flag, TargetingRule } from '@/types';
import LoadingSkeleton from '@/components/LoadingSkeleton';

export default function FlagDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [flag, setFlag] = useState<Flag | null>(null);
  const [rules, setRules] = useState<TargetingRule[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!id) return;
    async function load() {
      try {
        const [flagData, rulesData] = await Promise.all([
          flagsApi.get(id!),
          flagsApi.listRules?.(id!) ?? Promise.resolve({ rules: [] }),
        ]);
        setFlag(flagData);
        // Rules may come as part of flag detail or separate endpoint
        // The api.ts does not currently have a listRules method.
        // Rules are fetched via the targeting_rules included in the flag response
        // or via a separate call if needed.
        setRules(rulesData.rules ?? []);
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to load flag');
      } finally {
        setLoading(false);
      }
    }
    load();
  }, [id]);
```

Note: The current `api.ts` does not have a `listRules` method. Add one to `flagsApi`:

```typescript
// Add to flagsApi in api.ts:
listRules: (flagId: string) =>
  request<{ rules: TargetingRule[] }>(`/flags/${flagId}/rules`),
```

- [ ] **Step 2: Wire toggle and archive actions to API**

Replace the local-only `handleToggle` and `handleArchive` with real API calls:

```typescript
const handleToggle = async () => {
  if (!flag) return;
  try {
    const res = await flagsApi.toggle(flag.id, !flag.enabled);
    setFlag((prev) => prev ? { ...prev, enabled: res.enabled } : prev);
  } catch (err) {
    // Show error via toast
  }
};

const handleArchive = async () => {
  if (!flag) return;
  try {
    await flagsApi.archive(flag.id);
    setFlag((prev) => prev ? { ...prev, archived: true } : prev);
  } catch (err) {
    // Show error via toast
  }
};
```

- Show `<LoadingSkeleton />` when loading.
- Show error card when error is set.
- Keep all existing JSX structure.

**Commit message:** `feat(web): connect FlagDetailPage to flag and rules API endpoints`

---

### Task 10: Connect DeploymentsPage to API

**Files:**
- Modify: `web/src/pages/DeploymentsPage.tsx`

- [ ] **Step 1: Replace mock data with API call**

Remove the entire `MOCK_DEPLOYMENTS` array. Fetch from the API on mount:

```typescript
const PROJECT_ID = 'proj-1';

const [deployments, setDeployments] = useState<Deployment[]>([]);
const [loading, setLoading] = useState(true);
const [error, setError] = useState<string | null>(null);

useEffect(() => {
  async function load() {
    try {
      const res = await deploymentsApi.list(PROJECT_ID);
      setDeployments(res.deployments ?? []);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load deployments');
    } finally {
      setLoading(false);
    }
  }
  load();
}, []);
```

- Keep the existing filter logic but apply it to `deployments` state instead of `MOCK_DEPLOYMENTS`.
- Compute stat card values from real data (active count, completed today, rollbacks in 24h).
- Show `<LoadingSkeleton />` when loading.
- Keep all existing helper functions (`strategyBadgeClass`, `statusBadgeClass`, etc.) and JSX structure.

**Commit message:** `feat(web): connect DeploymentsPage to GET /api/v1/deployments`

---

### Task 11: Connect ReleasesPage and SettingsPage to API

**Files:**
- Modify: `web/src/pages/ReleasesPage.tsx`
- Modify: `web/src/pages/SettingsPage.tsx`

- [ ] **Step 1: Connect ReleasesPage**

Remove `MOCK_RELEASES`. Fetch from API:

```typescript
const PROJECT_ID = 'proj-1';

const [releases, setReleases] = useState<Release[]>([]);
const [loading, setLoading] = useState(true);
const [error, setError] = useState<string | null>(null);

useEffect(() => {
  async function load() {
    try {
      const res = await releasesApi.list(PROJECT_ID);
      setReleases(res.releases ?? []);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load releases');
    } finally {
      setLoading(false);
    }
  }
  load();
}, []);
```

- Apply the existing tab filter to `releases` state instead of `MOCK_RELEASES`.
- Show `<LoadingSkeleton />` when loading.

- [ ] **Step 2: Connect SettingsPage API Keys tab**

Remove `MOCK_API_KEYS`. Fetch API keys on mount and wire create/revoke actions:

```typescript
import { apiKeysApi } from '@/api';
import type { ApiKey } from '@/types';

const [apiKeys, setApiKeys] = useState<ApiKey[]>([]);
const [keysLoading, setKeysLoading] = useState(true);

useEffect(() => {
  async function loadKeys() {
    try {
      const res = await apiKeysApi.list();
      setApiKeys(res.api_keys ?? []);
    } catch (err) {
      console.error('Failed to load API keys:', err);
    } finally {
      setKeysLoading(false);
    }
  }
  loadKeys();
}, []);

const handleCreateKey = async () => {
  // Show a prompt/modal for name and scopes (simplified: prompt for name)
  const name = window.prompt('API key name:');
  if (!name) return;
  try {
    const res = await apiKeysApi.create({ name, scopes: ['flags:read'] });
    setApiKeys((prev) => [...prev, res.api_key]);
    // Show the token to the user (it's only visible once)
    window.alert(`API key created. Token (copy now, shown once):\n${res.token}`);
  } catch (err) {
    console.error('Failed to create API key:', err);
  }
};

const handleRevokeKey = async (id: string) => {
  if (!window.confirm('Revoke this API key? This cannot be undone.')) return;
  try {
    await apiKeysApi.revoke(id);
    setApiKeys((prev) => prev.filter((k) => k.id !== id));
  } catch (err) {
    console.error('Failed to revoke API key:', err);
  }
};
```

- Wire the "Create API Key" button to `handleCreateKey`.
- Wire each "Revoke" button to `handleRevokeKey(key.id)`.
- The Webhooks and Notifications tabs remain local-only (no backend endpoints exist yet).
- Show `<LoadingSkeleton />` in the API Keys tab when loading.

**Commit message:** `feat(web): connect ReleasesPage and SettingsPage to live API`

---

### Task 12: useSSE Hook for Real-time Flag Updates

**Files:**
- Create: `web/src/hooks/useSSE.ts`
- Modify: `web/src/components/AuthProvider.tsx`

- [ ] **Step 1: Create the useSSE hook**

```typescript
// web/src/hooks/useSSE.ts
import { useEffect, useRef, useCallback } from 'react';

interface SSEOptions {
  url: string;
  token: string | null;
  onMessage: (event: MessageEvent) => void;
  onError?: (event: Event) => void;
  enabled?: boolean;
}

export function useSSE({ url, token, onMessage, onError, enabled = true }: SSEOptions) {
  const sourceRef = useRef<EventSource | null>(null);
  const reconnectAttempt = useRef(0);
  const reconnectTimer = useRef<ReturnType<typeof setTimeout>>();

  const connect = useCallback(() => {
    if (!token || !enabled) return;

    // Close existing connection
    sourceRef.current?.close();

    const separator = url.includes('?') ? '&' : '?';
    const fullUrl = `${url}${separator}token=${encodeURIComponent(token)}`;
    const es = new EventSource(fullUrl);
    sourceRef.current = es;

    es.addEventListener('flag_change', (event) => {
      reconnectAttempt.current = 0; // Reset on successful message
      onMessage(event);
    });

    es.onopen = () => {
      reconnectAttempt.current = 0;
    };

    es.onerror = (event) => {
      onError?.(event);
      es.close();

      // Exponential backoff: 1s, 2s, 4s, 8s, 16s, 30s max
      const base = Math.min(1000 * Math.pow(2, reconnectAttempt.current), 30000);
      // Add +/- 20% jitter
      const jitter = base * (0.8 + Math.random() * 0.4);
      reconnectAttempt.current++;

      reconnectTimer.current = setTimeout(connect, jitter);
    };
  }, [url, token, onMessage, onError, enabled]);

  useEffect(() => {
    connect();
    return () => {
      sourceRef.current?.close();
      if (reconnectTimer.current) clearTimeout(reconnectTimer.current);
    };
  }, [connect]);

  return {
    close: () => {
      sourceRef.current?.close();
      if (reconnectTimer.current) clearTimeout(reconnectTimer.current);
    },
  };
}
```

- [ ] **Step 2: Integrate SSE into AuthProvider**

Add SSE connection management to `AuthProvider`. When authenticated, connect to the flag stream. Export an `onFlagChange` callback that pages can subscribe to:

Add to `AuthProvider.tsx`:

```typescript
import { useSSE } from '@/hooks/useSSE';

// Inside AuthProvider, add:
const [flagChangeCount, setFlagChangeCount] = useState(0);

// This counter increments on every SSE flag_change event.
// Pages that depend on flag data can use this as a dependency to re-fetch.
const handleFlagChange = useCallback((event: MessageEvent) => {
  console.log('SSE flag_change:', event.data);
  setFlagChangeCount((c) => c + 1);
}, []);

useSSE({
  url: '/api/v1/flags/stream?project_id=proj-1',
  token,
  onMessage: handleFlagChange,
  enabled: !!token,
});
```

Add `flagChangeCount` to the context value so pages can react to real-time updates:

```typescript
interface AuthContextValue {
  // ... existing fields ...
  flagChangeCount: number;
}
```

Pages that display flag data (DashboardPage, FlagListPage, FlagDetailPage) should include `flagChangeCount` in their `useEffect` dependency array to re-fetch when a flag changes:

```typescript
const { flagChangeCount } = useAuth();

useEffect(() => {
  // ... fetch flags ...
}, [flagChangeCount]); // Re-fetch when SSE notifies of changes
```

**Commit message:** `feat(web): add useSSE hook for real-time flag updates via SSE`

---

## Verification Checklist

After all tasks are complete, verify:

- [ ] Visiting `/` without a token redirects to `/login`
- [ ] Login form submits to `POST /api/v1/auth/login` and stores token
- [ ] Register form submits to `POST /api/v1/auth/register` and stores token
- [ ] After login, all pages load without mock data
- [ ] Dashboard shows real flag counts, deployment stats, and recent items
- [ ] FlagListPage fetches and filters flags from the API
- [ ] FlagCreatePage creates a real flag and navigates to detail
- [ ] FlagDetailPage loads flag + rules from API, toggle/archive work
- [ ] DeploymentsPage loads real deployments
- [ ] ReleasesPage loads real releases
- [ ] SettingsPage loads, creates, and revokes real API keys
- [ ] SSE connection established after login, flag changes trigger re-fetch
- [ ] 401 responses clear token and redirect to login
- [ ] Loading skeletons display while data is fetching
- [ ] Error states display when API calls fail
- [ ] TypeScript compiles with no errors (`npx tsc --noEmit`)
