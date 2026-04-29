import { useEffect, useMemo, useState } from 'react';
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

function summarizeRule(rule: TargetingRule): string {
  switch (rule.rule_type) {
    case 'percentage': {
      const n = rule.percentage ?? 0;
      return `${n}% rollout`;
    }
    case 'user_target': {
      const n = rule.user_ids?.length ?? 0;
      return `${n} user IDs`;
    }
    case 'attribute': {
      const combined = `${rule.attribute ?? ''} ${rule.operator ?? ''} ${rule.value ?? ''}`.trim();
      return combined.length > 40 ? `${combined.slice(0, 39)}…` : combined;
    }
    case 'segment':
      return `segment: ${rule.segment_id ?? ''}`;
    case 'schedule':
      return `${rule.start_time ?? ''} – ${rule.end_time ?? ''}`;
    case 'compound':
      return 'compound (edit on desktop)';
    default:
      return rule.value ?? '';
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
  rules: TargetingRule[];
  ruleEnvStates: RuleEnvironmentState[];
}

function EnvSection({ env, state, rules, ruleEnvStates }: EnvSectionProps) {
  const [expanded, setExpanded] = useState(false);
  const enabled = state?.enabled === true;

  const visibleRules = useMemo(() => {
    return rules
      .filter((rule) => {
        const rs = ruleEnvStates.find(
          (s) => s.rule_id === rule.id && s.environment_id === env.id,
        );
        return rs?.enabled === true;
      })
      .sort((a, b) => a.priority - b.priority);
  }, [rules, ruleEnvStates, env.id]);

  const valueDisplay =
    state?.value !== undefined && state?.value !== null
      ? String(state.value)
      : '(uses flag default)';

  return (
    <div className="m-flag-env-section">
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
        <span
          className="m-flag-env-onoff"
          data-on={enabled}
          aria-label={enabled ? 'On' : 'Off'}
        >
          {enabled ? 'On' : 'Off'}
        </span>
      </button>

      {expanded ? (
        <div className="m-flag-env-section-body">
          <div className="m-flag-env-default">
            <span className="m-muted">Default value:</span>{' '}
            <span style={{ fontFamily: 'var(--font-mono, monospace)' }}>{valueDisplay}</span>
          </div>

          {visibleRules.length === 0 ? (
            <p className="m-muted" style={{ fontSize: 12, margin: '8px 0' }}>
              No active rules for this environment.
            </p>
          ) : (
            <ol className="m-flag-rule-list">
              {visibleRules.map((rule) => (
                <li key={rule.id} className="m-flag-rule-row">
                  <span className="m-flag-rule-priority">{rule.priority}</span>
                  <span className="m-flag-rule-type">{rule.rule_type ?? 'rule'}</span>
                  <span className="m-flag-rule-summary">{summarizeRule(rule)}</span>
                </li>
              ))}
            </ol>
          )}
        </div>
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
                rules={rules}
                ruleEnvStates={ruleEnvStates}
              />
            );
          })}
        </div>
      )}

      <p className="m-muted" style={{ fontSize: 12, marginTop: 12 }}>
        Rules are read-only on mobile. Edit on desktop.
      </p>

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
    </section>
  );
}
