import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { LoginPage } from './LoginPage';
import { AuthContext } from '../authContext';
import type { AuthContextValue } from '../authTypes';

function renderWith(ctx: Partial<AuthContextValue>) {
  const value: AuthContextValue = {
    user: null,
    loading: false,
    login: async () => {},
    logout: () => {},
    expiresAt: null,
    expiryWarningOpen: false,
    extendSession: async () => {},
    ...ctx,
  };
  return render(
    <MemoryRouter>
      <AuthContext.Provider value={value}>
        <LoginPage />
      </AuthContext.Provider>
    </MemoryRouter>,
  );
}

describe('LoginPage', () => {
  it('submits email+password to login()', async () => {
    const login = vi.fn().mockResolvedValue(undefined);
    renderWith({ login });
    await userEvent.type(screen.getByLabelText(/email/i), 'a@b.c');
    await userEvent.type(screen.getByLabelText(/password/i), 'hunter2');
    await userEvent.click(screen.getByRole('button', { name: /sign in$/i }));
    expect(login).toHaveBeenCalledWith('a@b.c', 'hunter2');
  });

  it('shows the server error on failed login', async () => {
    const login = vi.fn().mockRejectedValue(new Error('invalid creds'));
    renderWith({ login });
    await userEvent.type(screen.getByLabelText(/email/i), 'a@b.c');
    await userEvent.type(screen.getByLabelText(/password/i), 'wrong');
    await userEvent.click(screen.getByRole('button', { name: /sign in$/i }));
    expect(await screen.findByText(/invalid creds/i)).toBeInTheDocument();
  });

  it('renders OAuth links pointing at /api/v1/auth/oauth/*', () => {
    renderWith({});
    const gh = screen.getByRole('link', { name: /github/i });
    const goog = screen.getByRole('link', { name: /google/i });
    expect(gh).toHaveAttribute('href', '/api/v1/auth/oauth/github');
    expect(goog).toHaveAttribute('href', '/api/v1/auth/oauth/google');
  });
});
