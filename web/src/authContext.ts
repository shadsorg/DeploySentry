import { createContext } from 'react';
import { type AuthContextValue } from './authTypes';

export type { AuthContextValue };
export const AuthContext = createContext<AuthContextValue | null>(null);
