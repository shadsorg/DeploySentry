import { createContext } from 'react';
import type { DeploySentryClient } from './client';

/**
 * React context that holds the DeploySentry client instance.
 *
 * The context value is `null` until the {@link DeploySentryProvider} mounts
 * and initialises the client. Hooks that consume this context will throw
 * if used outside of the provider tree.
 */
export const DeploySentryContext = createContext<DeploySentryClient | null>(null);

DeploySentryContext.displayName = 'DeploySentryContext';
