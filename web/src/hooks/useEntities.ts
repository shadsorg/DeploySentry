import { useState, useEffect, useCallback } from 'react';
import { entitiesApi } from '@/api';
import type { Organization, Project, Application, Environment } from '@/types';

export function useOrgs() {
  const [orgs, setOrgs] = useState<Organization[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(() => {
    setLoading(true);
    setError(null);
    entitiesApi.listOrgs()
      .then((res) => setOrgs(res.organizations ?? []))
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, []);

  useEffect(() => { refresh(); }, [refresh]);

  return { orgs, loading, error, refresh };
}

export function useProjects(orgSlug: string | undefined) {
  const [projects, setProjects] = useState<Project[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(() => {
    if (!orgSlug) { setProjects([]); setLoading(false); return; }
    setLoading(true);
    setError(null);
    entitiesApi.listProjects(orgSlug)
      .then((res) => setProjects(res.projects ?? []))
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, [orgSlug]);

  useEffect(() => { refresh(); }, [refresh]);

  return { projects, loading, error, refresh };
}

export function useApps(orgSlug: string | undefined, projectSlug: string | undefined) {
  const [apps, setApps] = useState<Application[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(() => {
    if (!orgSlug || !projectSlug) { setApps([]); setLoading(false); return; }
    setLoading(true);
    setError(null);
    entitiesApi.listApps(orgSlug, projectSlug)
      .then((res) => setApps(res.applications ?? []))
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, [orgSlug, projectSlug]);

  useEffect(() => { refresh(); }, [refresh]);

  return { apps, loading, error, refresh };
}

export function useEnvironments(orgSlug: string | undefined, projectSlug: string | undefined, appSlug: string | undefined) {
  const [environments, setEnvironments] = useState<Environment[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(() => {
    if (!orgSlug || !projectSlug || !appSlug) { setEnvironments([]); setLoading(false); return; }
    setLoading(true);
    setError(null);
    entitiesApi.listEnvironments(orgSlug, projectSlug, appSlug)
      .then((res) => setEnvironments(res.environments ?? []))
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, [orgSlug, projectSlug, appSlug]);

  useEffect(() => { refresh(); }, [refresh]);

  return { environments, loading, error, refresh };
}
