import { useEffect, useState } from 'react';
import type { EffectiveStrategy, TargetType } from '@/types';
import { strategiesApi } from '@/api';

interface Props {
  orgSlug: string;
  targetType?: TargetType; // filter results
  value: string; // selected strategy name ('' = none)
  onChange: (strategyName: string) => void;
  allowImmediate?: boolean;
  immediate: boolean;
  onImmediateChange: (v: boolean) => void;
}

export function StrategyPicker({ orgSlug, targetType, value, onChange, allowImmediate, immediate, onImmediateChange }: Props) {
  const [options, setOptions] = useState<EffectiveStrategy[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    strategiesApi.list(orgSlug)
      .then((r) => {
        const filtered = targetType
          ? r.items.filter((e) => e.strategy.target_type === targetType || e.strategy.target_type === 'any')
          : r.items;
        setOptions(filtered);
      })
      .finally(() => setLoading(false));
  }, [orgSlug, targetType]);

  if (loading) return <span>Loading strategies…</span>;

  return (
    <div className="strategy-picker">
      {allowImmediate && (
        <label>
          <input
            type="checkbox"
            checked={immediate}
            onChange={(e) => onImmediateChange(e.target.checked)}
          />
          Apply immediately (skip rollout)
        </label>
      )}
      {!immediate && (
        <select value={value} onChange={(e) => onChange(e.target.value)}>
          <option value="">— select strategy —</option>
          {options.map((eff) => (
            <option key={eff.strategy.id} value={eff.strategy.name}>
              {eff.strategy.name}
              {eff.is_inherited ? ` (inherited from ${eff.origin_scope.type})` : ''}
            </option>
          ))}
        </select>
      )}
    </div>
  );
}
