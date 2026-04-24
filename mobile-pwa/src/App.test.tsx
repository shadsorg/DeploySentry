import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { AppRoutes } from './App';
import { AuthProvider } from './auth';
import { setFetch } from './api';

describe('AppRoutes', () => {
  let fetchMock: ReturnType<typeof vi.fn<typeof fetch>>;
  beforeEach(() => {
    fetchMock = vi.fn<typeof fetch>();
    setFetch(fetchMock);
    localStorage.clear();
  });

  it('unauthenticated visit to / bounces to /login', async () => {
    render(
      <MemoryRouter initialEntries={['/']}>
        <AuthProvider>
          <AppRoutes />
        </AuthProvider>
      </MemoryRouter>,
    );
    await waitFor(() => expect(screen.getByText(/Sign in$/i)).toBeInTheDocument());
  });

  it('authenticated visit renders inside the layout (TabBar visible on status)', async () => {
    const exp = Math.floor(Date.now() / 1000) + 3600;
    const toB64u = (s: string) => btoa(s).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
    const token = `${toB64u('{"alg":"HS256"}')}.${toB64u(JSON.stringify({ exp }))}.sig`;
    localStorage.setItem('ds_token', token);
    fetchMock.mockImplementation((url) => {
      const s = typeof url === 'string' ? url : String(url);
      if (s.endsWith('/users/me'))
        return Promise.resolve(
          new Response(JSON.stringify({ id: '1', email: 'a@b.c', name: 'A' }), { status: 200 }),
        );
      if (s.endsWith('/orgs'))
        return Promise.resolve(
          new Response(
            JSON.stringify({
              organizations: [{ id: '1', name: 'Acme', slug: 'acme', created_at: '', updated_at: '' }],
            }),
            { status: 200 },
          ),
        );
      return Promise.reject(new Error('unexpected: ' + s));
    });
    render(
      <MemoryRouter initialEntries={['/orgs']}>
        <AuthProvider>
          <AppRoutes />
        </AuthProvider>
      </MemoryRouter>,
    );
    await waitFor(() => expect(screen.getByRole('button', { name: /status/i })).toBeInTheDocument());
  });
});
