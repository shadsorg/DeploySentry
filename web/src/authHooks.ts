import { useContext } from 'react';
import { AuthContext } from './authContext';
import { type AuthContextValue } from './authTypes';

/**
 * Hook to access the AuthContext value.
 */
export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error('useAuth must be used within AuthProvider');
  return ctx;
}
