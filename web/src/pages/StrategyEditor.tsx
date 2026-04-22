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

  function addAbort(stepIdx: number) {
    setSteps((prev) =>
      prev.map((s, i) =>
        i === stepIdx
          ? {
              ...s,
              abort_conditions: [
                ...(s.abort_conditions || []),
                { metric: 'error_rate', operator: '>', threshold: 0.02, window: 60 * 1000 * MS },
              ],
            }
          : s,
      ),
    );
  }

  function removeAbort(stepIdx: number, acIdx: number) {
    setSteps((prev) =>
      prev.map((s, i) =>
        i === stepIdx
          ? { ...s, abort_conditions: (s.abort_conditions || []).filter((_, j) => j !== acIdx) }
          : s,
      ),
    );
  }

  function updateAbort(
    stepIdx: number,
    acIdx: number,
    patch: Partial<{ metric: string; operator: string; threshold: number; window: number }>,
  ) {
    setSteps((prev) =>
      prev.map((s, i) =>
        i === stepIdx
          ? {
              ...s,
              abort_conditions: (s.abort_conditions || []).map((ac, j) =>
                j === acIdx ? { ...ac, ...patch } : ac,
              ),
            }
          : s,
      ),
    );
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

        <label>
          Name
          <input
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            disabled={Boolean(strategyName)}
          />
        </label>

        <label>
          Description
          <input type="text" value={description} onChange={(e) => setDescription(e.target.value)} />
        </label>

        <label>
          Target Type
          <select value={targetType} onChange={(e) => setTargetType(e.target.value as TargetType)}>
            <option value="deploy">deploy</option>
            <option value="config">config</option>
            <option value="any">any</option>
          </select>
        </label>

        <label>
          Default Health Threshold (0–1)
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
              <>
                <tr key={idx}>
                  <td>{idx + 1}</td>
                  <td>
                    <input
                      type="number"
                      value={s.percent}
                      onChange={(e) => updateStep(idx, { percent: Number(e.target.value) })}
                    />
                  </td>
                  <td>
                    <input
                      type="number"
                      value={s.min_duration / MS}
                      onChange={(e) =>
                        updateStep(idx, { min_duration: Number(e.target.value) * MS })
                      }
                    />
                  </td>
                  <td>
                    <input
                      type="number"
                      value={s.max_duration / MS}
                      onChange={(e) =>
                        updateStep(idx, { max_duration: Number(e.target.value) * MS })
                      }
                    />
                  </td>
                  <td>
                    <input
                      type="number"
                      value={s.bake_time_healthy / MS}
                      onChange={(e) =>
                        updateStep(idx, { bake_time_healthy: Number(e.target.value) * MS })
                      }
                    />
                  </td>
                  <td>
                    {steps.length > 1 && (
                      <button type="button" onClick={() => removeStep(idx)}>
                        ×
                      </button>
                    )}
                  </td>
                </tr>
                <tr key={`${idx}-advanced`}>
                  <td colSpan={6}>
                    <details className="step-advanced">
                      <summary>Advanced: approval, abort conditions, health threshold</summary>

                      <label>
                        Health threshold (leave blank for strategy default)
                        <input
                          type="number"
                          min={0}
                          max={1}
                          step={0.01}
                          value={s.health_threshold ?? ''}
                          onChange={(e) =>
                            updateStep(idx, {
                              health_threshold:
                                e.target.value === '' ? undefined : Number(e.target.value),
                            } as Partial<Step>)
                          }
                        />
                      </label>

                      <fieldset>
                        <legend>Approval (optional)</legend>
                        <label>
                          <input
                            type="checkbox"
                            checked={!!s.approval}
                            onChange={(e) =>
                              updateStep(idx, {
                                approval: e.target.checked
                                  ? { required_role: '', timeout: 0 }
                                  : undefined,
                              } as Partial<Step>)
                            }
                          />
                          Require manual approval
                        </label>
                        {s.approval && (
                          <>
                            <label>
                              Required role
                              <input
                                type="text"
                                value={s.approval.required_role}
                                onChange={(e) =>
                                  updateStep(idx, {
                                    approval: { ...s.approval!, required_role: e.target.value },
                                  })
                                }
                              />
                            </label>
                            <label>
                              Timeout (ms)
                              <input
                                type="number"
                                value={s.approval.timeout / MS}
                                onChange={(e) =>
                                  updateStep(idx, {
                                    approval: {
                                      ...s.approval!,
                                      timeout: Number(e.target.value) * MS,
                                    },
                                  })
                                }
                              />
                            </label>
                          </>
                        )}
                      </fieldset>

                      <fieldset>
                        <legend>Abort conditions</legend>
                        {(s.abort_conditions || []).map((ac, acIdx) => (
                          <div key={acIdx} className="abort-row">
                            <select
                              value={ac.metric.startsWith('custom:') ? '__custom__' : ac.metric}
                              onChange={(e) => {
                                const v = e.target.value;
                                updateAbort(idx, acIdx, {
                                  metric: v === '__custom__' ? 'custom:' : v,
                                });
                              }}
                            >
                              <option value="score">score</option>
                              <option value="error_rate">error_rate</option>
                              <option value="latency_p99_ms">latency_p99_ms</option>
                              <option value="latency_p50_ms">latency_p50_ms</option>
                              <option value="request_rate">request_rate</option>
                              <option value="__custom__">custom…</option>
                            </select>
                            {ac.metric.startsWith('custom:') && (
                              <input
                                type="text"
                                placeholder="custom:my_metric"
                                value={ac.metric}
                                onChange={(e) => {
                                  const v = e.target.value.startsWith('custom:')
                                    ? e.target.value
                                    : 'custom:' + e.target.value;
                                  updateAbort(idx, acIdx, { metric: v });
                                }}
                              />
                            )}
                            <select
                              value={ac.operator}
                              onChange={(e) =>
                                updateAbort(idx, acIdx, { operator: e.target.value })
                              }
                            >
                              {(['>', '>=', '<', '<=', '==', '!='] as const).map((op) => (
                                <option key={op} value={op}>
                                  {op}
                                </option>
                              ))}
                            </select>
                            <input
                              type="number"
                              step="any"
                              value={ac.threshold}
                              onChange={(e) =>
                                updateAbort(idx, acIdx, { threshold: Number(e.target.value) })
                              }
                              placeholder="threshold"
                            />
                            <input
                              type="number"
                              value={ac.window / MS}
                              onChange={(e) =>
                                updateAbort(idx, acIdx, { window: Number(e.target.value) * MS })
                              }
                              placeholder="window (ms)"
                            />
                            <button type="button" onClick={() => removeAbort(idx, acIdx)}>
                              ×
                            </button>
                          </div>
                        ))}
                        <button type="button" onClick={() => addAbort(idx)}>
                          + Add abort condition
                        </button>
                      </fieldset>
                    </details>
                  </td>
                </tr>
              </>
            ))}
          </tbody>
        </table>
        <button type="button" onClick={addStep}>
          + Add Step
        </button>

        <div className="modal-actions">
          <button type="button" onClick={onClose}>
            Cancel
          </button>
          <button type="button" onClick={submit} disabled={submitting} className="btn-primary">
            {strategyName ? 'Save' : 'Create'}
          </button>
        </div>
      </div>
    </div>
  );
}
