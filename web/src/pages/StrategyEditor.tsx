import { useEffect, useState } from 'react';
import type { Step, TargetType } from '@/types';
import { strategiesApi } from '@/api';

interface Props {
  orgSlug: string;
  strategyName: string | null; // null = creating new
  onClose: () => void;
}

const MS = 1_000_000; // nanoseconds per millisecond

function emptyStep(): Step {
  return {
    percent: 0,
    min_duration: 0,
    max_duration: 0,
    bake_time_healthy: 0,
  };
}

export function StrategyEditor({ orgSlug, strategyName, onClose }: Props) {
  const [name, setName] = useState(strategyName ?? '');
  const [description, setDescription] = useState('');
  const [targetType, setTargetType] = useState<TargetType>('deploy');
  const [healthThreshold, setHealthThreshold] = useState(0.95);
  const [rollbackOnFailure, setRollbackOnFailure] = useState(true);
  const [steps, setSteps] = useState<Step[]>([emptyStep()]);
  const [expectedVersion, setExpectedVersion] = useState(1);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (strategyName) {
      strategiesApi.get(orgSlug, strategyName).then((s) => {
        setName(s.name);
        setDescription(s.description);
        setTargetType(s.target_type);
        setHealthThreshold(s.default_health_threshold);
        setRollbackOnFailure(s.default_rollback_on_failure);
        setSteps(s.steps);
        setExpectedVersion(s.version);
      });
    }
  }, [orgSlug, strategyName]);

  function updateStep(idx: number, patch: Partial<Step>) {
    setSteps((prev) => prev.map((s, i) => (i === idx ? { ...s, ...patch } : s)));
  }

  function removeStep(idx: number) {
    setSteps((prev) => prev.filter((_, i) => i !== idx));
  }

  function addStep() {
    setSteps((prev) => [...prev, emptyStep()]);
  }

  async function submit() {
    setSubmitting(true);
    setError(null);
    try {
      if (strategyName) {
        await strategiesApi.update(orgSlug, strategyName, {
          description,
          target_type: targetType,
          steps,
          default_health_threshold: healthThreshold,
          default_rollback_on_failure: rollbackOnFailure,
          expected_version: expectedVersion,
        });
      } else {
        await strategiesApi.create(orgSlug, {
          name,
          description,
          target_type: targetType,
          steps,
          default_health_threshold: healthThreshold,
          default_rollback_on_failure: rollbackOnFailure,
        });
      }
      onClose();
    } catch (e) {
      setError(String(e));
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal modal-wide" onClick={(e) => e.stopPropagation()}>
        <h3>{strategyName ? `Edit Strategy: ${strategyName}` : 'New Strategy'}</h3>

        {error && <p className="error">{error}</p>}

        <label>Name
          <input
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            disabled={Boolean(strategyName)}
          />
        </label>

        <label>Description
          <input type="text" value={description} onChange={(e) => setDescription(e.target.value)} />
        </label>

        <label>Target Type
          <select value={targetType} onChange={(e) => setTargetType(e.target.value as TargetType)}>
            <option value="deploy">deploy</option>
            <option value="config">config</option>
            <option value="any">any</option>
          </select>
        </label>

        <label>Default Health Threshold (0–1)
          <input
            type="number"
            min={0}
            max={1}
            step={0.01}
            value={healthThreshold}
            onChange={(e) => setHealthThreshold(Number(e.target.value))}
          />
        </label>

        <label>
          <input
            type="checkbox"
            checked={rollbackOnFailure}
            onChange={(e) => setRollbackOnFailure(e.target.checked)}
          />
          Rollback on failure
        </label>

        <h4>Steps</h4>
        <table className="step-table">
          <thead>
            <tr>
              <th>#</th>
              <th>Percent</th>
              <th>Min (ms)</th>
              <th>Max (ms)</th>
              <th>Bake (ms)</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {steps.map((s, idx) => (
              <tr key={idx}>
                <td>{idx + 1}</td>
                <td><input type="number" value={s.percent} onChange={(e) => updateStep(idx, { percent: Number(e.target.value) })} /></td>
                <td><input type="number" value={s.min_duration / MS} onChange={(e) => updateStep(idx, { min_duration: Number(e.target.value) * MS })} /></td>
                <td><input type="number" value={s.max_duration / MS} onChange={(e) => updateStep(idx, { max_duration: Number(e.target.value) * MS })} /></td>
                <td><input type="number" value={s.bake_time_healthy / MS} onChange={(e) => updateStep(idx, { bake_time_healthy: Number(e.target.value) * MS })} /></td>
                <td>
                  {steps.length > 1 && (
                    <button type="button" onClick={() => removeStep(idx)}>×</button>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        <button type="button" onClick={addStep}>+ Add Step</button>

        <div className="modal-actions">
          <button type="button" onClick={onClose}>Cancel</button>
          <button type="button" onClick={submit} disabled={submitting} className="btn-primary">
            {strategyName ? 'Save' : 'Create'}
          </button>
        </div>
      </div>
    </div>
  );
}
