import { useEffect, useState } from 'react';
import type { RolloutGroup } from '@/types';
import { rolloutGroupsApi } from '@/api';

interface Props {
  orgSlug: string;
  value: string; // selected group id ('' = none)
  onChange: (groupID: string) => void;
}

export function GroupPicker({ orgSlug, value, onChange }: Props) {
  const [groups, setGroups] = useState<RolloutGroup[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    rolloutGroupsApi
      .list(orgSlug)
      .then((r) => setGroups(r.items || []))
      .finally(() => setLoading(false));
  }, [orgSlug]);

  if (loading) return <span>Loading groups…</span>;
  if (groups.length === 0) {
    return <span className="helper-text">No rollout groups in this org.</span>;
  }

  return (
    <select value={value} onChange={(e) => onChange(e.target.value)}>
      <option value="">— no group —</option>
      {groups.map((g) => (
        <option key={g.id} value={g.id}>
          {g.name} ({g.coordination_policy})
        </option>
      ))}
    </select>
  );
}
