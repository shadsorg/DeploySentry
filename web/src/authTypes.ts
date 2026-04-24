import { type AuthUser } from './api';

export interface AuthContextValue {
  user: AuthUser | null;
  loading: boolean;
  login: (email: string, password: string) => Promise<void>;
  register: (email: string, password: string, name: string) => Promise<void>;
  logout: () => void;
  /** Unix ms timestamp when the session JWT expires, or null (API key or unknown). */
  expiresAt: number | null;
  /** Fires the expiry warning when within ~60s of exp; hides on refresh or logout. */
  expiryWarningOpen: boolean;
  /** Re-issues a fresh access token and resets timers. Rejects on failure. */
  extendSession: () => Promise<void>;
}
