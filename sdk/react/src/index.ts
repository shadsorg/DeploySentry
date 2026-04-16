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

// Local evaluation (file/flagData mode)
export { evaluateLocal } from './local-evaluator';

// Types
export type {
  ApiFlagResponse,
  EvaluationContext,
  Flag,
  FlagCategory,
  FlagConfig,
  FlagConfigEnvironment,
  FlagConfigFlag,
  FlagConfigRule,
  FlagDetail,
  FlagMetadata,
  ProviderProps,
  Registration,
  SSEFlagUpdate,
  UserContext,
} from './types';
