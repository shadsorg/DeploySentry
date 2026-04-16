import { useState, useEffect, useCallback } from 'react';
import { groupsApi } from '../api';
import type { Group, GroupMember } from '../api';

export function useGroups(orgSlug: string | undefined) {
  const [groups, setGroups] = useState<Group[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(() => {
    if (!orgSlug) {
      setGroups([]);
      setLoading(false);
      return;
    }
    setLoading(true);
    setError(null);
    groupsApi
      .list(orgSlug)
      .then((g) => setGroups(g ?? []))
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false));
  }, [orgSlug]);

  useEffect(() => {
    refresh();
  }, [refresh]);

  return { groups, loading, error, refresh };
}

export function useGroupMembers(orgSlug: string | undefined, groupSlug: string | undefined) {
  const [members, setMembers] = useState<GroupMember[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(() => {
    if (!orgSlug || !groupSlug) {
      setMembers([]);
      setLoading(false);
      return;
    }
    setLoading(true);
    setError(null);
    groupsApi
      .listMembers(orgSlug, groupSlug)
      .then((m) => setMembers(m ?? []))
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false));
  }, [orgSlug, groupSlug]);

  useEffect(() => {
    refresh();
  }, [refresh]);

  return { members, loading, error, refresh };
}
