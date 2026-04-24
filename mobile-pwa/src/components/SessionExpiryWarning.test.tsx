import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { SessionExpiryWarning } from './SessionExpiryWarning';
import { AuthContext } from '../authContext';
import type { AuthContextValue } from '../authTypes';

function Harness({ ctx }: { ctx: Partial<AuthContextValue> }) {
  const value: AuthContextValue = {
    user: { id: '1', email: 'a@b.c', name: 'A' },
    loading: false,
    login: async () => {},
    logout: () => {},
    expiresAt: null,
    expiryWarningOpen: false,
    extendSession: async () => {},
    ...ctx,
  };
  return (
    <AuthContext.Provider value={value}>
      <SessionExpiryWarning />
    </AuthContext.Provider>
  );
}

describe('SessionExpiryWarning', () => {
  it('renders nothing when closed', () => {
    const { container } = render(<Harness ctx={{ expiryWarningOpen: false }} />);
    expect(container).toBeEmptyDOMElement();
  });

  it('renders a warning and calls extendSession on button click', async () => {
    const extend = vi.fn().mockResolvedValue(undefined);
    render(<Harness ctx={{ expiryWarningOpen: true, expiresAt: Date.now() + 30_000, extendSession: extend }} />);
    expect(screen.getByText(/signing you out/i)).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: /stay signed in/i }));
    expect(extend).toHaveBeenCalled();
  });

  it('calls logout when Sign out is clicked', async () => {
    const logout = vi.fn();
    render(<Harness ctx={{ expiryWarningOpen: true, logout }} />);
    await userEvent.click(screen.getByRole('button', { name: /sign out/i }));
    expect(logout).toHaveBeenCalled();
  });
});
