// Context & Provider
export { DeploySentryContext } from './context';
export { DeploySentryProvider } from './provider';

// Hooks
export {
  useDeploySentry,
  useExpiredFlags,
  useFlag,
  useFlagDetail,
  useFlagsByCategory,
} from './hooks';

// Client (advanced usage)
export { DeploySentryClient } from './client';

// Types
export type {
  ApiFlagResponse,
  Flag,
  FlagCategory,
  FlagDetail,
  FlagMetadata,
  ProviderProps,
  SSEFlagUpdate,
  UserContext,
} from './types';
