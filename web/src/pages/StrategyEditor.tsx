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
      <div className="modal modal-wide strategy-editor" onClick={(e) => e.stopPropagation()}>
        <div className="strategy-editor-head">
          <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
            <div style={{
              width: 32, height: 32, borderRadius: 8,
              background: 'var(--color-primary-bg)', border: '1px solid rgba(99,102,241,0.25)',
              display: 'flex', alignItems: 'center', justifyContent: 'center',
            }}>
              <span className="ms" style={{ fontSize: 18, color: 'var(--color-primary)' }}>architecture</span>
            </div>
            <h3 style={{ margin: 0 }}>{strategyName ? `Edit Strategy: ${strategyName}` : 'New Strategy'}</h3>
          </div>
          <button
            type="button"
            className="btn-icon"
            onClick={onClose}
            aria-label="Close"
          >
            <span className="ms" style={{ fontSize: 16 }}>close</span>
          </button>
        </div>

        {error && <p className="error">{error}</p>}

        <section className="strategy-editor-section">
          <div className="strategy-editor-section-title">
            <span className="ms" style={{ fontSize: 16, color: 'var(--color-primary)' }}>tune</span>
            General
          </div>
          <div className="strategy-editor-grid">
            <div className="form-group">
              <label className="form-label">Name</label>
              <input
                className="form-input"
                type="text"
                value={name}
                onChange={(e) => setName(e.target.value)}
                disabled={Boolean(strategyName)}
                placeholder="canary-progressive"
              />
            </div>
            <div className="form-group">
              <label className="form-label">Target Type</label>
              <select
                className="form-select"
                value={targetType}
                onChange={(e) => setTargetType(e.target.value as TargetType)}
              >
                <option value="deploy">deploy</option>
                <option value="config">config</option>
                <option value="any">any</option>
              </select>
            </div>
          </div>
          <div className="form-group">
            <label className="form-label">Description</label>
            <input
              className="form-input"
              type="text"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Short description for reviewers"
            />
          </div>
          <div className="strategy-editor-grid">
            <div className="form-group" style={{ marginBottom: 0 }}>
              <label className="form-label">Default Health Threshold (0–1)</label>
              <input
                className="form-input"
                type="number"
                min={0}
                max={1}
                step={0.01}
                value={healthThreshold}
                onChange={(e) => setHealthThreshold(Number(e.target.value))}
              />
              <p className="form-hint">Rollouts abort when the health score drops below this value.</p>
            </div>
            <div className="form-group" style={{ marginBottom: 0 }}>
              <label className="form-label">Rollback on failure</label>
              <label style={{
                display: 'flex', alignItems: 'center', gap: 10,
                padding: '9px 12px',
                background: 'var(--color-bg)',
                border: '1px solid var(--color-border)',
                borderRadius: 'var(--radius-md)',
                cursor: 'pointer',
                fontSize: 13,
                color: 'var(--color-text)',
              }}>
                <input
                  type="checkbox"
                  checked={rollbackOnFailure}
                  onChange={(e) => setRollbackOnFailure(e.target.checked)}
                />
                Automatically rollback when rollout aborts
              </label>
            </div>
          </div>
        </section>

        <section className="strategy-editor-section">
          <div className="strategy-editor-section-title">
            <span className="ms" style={{ fontSize: 16, color: 'var(--color-primary)' }}>steps</span>
            Steps
            <span className="badge" style={{
              background: 'var(--color-primary-bg)',
              color: 'var(--color-primary)',
              marginLeft: 'auto',
            }}>{steps.length}</span>
          </div>

          <div className="strategy-editor-steps">
            {steps.map((s, idx) => (
              <div className="strategy-step" key={idx}>
                <div className="strategy-step-head">
                  <div className="strategy-step-num">{idx + 1}</div>
                  <div style={{ flex: 1, fontSize: 13, fontWeight: 600 }}>Step {idx + 1}</div>
                  {steps.length > 1 && (
                    <button
                      type="button"
                      className="btn-icon"
                      title="Remove step"
                      onClick={() => removeStep(idx)}
                    >
                      <span className="ms" style={{ fontSize: 14, color: 'var(--color-danger)' }}>delete</span>
                    </button>
                  )}
                </div>

                <div className="strategy-step-fields">
                  <div className="form-group" style={{ marginBottom: 0 }}>
                    <label className="form-label">Percent</label>
                    <input
                      className="form-input"
                      type="number"
                      value={s.percent}
                      onChange={(e) => updateStep(idx, { percent: Number(e.target.value) })}
                    />
                  </div>
                  <div className="form-group" style={{ marginBottom: 0 }}>
                    <label className="form-label">Min (ms)</label>
                    <input
                      className="form-input"
                      type="number"
                      value={s.min_duration / MS}
                      onChange={(e) => updateStep(idx, { min_duration: Number(e.target.value) * MS })}
                    />
                  </div>
                  <div className="form-group" style={{ marginBottom: 0 }}>
                    <label className="form-label">Max (ms)</label>
                    <input
                      className="form-input"
                      type="number"
                      value={s.max_duration / MS}
                      onChange={(e) => updateStep(idx, { max_duration: Number(e.target.value) * MS })}
                    />
                  </div>
                  <div className="form-group" style={{ marginBottom: 0 }}>
                    <label className="form-label">Bake (ms)</label>
                    <input
                      className="form-input"
                      type="number"
                      value={s.bake_time_healthy / MS}
                      onChange={(e) => updateStep(idx, { bake_time_healthy: Number(e.target.value) * MS })}
                    />
                  </div>
                </div>

                <details className="step-advanced">
                  <summary>
                    <span className="ms" style={{ fontSize: 14, verticalAlign: 'middle' }}>tune</span>
                    {' '}Advanced: approval, abort conditions, per-step health threshold
                  </summary>

                  <div className="form-group">
                    <label className="form-label">Health threshold (leave blank for strategy default)</label>
                    <input
                      className="form-input"
                      type="number"
                      min={0}
                      max={1}
                      step={0.01}
                      value={s.health_threshold ?? ''}
                      onChange={(e) =>
                        updateStep(idx, {
                          health_threshold: e.target.value === '' ? undefined : Number(e.target.value),
                        } as Partial<Step>)
                      }
                    />
                  </div>

                  <fieldset>
                    <legend>Approval (optional)</legend>
                    <label style={{ display: 'flex', gap: 8, alignItems: 'center', fontSize: 13, marginBottom: 8 }}>
                      <input
                        type="checkbox"
                        checked={!!s.approval}
                        onChange={(e) =>
                          updateStep(idx, {
                            approval: e.target.checked ? { required_role: '', timeout: 0 } : undefined,
                          } as Partial<Step>)
                        }
                      />
                      Require manual approval
                    </label>
                    {s.approval && (
                      <div className="form-row">
                        <div className="form-group" style={{ marginBottom: 0 }}>
                          <label className="form-label">Required role</label>
                          <input
                            className="form-input"
                            type="text"
                            value={s.approval.required_role}
                            onChange={(e) =>
                              updateStep(idx, { approval: { ...s.approval!, required_role: e.target.value } })
                            }
                          />
                        </div>
                        <div className="form-group" style={{ marginBottom: 0 }}>
                          <label className="form-label">Timeout (ms)</label>
                          <input
                            className="form-input"
                            type="number"
                            value={s.approval.timeout / MS}
                            onChange={(e) =>
                              updateStep(idx, { approval: { ...s.approval!, timeout: Number(e.target.value) * MS } })
                            }
                          />
                        </div>
                      </div>
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
                            updateAbort(idx, acIdx, { metric: v === '__custom__' ? 'custom:' : v });
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
                          onChange={(e) => updateAbort(idx, acIdx, { operator: e.target.value })}
                        >
                          {(['>', '>=', '<', '<=', '==', '!='] as const).map((op) => (
                            <option key={op} value={op}>{op}</option>
                          ))}
                        </select>
                        <input
                          type="number"
                          step="any"
                          value={ac.threshold}
                          onChange={(e) => updateAbort(idx, acIdx, { threshold: Number(e.target.value) })}
                          placeholder="threshold"
                        />
                        <input
                          type="number"
                          value={ac.window / MS}
                          onChange={(e) => updateAbort(idx, acIdx, { window: Number(e.target.value) * MS })}
                          placeholder="window (ms)"
                        />
                        <button type="button" onClick={() => removeAbort(idx, acIdx)} aria-label="Remove">×</button>
                      </div>
                    ))}
                    <button
                      type="button"
                      className="btn btn-sm btn-secondary"
                      onClick={() => addAbort(idx)}
                      style={{ marginTop: 6 }}
                    >
                      <span className="ms" style={{ fontSize: 14 }}>add</span>
                      Add abort condition
                    </button>
                  </fieldset>
                </details>
              </div>
            ))}
          </div>

          <button
            type="button"
            className="btn btn-secondary"
            onClick={addStep}
            style={{ marginTop: 12, width: '100%', justifyContent: 'center' }}
          >
            <span className="ms" style={{ fontSize: 16 }}>add</span>
            Add Step
          </button>
        </section>

        <div className="modal-actions">
          <button type="button" onClick={onClose} disabled={submitting}>Cancel</button>
          <button type="button" onClick={submit} disabled={submitting} className="btn-primary">
            {submitting ? 'Saving…' : strategyName ? 'Save' : 'Create'}
          </button>
        </div>
      </div>
    </div>
  );
}
