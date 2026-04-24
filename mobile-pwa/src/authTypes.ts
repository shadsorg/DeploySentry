import type { AuthUser } from './types';

export interface AuthContextValue {
  user: AuthUser | null;
  loading: boolean;
  login: (email: string, password: string) => Promise<void>;
  logout: () => void;
  expiresAt: number | null;
  expiryWarningOpen: boolean;
  extendSession: () => Promise<void>;
}
