import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { render, screen, waitFor, act } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import { AuthProvider, RequireAuth, RedirectIfAuth } from './auth';
import { useAuth } from './authHooks';
import { setFetch } from './api';

function Protected() {
  return <div>protected</div>;
}
function LoginScreen() {
  return <div>login</div>;
}
function Status() {
  const { user } = useAuth();
  return <div>user:{user?.email ?? 'none'}</div>;
}

function makeJwt(expSec: number): string {
  const toB64u = (s: string) =>
    btoa(s).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
  return `${toB64u('{"alg":"HS256"}')}.${toB64u(JSON.stringify({ exp: expSec }))}.sig`;
}

describe('AuthProvider', () => {
  let fetchMock: ReturnType<typeof vi.fn<typeof fetch>>;
  beforeEach(() => {
    fetchMock = vi.fn<typeof fetch>();
    setFetch(fetchMock);
    localStorage.clear();
    vi.useFakeTimers({ shouldAdvanceTime: true });
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it('renders loading then redirects unauthenticated to /login', async () => {
    render(
      <MemoryRouter initialEntries={['/status']}>
        <AuthProvider>
          <Routes>
            <Route path="/login" element={<LoginScreen />} />
            <Route element={<RequireAuth />}>
              <Route path="/status" element={<Protected />} />
            </Route>
          </Routes>
        </AuthProvider>
      </MemoryRouter>,
    );
    await waitFor(() => expect(screen.getByText('login')).toBeInTheDocument());
    expect(screen.queryByText('protected')).not.toBeInTheDocument();
  });

  it('restores session from localStorage token on mount', async () => {
    const exp = Math.floor(Date.now() / 1000) + 3600;
    localStorage.setItem('ds_token', makeJwt(exp));
    fetchMock.mockResolvedValue(
      new Response(JSON.stringify({ id: '1', email: 'a@b.c', name: 'A' }), { status: 200 }),
    );
    render(
      <MemoryRouter initialEntries={['/status']}>
        <AuthProvider>
          <Routes>
            <Route element={<RequireAuth />}>
              <Route path="/status" element={<Status />} />
            </Route>
          </Routes>
        </AuthProvider>
      </MemoryRouter>,
    );
    await waitFor(() => expect(screen.getByText('user:a@b.c')).toBeInTheDocument());
  });

  it('RedirectIfAuth pushes authed users away from /login', async () => {
    const exp = Math.floor(Date.now() / 1000) + 3600;
    localStorage.setItem('ds_token', makeJwt(exp));
    fetchMock.mockResolvedValue(
      new Response(JSON.stringify({ id: '1', email: 'a@b.c', name: 'A' }), { status: 200 }),
    );
    render(
      <MemoryRouter initialEntries={['/login']}>
        <AuthProvider>
          <Routes>
            <Route element={<RedirectIfAuth />}>
              <Route path="/login" element={<LoginScreen />} />
            </Route>
            <Route path="/" element={<div>home</div>} />
          </Routes>
        </AuthProvider>
      </MemoryRouter>,
    );
    await waitFor(() => expect(screen.getByText('home')).toBeInTheDocument());
  });

  it('login() stores token and sets user', async () => {
    const exp = Math.floor(Date.now() / 1000) + 3600;
    const token = makeJwt(exp);
    fetchMock.mockImplementation((url) => {
      if (typeof url === 'string' && url.endsWith('/auth/login')) {
        return Promise.resolve(
          new Response(JSON.stringify({ token, user: { id: '1', email: 'a@b.c', name: 'A' } }), {
            status: 200,
          }),
        );
      }
      return Promise.reject(new Error('unexpected fetch: ' + String(url)));
    });
    function LoginForm() {
      const { login, user } = useAuth();
      return (
        <div>
          <button onClick={() => login('a@b.c', 'pw')}>go</button>
          <span>email:{user?.email ?? 'none'}</span>
        </div>
      );
    }
    render(
      <MemoryRouter>
        <AuthProvider>
          <LoginForm />
        </AuthProvider>
      </MemoryRouter>,
    );
    await waitFor(() => expect(screen.getByText('email:none')).toBeInTheDocument());
    await act(async () => {
      screen.getByText('go').click();
    });
    await waitFor(() => expect(screen.getByText('email:a@b.c')).toBeInTheDocument());
    expect(localStorage.getItem('ds_token')).toBe(token);
  });
});
