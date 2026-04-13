// Context & Provider
export { DeploySentryContext } from './context';
export { DeploySentryProvider } from './provider';

// Hooks
export {
  useDeploySentry,
  useDispatch,
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
  EvaluationContext,
  Flag,
  FlagCategory,
  FlagDetail,
  FlagMetadata,
  ProviderProps,
  Registration,
  SSEFlagUpdate,
  UserContext,
} from './types';
