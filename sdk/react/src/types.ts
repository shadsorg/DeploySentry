/**
 * DeploySentry React SDK type definitions.
 *
 * Types are defined locally so the React SDK has zero runtime dependency on
 * the Node SDK package. They mirror the canonical shapes from @deploysentry/sdk.
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
  /** Display name of the flag. */
  name: string;
  /** Category classification. */
  category: FlagCategory;
  /** Human-readable explanation of what this flag controls. */
  purpose: string;
  /** Team or individual owners responsible for this flag. */
  owners: string[];
  /** When true the flag is not expected to be removed. */
  isPermanent: boolean;
  /** ISO-8601 expiration timestamp. Undefined for permanent flags. */
  expiresAt?: string;
  /** Whether the flag is currently enabled. */
  enabled: boolean;
  /** The resolved value (boolean, string, number, or JSON object). */
  value: unknown;
  /** Free-form tags for filtering and grouping. */
  tags: string[];
}

/** Detailed evaluation result for a single flag. */
export interface FlagDetail<T = unknown> {
  /** The resolved value after evaluation. */
  value: T;
  /** Whether the flag is currently enabled. */
  enabled: boolean;
  /** Rich metadata for the flag. */
  metadata: FlagMetadata;
  /** True while the initial fetch is in progress. */
  loading: boolean;
}

/** User context sent with evaluation requests. */
export interface UserContext {
  /** Unique user identifier. */
  id: string;
  /** Arbitrary attributes for targeting rules. */
  attributes?: Record<string, string>;
}

/** Props accepted by {@link DeploySentryProvider}. */
export interface ProviderProps {
  /** API key for authenticating with the DeploySentry service. */
  apiKey: string;
  /** Base URL of the DeploySentry API. */
  baseURL: string;
  /** Environment identifier (e.g. "production", "staging"). */
  environment: string;
  /** Project identifier. */
  project: string;
  /** Application identifier. */
  application: string;
  /** Optional user context for targeting. */
  user?: UserContext;
  /** Optional session identifier for consistent flag evaluation across requests. */
  sessionId?: string;
  /** SDK mode: 'server' (default), 'file' (local only), or 'server-with-fallback'. */
  mode?: 'server' | 'file' | 'server-with-fallback';
  /** Pre-loaded flag configuration for file/fallback mode. */
  flagData?: FlagConfig;
  /** React children. */
  children: React.ReactNode;
}

/**
 * Internal API response shape for flag evaluation.
 * This represents the raw JSON payload from the server.
 */
export interface ApiFlagResponse {
  key: string;
  name?: string;
  enabled: boolean;
  value: unknown;
  metadata?: {
    category?: FlagCategory;
    purpose?: string;
    owners?: string[];
    isPermanent?: boolean;
    expiresAt?: string;
    tags?: string[];
  };
}

/** SSE event data for real-time flag updates. */
export interface SSEFlagUpdate {
  type: 'flag.updated' | 'flag.created' | 'flag.deleted';
  flag: ApiFlagResponse;
}

/**
 * A registered handler entry used by {@link DeploySentryClient.register} and
 * {@link DeploySentryClient.dispatch}.
 */
export interface Registration<T extends (...args: any[]) => any = (...args: any[]) => any> {
  handler: T;
  flagKey?: string;
}

/** Optional evaluation context passed to {@link DeploySentryClient.dispatch}. */
export interface EvaluationContext {
  userId?: string;
  attributes?: Record<string, string>;
}

/** Pre-loaded flag configuration for file/flagData mode. */
export interface FlagConfig {
  version: number;
  project: string;
  application: string;
  exported_at: string;
  environments: FlagConfigEnvironment[];
  flags: FlagConfigFlag[];
}

/** Environment entry in a flag config file. */
export interface FlagConfigEnvironment {
  id: string;
  name: string;
  is_production: boolean;
}

/** Flag entry in a flag config file. */
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

/** Targeting rule within a flag config file. */
export interface FlagConfigRule {
  attribute: string;
  operator: string;
  target_values: string[];
  value: string;
  priority: number;
  environments: Record<string, boolean>;
}
