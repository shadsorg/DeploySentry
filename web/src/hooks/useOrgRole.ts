import { useEffect, useState } from 'react';
import { membersApi } from '@/api';
import { useAuth } from '@/authHooks';
import type { Member } from '@/types';

export type OrgRole = Member['role'];

export function useOrgRole(orgSlug: string | undefined): {
  role: OrgRole | null;
  loading: boolean;
} {
  const { user } = useAuth();
  const [role, setRole] = useState<OrgRole | null>(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (!orgSlug || !user) {
      setRole(null);
      return;
    }
    let cancelled = false;
    setLoading(true);
    membersApi
      .listByOrg(orgSlug)
      .then((res) => {
        if (cancelled) return;
        const me = (res.members ?? []).find((m) => m.user_id === user.id);
        setRole(me ? me.role : null);
      })
      .catch(() => {
        if (!cancelled) setRole(null);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [orgSlug, user]);

  return { role, loading };
}
