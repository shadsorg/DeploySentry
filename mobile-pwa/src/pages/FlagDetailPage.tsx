import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import {
  auditApi,
  envApi,
  flagEnvStateApi,
  flagsApi,
} from '../api';
import type {
  AuditLogEntry,
  Flag,
  FlagEnvironmentState,
  OrgEnvironment,
  RuleEnvironmentState,
  TargetingRule,
} from '../types';
import { CategoryBadge } from '../components/CategoryBadge';
import { ToggleSwitch } from '../components/ToggleSwitch';
import { RuleEditSheet } from '../components/RuleEditSheet';
import { ruleSummary } from '../lib/ruleSummary';

const PAGE_SIZE = 20;

function fmt(iso?: string | null): string {
  if (!iso) return '—';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '—';
  return d.toLocaleString();
}

function describeAction(action: string): string {
  switch (action) {
    case 'flag.created':
      return 'Created flag';
    case 'flag.updated':
      return 'Updated flag settings';
    case 'flag.toggled':
      return 'Toggled flag';
    case 'flag.archived':
      return 'Archived flag';
    case 'flag.env_state.updated':
      return 'Updated environment state';
    case 'flag.rule.created':
      return 'Added targeting rule';
    case 'flag.rule.deleted':
      return 'Deleted targeting rule';
    case 'flag.rule.env_state.updated':
      return 'Updated rule environment state';
    default:
      return action;
  }
}

function buildDesktopUrl(
  orgSlug: string,
  projectSlug: string,
  flagId: string,
  appSlug?: string,
): string {
  const rawBase = (import.meta.env.VITE_WEB_BASE_URL as string | undefined) ?? '/';
  const base = rawBase === '/' || rawBase === '' ? '' : rawBase.replace(/\/+$/, '');
  const path = appSlug
    ? `/orgs/${orgSlug}/projects/${projectSlug}/apps/${appSlug}/flags/${flagId}`
    : `/orgs/${orgSlug}/projects/${projectSlug}/flags/${flagId}`;
  return `${base}${path}`;
}

function ownersDisplay(owners?: string[]): string {
  if (!owners || owners.length === 0) return '—';
  if (owners.length <= 3) return owners.join(', ');
  const head = owners.slice(0, 3).join(', ');
  return `${head} …+${owners.length - 3} more`;
}

interface EnvSectionProps {
  env: OrgEnvironment;
  state?: FlagEnvironmentState;
  flag: Flag;
  desktopUrl: string;
  rules: TargetingRule[];
  ruleEnvStates: RuleEnvironmentState[];
  onCommit: (
    envId: string,
    patch: { enabled?: boolean; value?: unknown },
    prev: FlagEnvironmentState | undefined,
  ) => Promise<void>;
  onCommitRule: (
    ruleId: string,
    envId: string,
    patch: { enabled: boolean },
    prev: RuleEnvironmentState | undefined,
  ) => Promise<void>;
  onEditRule: (rule: TargetingRule) => void;
}

interface RuleRowProps {
  rule: TargetingRule;
  envId: string;
  envName: string;
  desktopUrl: string;
  ruleState: RuleEnvironmentState | undefined;
  onCommitRule: EnvSectionProps['onCommitRule'];
  onEditRule: (rule: TargetingRule) => void;
}

function RuleRow({
  rule,
  envId,
  envName,
  desktopUrl,
  ruleState,
  onCommitRule,
  onEditRule,
}: RuleRowProps) {
  const enabled = ruleState?.enabled === true;
  const [error, setError] = useState<string | null>(null);
  const errorTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    return () => {
      if (errorTimerRef.current) clearTimeout(errorTimerRef.current);
    };
  }, []);

  function flashError(msg: string) {
    setError(msg);
    if (errorTimerRef.current) clearTimeout(errorTimerRef.current);
    errorTimerRef.current = setTimeout(() => setError(null), 4000);
  }

  async function handleToggle(next: boolean) {
    try {
      await onCommitRule(rule.id, envId, { enabled: next }, ruleState);
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'unknown error';
      flashError(`couldn't save: ${msg}`);
    }
  }

  return (
    <li
      className="m-flag-rule-row"
      data-rule-disabled={enabled ? undefined : 'true'}
      style={enabled ? undefined : { opacity: 0.5 }}
    >
      <span className="m-flag-rule-priority">{rule.priority}</span>
      <span className="m-flag-rule-type">{rule.rule_type ?? 'rule'}</span>
      <span className="m-flag-rule-summary">{ruleSummary(rule)}</span>
      <span
        className="m-flag-rule-toggle-wrap"
        onClick={(e) => e.stopPropagation()}
      >
        <ToggleSwitch
          checked={enabled}
          onChange={(next) => void handleToggle(next)}
          ariaLabel={`Toggle rule ${rule.priority} in ${envName}`}
          size="sm"
        />
      </span>
      {rule.rule_type === 'compound' ? (
        <a
          href={desktopUrl}
          target="_blank"
          rel="noopener noreferrer"
          className="m-flag-rule-edit-link"
        >
          Edit on desktop →
        </a>
      ) : (
        <button
          type="button"
          className="m-flag-rule-edit"
          aria-label={`Edit rule ${rule.priority} in ${envName}`}
          onClick={() => onEditRule(rule)}
        >
          Edit
        </button>
      )}
      {error ? (
        <div className="m-flag-rule-error" role="alert">
          {error}
        </div>
      ) : null}
    </li>
  );
}

function EnvSection({
  env,
  state,
  flag,
  desktopUrl,
  rules,
  ruleEnvStates,
  onCommit,
  onCommitRule,
  onEditRule,
}: EnvSectionProps) {
  const [expanded, setExpanded] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const errorTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const enabled = state?.enabled === true;

  const rawValue = state?.value;
  const effectiveValue =
    rawValue !== undefined && rawValue !== null ? String(rawValue) : flag.default_value;

  const [valueDraft, setValueDraft] = useState(effectiveValue);
  // Keep valueDraft in sync with effectiveValue when state changes externally.
  useEffect(() => {
    setValueDraft(effectiveValue);
  }, [effectiveValue]);

  useEffect(() => {
    return () => {
      if (errorTimerRef.current) clearTimeout(errorTimerRef.current);
    };
  }, []);

  const flashError = useCallback((msg: string) => {
    setError(msg);
    if (errorTimerRef.current) clearTimeout(errorTimerRef.current);
    errorTimerRef.current = setTimeout(() => setError(null), 4000);
  }, []);

  const sortedRules = useMemo(() => {
    return [...rules].sort((a, b) => a.priority - b.priority);
  }, [rules]);

  async function commit(patch: { enabled?: boolean; value?: unknown }) {
    try {
      await onCommit(env.id, patch, state);
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'unknown error';
      flashError(`couldn't save: ${msg}`);
    }
  }

  function handleToggle(next: boolean) {
    void commit({ enabled: next });
  }

  function commitValueIfChanged(next: string) {
    if (next === effectiveValue) return;
    void commit({ value: next });
  }

  return (
    <div className="m-flag-env-section">
      <div className="m-flag-env-section-head-row">
        <button
          type="button"
          className="m-flag-env-section-head"
          aria-expanded={expanded}
          onClick={() => setExpanded((v) => !v)}
        >
          <span className="m-flag-env-section-caret" aria-hidden>
            {expanded ? '▾' : '▸'}
          </span>
          <span className="m-flag-env-section-name">{env.name}</span>
          {env.is_production ? (
            <span className="m-flag-env-prod-badge">production</span>
          ) : null}
        </button>
        <span
          className="m-flag-env-toggle-wrap"
          onClick={(e) => e.stopPropagation()}
        >
          <ToggleSwitch
            checked={enabled}
            onChange={handleToggle}
            ariaLabel={`Toggle ${env.name}`}
            size="sm"
          />
        </span>
      </div>

      {error ? (
        <div className="m-flag-env-error" role="alert">
          {error}
        </div>
      ) : null}

      {expanded ? (
        <div className="m-flag-env-section-body">
          <div className="m-flag-env-default">
            <label className="m-muted" htmlFor={`env-default-${env.id}`}>
              Default value:
            </label>{' '}
            {flag.flag_type === 'boolean' ? (
              <select
                id={`env-default-${env.id}`}
                className="m-flag-env-default-select"
                value={effectiveValue === 'true' ? 'true' : 'false'}
                onChange={(e) => commitValueIfChanged(e.target.value)}
              >
                <option value="true">true</option>
                <option value="false">false</option>
              </select>
            ) : flag.flag_type === 'json' ? (
              <span>
                <span
                  style={{ fontFamily: 'var(--font-mono, monospace)' }}
                  className="m-flag-env-json-preview"
                >
                  {effectiveValue}
                </span>{' '}
                <a
                  href={desktopUrl}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="m-flag-env-json-link"
                >
                  Edit JSON on desktop →
                </a>
              </span>
            ) : (
              <input
                id={`env-default-${env.id}`}
                type="text"
                className="m-flag-env-default-input"
                inputMode={flag.flag_type === 'number' ? 'numeric' : 'text'}
                enterKeyHint="done"
                value={valueDraft}
                onChange={(e) => setValueDraft(e.target.value)}
                onBlur={() => commitValueIfChanged(valueDraft)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') {
                    e.preventDefault();
                    (e.target as HTMLInputElement).blur();
                  }
                }}
              />
            )}
          </div>

          {sortedRules.length === 0 ? (
            <p className="m-muted" style={{ fontSize: 12, margin: '8px 0' }}>
              No rules configured.
            </p>
          ) : (
            <ol className="m-flag-rule-list">
              {sortedRules.map((rule) => {
                const rs = ruleEnvStates.find(
                  (s) => s.rule_id === rule.id && s.environment_id === env.id,
                );
                return (
                  <RuleRow
                    key={rule.id}
                    rule={rule}
                    envId={env.id}
                    envName={env.name}
                    desktopUrl={desktopUrl}
                    ruleState={rs}
                    onCommitRule={onCommitRule}
                    onEditRule={onEditRule}
                  />
                );
              })}
            </ol>
          )}
        </div>
      ) : null}
    </div>
  );
}

interface RuleOrderPanelProps {
  rules: TargetingRule[];
  expanded: boolean;
  onToggleExpanded: () => void;
  error: string | null;
  onSwap: (idxA: number, idxB: number) => void;
}

function RuleOrderPanel({
  rules,
  expanded,
  onToggleExpanded,
  error,
  onSwap,
}: RuleOrderPanelProps) {
  const ordered = useMemo(
    () => [...rules].sort((a, b) => a.priority - b.priority),
    [rules],
  );
  const count = ordered.length;

  return (
    <div className="m-rule-order-panel" style={{ marginTop: 16 }}>
      <button
        type="button"
        className="m-rule-order-head"
        aria-expanded={expanded}
        onClick={onToggleExpanded}
      >
        <span className="m-rule-order-caret" aria-hidden>
          {expanded ? '▾' : '▸'}
        </span>
        <span className="m-rule-order-title">Rule order</span>
        <span className="m-rule-order-count">
          {count} {count === 1 ? 'rule' : 'rules'}
        </span>
      </button>
      {error ? (
        <div className="m-rule-order-error" role="alert">
          couldn't reorder: {error}
        </div>
      ) : null}
      {expanded ? (
        ordered.length === 0 ? (
          <p className="m-muted" style={{ fontSize: 12, margin: '8px 0' }}>
            No rules configured.
          </p>
        ) : (
          <ol className="m-rule-order-list">
            {ordered.map((rule, idx) => {
              const isFirst = idx === 0;
              const isLast = idx === ordered.length - 1;
              return (
                <li key={rule.id} className="m-rule-order-row">
                  <button
                    type="button"
                    className="m-rule-order-arrow"
                    aria-label={`Move ${rule.id} up`}
                    disabled={isFirst}
                    onClick={() => onSwap(idx, idx - 1)}
                  >
                    ↑
                  </button>
                  <button
                    type="button"
                    className="m-rule-order-arrow"
                    aria-label={`Move ${rule.id} down`}
                    disabled={isLast}
                    onClick={() => onSwap(idx, idx + 1)}
                  >
                    ↓
                  </button>
                  <span className="m-rule-order-priority">{rule.priority}</span>
                  <span className="m-rule-order-summary">{ruleSummary(rule)}</span>
                </li>
              );
            })}
          </ol>
        )
      ) : null}
    </div>
  );
}

export function FlagDetailPage() {
  const { orgSlug, projectSlug, appSlug, flagId } = useParams<{
    orgSlug: string;
    projectSlug: string;
    appSlug?: string;
    flagId: string;
  }>();

  const [flag, setFlag] = useState<Flag | null>(null);
  const [rules, setRules] = useState<TargetingRule[]>([]);
  const [ruleEnvStates, setRuleEnvStates] = useState<RuleEnvironmentState[]>([]);
  const [envStates, setEnvStates] = useState<FlagEnvironmentState[]>([]);
  const [environments, setEnvironments] = useState<OrgEnvironment[]>([]);
  const [error, setError] = useState<string | null>(null);

  // History pagination
  const [history, setHistory] = useState<AuditLogEntry[]>([]);
  const [offset, setOffset] = useState(0);
  const [hasMore, setHasMore] = useState(true);
  const [historyLoading, setHistoryLoading] = useState(false);

  // Rule edit sheet — track the rule currently being edited (null = closed).
  const [editingRule, setEditingRule] = useState<TargetingRule | null>(null);

  // Rule order panel — collapsed by default; surfaces a swap error inline.
  const [rulePanelExpanded, setRulePanelExpanded] = useState(false);
  const [reorderError, setReorderError] = useState<string | null>(null);

  // Fan-in fetch for the 5 detail endpoints.
  useEffect(() => {
    if (!flagId || !orgSlug) return;
    let cancelled = false;
    setError(null);
    setFlag(null);

    Promise.all([
      flagsApi.get(flagId),
      flagsApi.listRules(flagId),
      flagsApi.listRuleEnvStates(flagId),
      flagEnvStateApi.list(flagId),
      envApi.listOrg(orgSlug),
    ])
      .then(([flagRes, rulesRes, ruleStatesRes, envStatesRes, envRes]) => {
        if (cancelled) return;
        setFlag(flagRes);
        setRules(rulesRes.rules);
        setRuleEnvStates(ruleStatesRes.rule_environment_states);
        setEnvStates(envStatesRes.environment_states);
        setEnvironments(envRes.environments);
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setError(err instanceof Error ? err.message : 'Failed to load flag');
      });

    return () => {
      cancelled = true;
    };
  }, [flagId, orgSlug]);

  // Audit history fetch — depends on flagId and offset. Append on offset > 0.
  useEffect(() => {
    if (!flagId) return;
    let cancelled = false;
    setHistoryLoading(true);

    auditApi
      .listForFlag(flagId, { limit: PAGE_SIZE, offset })
      .then((res) => {
        if (cancelled) return;
        const batch = res.entries ?? [];
        setHistory((prev) => (offset === 0 ? batch : [...prev, ...batch]));
        setHasMore(batch.length === PAGE_SIZE);
      })
      .catch(() => {
        if (cancelled) return;
        setHasMore(false);
      })
      .finally(() => {
        if (!cancelled) setHistoryLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [flagId, offset]);

  // Optimistic env-state commit. Apply locally, fire PUT, sync with response on
  // success or revert on failure. Throws on failure so the env section can
  // surface an inline error.
  const commitEnvState = useCallback(
    async (
      envId: string,
      patch: { enabled?: boolean; value?: unknown },
      prev: FlagEnvironmentState | undefined,
    ): Promise<void> => {
      if (!flagId) return;
      // Apply optimistically.
      setEnvStates((prevStates) => {
        const idx = prevStates.findIndex((s) => s.environment_id === envId);
        if (idx === -1) {
          const stub: FlagEnvironmentState = {
            flag_id: flagId,
            environment_id: envId,
            enabled: patch.enabled ?? false,
            value: patch.value,
          };
          return [...prevStates, stub];
        }
        const next = { ...prevStates[idx], ...patch };
        const copy = prevStates.slice();
        copy[idx] = next;
        return copy;
      });

      try {
        const res = await flagEnvStateApi.set(flagId, envId, patch);
        // Sync local state with server response.
        setEnvStates((prevStates) => {
          const idx = prevStates.findIndex((s) => s.environment_id === envId);
          if (idx === -1) return [...prevStates, res];
          const copy = prevStates.slice();
          copy[idx] = res;
          return copy;
        });
      } catch (err) {
        // Revert.
        setEnvStates((prevStates) => {
          const idx = prevStates.findIndex((s) => s.environment_id === envId);
          if (prev === undefined) {
            // Was a stub; remove it.
            if (idx === -1) return prevStates;
            const copy = prevStates.slice();
            copy.splice(idx, 1);
            return copy;
          }
          if (idx === -1) return [...prevStates, prev];
          const copy = prevStates.slice();
          copy[idx] = prev;
          return copy;
        });
        throw err;
      }
    },
    [flagId],
  );

  // Optimistic per-rule per-env state commit. Apply locally, fire PUT, sync
  // with response on success or revert on failure. Throws on failure so the
  // rule row can surface an inline error.
  const commitRuleEnvState = useCallback(
    async (
      ruleId: string,
      envId: string,
      patch: { enabled: boolean },
      prev: RuleEnvironmentState | undefined,
    ): Promise<void> => {
      if (!flagId) return;
      // Apply optimistically.
      setRuleEnvStates((prevStates) => {
        const idx = prevStates.findIndex(
          (s) => s.rule_id === ruleId && s.environment_id === envId,
        );
        if (idx === -1) {
          const stub: RuleEnvironmentState = {
            rule_id: ruleId,
            environment_id: envId,
            enabled: patch.enabled,
          };
          return [...prevStates, stub];
        }
        const next = { ...prevStates[idx], ...patch };
        const copy = prevStates.slice();
        copy[idx] = next;
        return copy;
      });

      try {
        const res = await flagsApi.setRuleEnvState(flagId, ruleId, envId, patch);
        // Sync local state with server response.
        setRuleEnvStates((prevStates) => {
          const idx = prevStates.findIndex(
            (s) => s.rule_id === ruleId && s.environment_id === envId,
          );
          if (idx === -1) return [...prevStates, res];
          const copy = prevStates.slice();
          copy[idx] = res;
          return copy;
        });
      } catch (err) {
        // Revert.
        setRuleEnvStates((prevStates) => {
          const idx = prevStates.findIndex(
            (s) => s.rule_id === ruleId && s.environment_id === envId,
          );
          if (prev === undefined) {
            if (idx === -1) return prevStates;
            const copy = prevStates.slice();
            copy.splice(idx, 1);
            return copy;
          }
          if (idx === -1) return [...prevStates, prev];
          const copy = prevStates.slice();
          copy[idx] = prev;
          return copy;
        });
        throw err;
      }
    },
    [flagId],
  );

  // Handle a rule replacement coming back from the RuleEditSheet save.
  const handleRuleSaved = useCallback((updated: TargetingRule) => {
    setRules((prev) => prev.map((r) => (r.id === updated.id ? updated : r)));
  }, []);

  // Swap two adjacent rules' priorities. Optimistically swap locally, then
  // fire two PUTs in parallel; on any failure, revert both.
  const swapRulePriorities = useCallback(
    async (idxA: number, idxB: number): Promise<void> => {
      if (!flagId) return;
      const ordered = [...rules].sort((a, b) => a.priority - b.priority);
      const ruleA = ordered[idxA];
      const ruleB = ordered[idxB];
      if (!ruleA || !ruleB) return;
      const priA = ruleA.priority;
      const priB = ruleB.priority;

      // Optimistic swap.
      setRules((prev) =>
        prev.map((r) => {
          if (r.id === ruleA.id) return { ...r, priority: priB };
          if (r.id === ruleB.id) return { ...r, priority: priA };
          return r;
        }),
      );
      setReorderError(null);

      try {
        await Promise.all([
          flagsApi.updateRule(flagId, ruleA.id, { priority: priB }),
          flagsApi.updateRule(flagId, ruleB.id, { priority: priA }),
        ]);
      } catch (err) {
        // Revert both rules to original priorities.
        setRules((prev) =>
          prev.map((r) => {
            if (r.id === ruleA.id) return { ...r, priority: priA };
            if (r.id === ruleB.id) return { ...r, priority: priB };
            return r;
          }),
        );
        setReorderError(err instanceof Error ? err.message : 'Failed to reorder');
      }
    },
    [flagId, rules],
  );

  const sortedEnvironments = useMemo(() => {
    return [...environments].sort((a, b) => {
      const ao = a.sort_order ?? 0;
      const bo = b.sort_order ?? 0;
      if (ao !== bo) return ao - bo;
      return a.name.localeCompare(b.name);
    });
  }, [environments]);

  const backHref = appSlug
    ? `/m/orgs/${orgSlug}/flags/${projectSlug}/apps/${appSlug}`
    : `/m/orgs/${orgSlug}/flags/${projectSlug}`;

  if (error) {
    return (
      <section style={{ padding: 20 }}>
        <Link to={backHref} className="m-back-link">
          ‹ Back
        </Link>
        <p style={{ color: 'var(--color-danger, #ef4444)' }}>{error}</p>
      </section>
    );
  }

  if (!flag) {
    return (
      <section style={{ padding: 20 }}>
        <p>Loading flag…</p>
      </section>
    );
  }

  const desktopUrl = buildDesktopUrl(
    orgSlug ?? '',
    projectSlug ?? '',
    flagId ?? '',
    appSlug,
  );

  const expiresLabel = flag.is_permanent ? 'Permanent' : fmt(flag.expires_at);

  return (
    <section style={{ padding: 20 }}>
      <Link to={backHref} className="m-back-link">
        ‹ Back to flags
      </Link>

      <header style={{ marginBottom: 16 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
          <h2
            style={{
              margin: 0,
              fontFamily: 'var(--font-mono, monospace)',
              fontSize: 18,
              wordBreak: 'break-all',
            }}
          >
            {flag.key}
          </h2>
          <CategoryBadge category={flag.category} />
        </div>
        <p style={{ margin: '4px 0 0', color: 'var(--color-text-secondary, #94a3b8)' }}>
          {flag.name}
        </p>
      </header>

      <div className="m-card" style={{ marginBottom: 12 }}>
        <div className="m-list-row">
          <span className="m-muted">Owners</span>
          <span style={{ textAlign: 'right' }}>{ownersDisplay(flag.owners)}</span>
        </div>
        <div className="m-list-row">
          <span className="m-muted">Default value</span>
          <span style={{ fontFamily: 'var(--font-mono, monospace)' }}>{flag.default_value}</span>
        </div>
        <div className="m-list-row">
          <span className="m-muted">Expires</span>
          <span>{expiresLabel}</span>
        </div>
        <div className="m-list-row">
          <span className="m-muted">Edit on desktop</span>
          <a
            href={desktopUrl}
            target="_blank"
            rel="noopener noreferrer"
            style={{ color: 'var(--color-primary, #6366f1)', textDecoration: 'none' }}
          >
            Open →
          </a>
        </div>
      </div>

      <RuleOrderPanel
        rules={rules}
        expanded={rulePanelExpanded}
        onToggleExpanded={() => setRulePanelExpanded((v) => !v)}
        error={reorderError}
        onSwap={(idxA, idxB) => void swapRulePriorities(idxA, idxB)}
      />

      <h3 style={{ fontSize: 14, margin: '20px 0 8px', textTransform: 'uppercase', letterSpacing: '0.05em', color: 'var(--color-text-muted, #64748b)' }}>
        Environments
      </h3>
      {sortedEnvironments.length === 0 ? (
        <p className="m-muted">No environments configured.</p>
      ) : (
        <div className="m-flag-env-sections">
          {sortedEnvironments.map((env) => {
            const state = envStates.find((s) => s.environment_id === env.id);
            return (
              <EnvSection
                key={env.id}
                env={env}
                state={state}
                flag={flag}
                desktopUrl={desktopUrl}
                rules={rules}
                ruleEnvStates={ruleEnvStates}
                onCommit={commitEnvState}
                onCommitRule={commitRuleEnvState}
                onEditRule={(r) => setEditingRule(r)}
              />
            );
          })}
        </div>
      )}

      <h3 style={{ fontSize: 14, margin: '24px 0 8px', textTransform: 'uppercase', letterSpacing: '0.05em', color: 'var(--color-text-muted, #64748b)' }}>
        History
      </h3>
      {history.length === 0 && !historyLoading ? (
        <p className="m-muted">No history yet.</p>
      ) : (
        <ul className="m-flag-history-list">
          {history.map((entry) => (
            <li key={entry.id} className="m-flag-history-row">
              <span className="m-flag-history-time">{fmt(entry.created_at)}</span>
              <span className="m-flag-history-actor">{entry.actor_name || 'System'}</span>
              <span className="m-flag-history-action">{describeAction(entry.action)}</span>
            </li>
          ))}
        </ul>
      )}
      {hasMore && history.length > 0 ? (
        <button
          type="button"
          className="m-button"
          style={{ width: '100%', marginTop: 8 }}
          onClick={() => setOffset((o) => o + PAGE_SIZE)}
          disabled={historyLoading}
        >
          {historyLoading ? 'Loading…' : 'Load more'}
        </button>
      ) : null}

      {editingRule ? (
        <RuleEditSheet
          rule={editingRule}
          flagId={flag.id}
          open
          onClose={() => setEditingRule(null)}
          onSaved={(updated) => {
            handleRuleSaved(updated);
            setEditingRule(null);
          }}
        />
      ) : null}
    </section>
  );
}
