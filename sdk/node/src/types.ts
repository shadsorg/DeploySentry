/**
 * DeploySentry SDK type definitions.
 */

/** Categories that classify the intent and lifecycle of a feature flag. */
export type FlagCategory = 'release' | 'feature' | 'experiment' | 'ops' | 'permission';

/** Rich metadata attached to every feature flag. */
export interface FlagMetadata {
  /** Categorisation that drives lifecycle policies. */
  category: FlagCategory;
  /** Human-readable explanation of what this flag controls. */
  purpose: string;
  /** Team or individual owners responsible for this flag. */
  owners: string[];
  /** When true the flag is not expected to be removed. */
  isPermanent: boolean;
  /** ISO-8601 expiration timestamp. Undefined for permanent flags. */
  expiresAt?: string;
  /** Free-form tags for filtering and grouping. */
  tags: string[];
}

/** A feature flag as returned by the DeploySentry API. */
export interface Flag {
  /** Unique key used to reference the flag in code. */
  key: string;
  /** Whether the flag is currently enabled. */
  enabled: boolean;
  /** The resolved value (boolean, string, number, or JSON object). */
  value: unknown;
  /** Rich metadata describing the flag. */
  metadata: FlagMetadata;
  /** ISO-8601 timestamp of the last update. */
  updatedAt: string;
}

/** Contextual information sent with every evaluation request. */
export interface EvaluationContext {
  /** Identifier of the user being evaluated. */
  userId?: string;
  /** Identifier of the organisation the user belongs to. */
  orgId?: string;
  /** Arbitrary key-value attributes for targeting rules. */
  attributes?: Record<string, string>;
}

/** Full evaluation result including the resolved value and metadata. */
export interface EvaluationResult<T = unknown> {
  /** The flag key that was evaluated. */
  key: string;
  /** The resolved value after evaluation. */
  value: T;
  /** Whether the flag is enabled for the given context. */
  enabled: boolean;
  /** Reason the value was resolved (e.g. "TARGETING_MATCH", "DEFAULT"). */
  reason: string;
  /** Rich metadata for the flag. */
  metadata: FlagMetadata;
  /** ISO-8601 evaluation timestamp. */
  evaluatedAt: string;
}

/** Configuration options for the DeploySentry client. */
export interface ClientOptions {
  /** API key used for authenticating with the DeploySentry service. */
  apiKey: string;
  /** Base URL of the DeploySentry API. Defaults to https://api.dr-sentry.com. */
  baseURL?: string;
  /** Environment identifier (e.g. "production", "staging"). */
  environment: string;
  /** Project identifier. */
  project: string;
  /** Application identifier. */
  application: string;
  /** Cache TTL in milliseconds. Defaults to 60 000 (1 minute). */
  cacheTimeout?: number;
  /** When true, the client returns default values without contacting the API. */
  offlineMode?: boolean;
  /** Optional session identifier for consistent flag evaluation across requests. */
  sessionId?: string;
  /** SDK mode: server (default), file (YAML only), or server-with-fallback. */
  mode?: 'server' | 'file' | 'server-with-fallback';
  /** Path to a local YAML flag config file. Defaults to .deploysentry/flags.yaml. */
  flagFilePath?: string;
  /** Called whenever the flag cache is refreshed from an SSE change event. */
  onFlagChange?: (flags: Flag[]) => void;
}

export interface FlagConfig {
  version: number;
  project: string;
  application: string;
  exported_at: string;
  environments: FlagConfigEnvironment[];
  flags: FlagConfigFlag[];
}
export interface FlagConfigEnvironment {
  id: string;
  name: string;
  is_production: boolean;
}
export interface FlagConfigFlag {
  key: string;
  name: string;
  flag_type: string;
  category: string;
  default_value: string;
  is_permanent: boolean;
  expires_at?: string;
  environments: Record<string, { enabled: boolean; value: string }>;
  rules?: FlagConfigRule[];
}
export interface FlagConfigRule {
  attribute: string;
  operator: string;
  target_values: string[];
  value: string;
  priority: number;
  environments: Record<string, boolean>;
}

/** Internal representation of an API error response. */
export interface ApiError {
  status: number;
  message: string;
  code?: string;
}

export interface Registration<T extends (...args: any[]) => any = (...args: any[]) => any> {
  handler: T;
  flagKey?: string;
}
