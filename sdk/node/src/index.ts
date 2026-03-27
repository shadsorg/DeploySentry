/**
 * @deploysentry/sdk – Official Node.js/TypeScript SDK for DeploySentry.
 *
 * @example
 * ```ts
 * import { DeploySentryClient } from '@deploysentry/sdk';
 *
 * const client = new DeploySentryClient({
 *   apiKey: 'ds_live_xxx',
 *   environment: 'production',
 *   project: 'my-app',
 * });
 *
 * await client.initialize();
 *
 * const enabled = await client.boolValue('new-checkout', false, {
 *   userId: 'user-42',
 * });
 * ```
 *
 * @packageDocumentation
 */

export { DeploySentryClient } from './client';
export { FlagCache } from './cache';
export { FlagStreamClient } from './streaming';

export type {
  ClientOptions,
  EvaluationContext,
  EvaluationResult,
  Flag,
  FlagCategory,
  FlagMetadata,
  ApiError,
} from './types';
