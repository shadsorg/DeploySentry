import { useEffect, useMemo, useState } from 'react';
import { Link, useParams, useSearchParams } from 'react-router-dom';
import { appsApi, envApi, flagEnvStateApi, flagsApi, projectsApi } from '../api';
import type {
  Application,
  Flag,
  FlagCategory,
  FlagEnvironmentState,
  OrgEnvironment,
  Project,
} from '../types';
import { CategoryFilterChips } from '../components/CategoryFilterChips';
import { FlagRow } from '../components/FlagRow';
import { StaleBadge } from '../components/StaleBadge';

const ALL_CATEGORIES: FlagCategory[] = [
  'release',
  'feature',
  'experiment',
  'ops',
  'permission',
];

function parseCategoryParam(raw: string | null): FlagCategory[] {
  if (!raw) return [];
  return raw
    .split(',')
    .map((s) => s.trim())
    .filter((s): s is FlagCategory => (ALL_CATEGORIES as string[]).includes(s));
}

export function FlagListPage() {
  const { orgSlug, projectSlug, appSlug } = useParams<{
    orgSlug: string;
    projectSlug: string;
    appSlug?: string;
  }>();
  const [searchParams, setSearchParams] = useSearchParams();

  const [project, setProject] = useState<Project | null>(null);
  const [application, setApplication] = useState<Application | null>(null);
  const [environments, setEnvironments] = useState<OrgEnvironment[]>([]);
  const [flags, setFlags] = useState<Flag[] | null>(null);
  const [stateMap, setStateMap] = useState<Map<string, FlagEnvironmentState[]>>(new Map());
  const [error, setError] = useState<string | null>(null);
  const [lastSuccess, setLastSuccess] = useState<number | null>(null);
  const [refreshing, setRefreshing] = useState(false);

  // Filter state, hydrated from URL on first mount.
  const [query, setQuery] = useState<string>(searchParams.get('q') ?? '');
  const [categories, setCategories] = useState<FlagCategory[]>(
    parseCategoryParam(searchParams.get('category')),
  );

  // Sync filter state -> URL.
  useEffect(() => {
    const next = new URLSearchParams(searchParams);
    if (query) next.set('q', query);
    else next.delete('q');
    if (categories.length > 0) next.set('category', categories.join(','));
    else next.delete('category');
    if (next.toString() !== searchParams.toString()) {
      setSearchParams(next, { replace: true });
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [query, categories]);

  // Resolve project (and optional app) + envs in parallel; then fetch flags + env states.
  useEffect(() => {
    if (!orgSlug || !projectSlug) return;
    let cancelled = false;

    (async () => {
      try {
        setError(null);
        setFlags(null);
        setRefreshing(true);

        const projectListPromise = projectsApi.list(orgSlug);
        const envPromise = envApi.listOrg(orgSlug);
        const appsPromise = appSlug ? appsApi.list(orgSlug, projectSlug) : null;

        const [projRes, envRes, appsRes] = await Promise.all([
          projectListPromise,
          envPromise,
          appsPromise,
        ]);

        const proj = projRes.projects.find((p) => p.slug === projectSlug) ?? null;
        if (!proj) {
          if (!cancelled) {
            setError('Project not found');
            setRefreshing(false);
          }
          return;
        }

        let app: Application | null = null;
        if (appsRes && appSlug) {
          app = appsRes.applications.find((a) => a.slug === appSlug) ?? null;
          if (!app) {
            if (!cancelled) {
              setError('Application not found');
              setRefreshing(false);
            }
            return;
          }
        }

        if (cancelled) return;
        setProject(proj);
        setApplication(app);
        setEnvironments(envRes.environments);

        const flagsRes = await flagsApi.list(proj.id, {
          applicationId: app?.id,
        });
        if (cancelled) return;

        // Single Promise.all for all env-state fetches; commit Map atomically.
        const stateResults = await Promise.all(
          flagsRes.flags.map((f) =>
            flagEnvStateApi.list(f.id).then((r) => [f.id, r.environment_states] as const),
          ),
        );
        if (cancelled) return;

        setStateMap(new Map(stateResults));
        setFlags(flagsRes.flags);
        setLastSuccess(Date.now());
        setRefreshing(false);
      } catch (e) {
        if (!cancelled) {
          setError(e instanceof Error ? e.message : 'Failed to load flags');
          setFlags([]);
          setRefreshing(false);
        }
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [orgSlug, projectSlug, appSlug]);

  const filteredFlags = useMemo(() => {
    if (!flags) return [];
    const q = query.trim().toLowerCase();
    return flags.filter((f) => {
      if (categories.length > 0 && !categories.includes(f.category)) return false;
      if (q) {
        const inKey = f.key.toLowerCase().includes(q);
        const inName = f.name.toLowerCase().includes(q);
        if (!inKey && !inName) return false;
      }
      return true;
    });
  }, [flags, query, categories]);

  const heading = application
    ? `${project?.name ?? projectSlug} / ${application.name}`
    : project?.name ?? projectSlug ?? 'Flags';

  if (error) {
    return (
      <section style={{ padding: 20 }}>
        <p style={{ color: 'var(--color-danger, #ef4444)' }}>{error}</p>
      </section>
    );
  }

  if (flags === null) {
    return (
      <section style={{ padding: 20 }}>
        <p>Loading flags…</p>
      </section>
    );
  }

  const detailHref = (flagId: string) =>
    appSlug
      ? `/m/orgs/${orgSlug}/flags/${projectSlug}/apps/${appSlug}/${flagId}`
      : `/m/orgs/${orgSlug}/flags/${projectSlug}/${flagId}`;

  return (
    <section style={{ padding: 20 }}>
      <div className="m-flag-list-head">
        <h2 style={{ margin: '4px 0 4px' }}>{heading}</h2>
        <Link
          to={`/m/orgs/${orgSlug}/flags`}
          style={{
            fontSize: 13,
            color: 'var(--color-text-muted, #64748b)',
            textDecoration: 'none',
          }}
        >
          Switch project →
        </Link>
      </div>
      <StaleBadge lastSuccess={lastSuccess} inflight={refreshing} />

      <input
        type="search"
        className="m-input"
        placeholder="Search flags…"
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        style={{ width: '100%', margin: '12px 0 8px' }}
      />

      <CategoryFilterChips value={categories} onChange={setCategories} />

      {filteredFlags.length === 0 ? (
        <p style={{ color: 'var(--color-text-muted, #64748b)', marginTop: 16 }}>
          {flags.length === 0
            ? 'No flags in this project.'
            : 'No flags match these filters.'}
        </p>
      ) : (
        <ul
          style={{
            listStyle: 'none',
            padding: 0,
            margin: '12px 0 0',
            display: 'flex',
            flexDirection: 'column',
            gap: 8,
          }}
        >
          {filteredFlags.map((flag) => (
            <li key={flag.id}>
              <FlagRow
                flag={flag}
                detailHref={detailHref(flag.id)}
                environments={environments}
                states={stateMap.get(flag.id) ?? []}
              />
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
