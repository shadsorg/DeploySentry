import { useMemo, useState } from 'react';
import type { TargetingRule } from '../types';
import { flagsApi } from '../api';
import { isOfflineWriteBlockedError } from '../lib/offlineError';
import { ruleSummary } from '../lib/ruleSummary';

interface RuleEditSheetProps {
  rule: TargetingRule;
  flagId: string;
  open: boolean;
  onClose: () => void;
  onSaved: (updatedRule: TargetingRule) => void;
  onOfflineBlocked?: () => void;
}

const ATTRIBUTE_OPERATORS = [
  'eq',
  'neq',
  'contains',
  'starts_with',
  'ends_with',
  'in',
  'gt',
  'gte',
  'lt',
  'lte',
] as const;

interface EditState {
  percentage: number;
  user_ids: string[];
  attribute: string;
  operator: string;
  value: string;
  segment_id: string;
  start_time: string;
  end_time: string;
}

function initialState(rule: TargetingRule): EditState {
  return {
    percentage: rule.percentage ?? 0,
    user_ids: rule.user_ids ?? [],
    attribute: rule.attribute ?? '',
    operator: rule.operator ?? 'eq',
    value: rule.value ?? '',
    segment_id: rule.segment_id ?? '',
    start_time: rule.start_time ?? '',
    end_time: rule.end_time ?? '',
  };
}

function buildPatch(rule: TargetingRule, edits: EditState): Partial<TargetingRule> {
  const patch: Partial<TargetingRule> = {};
  switch (rule.rule_type) {
    case 'percentage': {
      if (edits.percentage !== (rule.percentage ?? 0)) {
        patch.percentage = edits.percentage;
      }
      return patch;
    }
    case 'user_target': {
      const before = rule.user_ids ?? [];
      const same =
        before.length === edits.user_ids.length &&
        before.every((v, i) => v === edits.user_ids[i]);
      if (!same) patch.user_ids = edits.user_ids;
      return patch;
    }
    case 'attribute': {
      if (edits.attribute !== (rule.attribute ?? '')) patch.attribute = edits.attribute;
      if (edits.operator !== (rule.operator ?? 'eq')) patch.operator = edits.operator;
      if (edits.value !== (rule.value ?? '')) patch.value = edits.value;
      return patch;
    }
    case 'segment': {
      if (edits.segment_id !== (rule.segment_id ?? '')) patch.segment_id = edits.segment_id;
      return patch;
    }
    case 'schedule': {
      if (edits.start_time !== (rule.start_time ?? '')) patch.start_time = edits.start_time;
      if (edits.end_time !== (rule.end_time ?? '')) patch.end_time = edits.end_time;
      return patch;
    }
    default:
      return patch;
  }
}

function previewRule(rule: TargetingRule, edits: EditState): TargetingRule {
  return {
    ...rule,
    percentage: edits.percentage,
    user_ids: edits.user_ids,
    attribute: edits.attribute,
    operator: edits.operator,
    value: edits.value,
    segment_id: edits.segment_id,
    start_time: edits.start_time,
    end_time: edits.end_time,
  };
}

export function RuleEditSheet({
  rule,
  flagId,
  open,
  onClose,
  onSaved,
  onOfflineBlocked,
}: RuleEditSheetProps) {
  const [edits, setEdits] = useState<EditState>(() => initialState(rule));
  const [chipDraft, setChipDraft] = useState('');
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const patch = useMemo(() => buildPatch(rule, edits), [rule, edits]);
  const hasChanges = Object.keys(patch).length > 0;
  const isCompound = rule.rule_type === 'compound';

  if (!open) return null;

  const handleClose = () => {
    setEdits(initialState(rule));
    setChipDraft('');
    setError(null);
    onClose();
  };

  const handleSave = async () => {
    if (!hasChanges || isCompound) return;
    setSaving(true);
    setError(null);
    try {
      const updated = await flagsApi.updateRule(flagId, rule.id, patch);
      onSaved(updated);
      onClose();
    } catch (err) {
      if (isOfflineWriteBlockedError(err)) {
        onOfflineBlocked?.();
      } else {
        setError(err instanceof Error ? err.message : 'Failed to save rule');
      }
    } finally {
      setSaving(false);
    }
  };

  const addChip = (raw: string) => {
    const v = raw.trim();
    if (!v) return;
    setEdits((s) =>
      s.user_ids.includes(v) ? s : { ...s, user_ids: [...s.user_ids, v] },
    );
  };

  const removeChip = (id: string) => {
    setEdits((s) => ({ ...s, user_ids: s.user_ids.filter((x) => x !== id) }));
  };

  const handleChipKey = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      addChip(chipDraft);
      setChipDraft('');
    }
  };

  const handleChipChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const v = e.target.value;
    if (v.endsWith(',')) {
      addChip(v.slice(0, -1));
      setChipDraft('');
    } else {
      setChipDraft(v);
    }
  };

  return (
    <div
      className="m-sheet"
      role="dialog"
      aria-modal="true"
      aria-label="Edit rule"
    >
      <div className="m-sheet-header">
        <button
          type="button"
          className="m-sheet-close"
          aria-label="Close"
          onClick={handleClose}
        >
          ✕
        </button>
        <span className="m-sheet-title">Edit rule</span>
        <button
          type="button"
          className="m-button m-button-primary m-sheet-save"
          onClick={handleSave}
          disabled={saving || !hasChanges || isCompound}
        >
          Save
        </button>
      </div>
      <div className="m-sheet-body">
        {isCompound ? (
          <p className="m-muted">Compound rules must be edited on desktop.</p>
        ) : rule.rule_type === 'percentage' ? (
          <div className="m-sheet-field">
            <label htmlFor="re-percentage" className="m-sheet-label">Percentage</label>
            <input
              id="re-percentage"
              className="m-input"
              type="number"
              min={0}
              max={100}
              value={edits.percentage}
              onChange={(e) =>
                setEdits((s) => ({ ...s, percentage: Number(e.target.value) }))
              }
            />
            <input
              type="range"
              min={0}
              max={100}
              value={edits.percentage}
              aria-label="Percentage slider"
              onChange={(e) =>
                setEdits((s) => ({ ...s, percentage: Number(e.target.value) }))
              }
              className="m-sheet-slider"
            />
          </div>
        ) : rule.rule_type === 'user_target' ? (
          <div className="m-sheet-field">
            <span className="m-sheet-label">User IDs</span>
            <div className="m-sheet-chips">
              {edits.user_ids.map((u) => (
                <button
                  key={u}
                  type="button"
                  className="m-chip"
                  aria-label={`Remove ${u}`}
                  onClick={() => removeChip(u)}
                >
                  {u} ✕
                </button>
              ))}
            </div>
            <input
              className="m-input"
              type="text"
              aria-label="Add user ID"
              placeholder="Type and press Enter or comma"
              value={chipDraft}
              onChange={handleChipChange}
              onKeyDown={handleChipKey}
            />
          </div>
        ) : rule.rule_type === 'attribute' ? (
          <>
            <div className="m-sheet-field">
              <label htmlFor="re-attr" className="m-sheet-label">Attribute</label>
              <input
                id="re-attr"
                className="m-input"
                type="text"
                value={edits.attribute}
                onChange={(e) =>
                  setEdits((s) => ({ ...s, attribute: e.target.value }))
                }
              />
            </div>
            <div className="m-sheet-field">
              <label htmlFor="re-op" className="m-sheet-label">Operator</label>
              <select
                id="re-op"
                className="m-input"
                value={edits.operator}
                onChange={(e) =>
                  setEdits((s) => ({ ...s, operator: e.target.value }))
                }
              >
                {ATTRIBUTE_OPERATORS.map((op) => (
                  <option key={op} value={op}>{op}</option>
                ))}
              </select>
            </div>
            <div className="m-sheet-field">
              <label htmlFor="re-val" className="m-sheet-label">Value</label>
              <input
                id="re-val"
                className="m-input"
                type="text"
                value={edits.value}
                onChange={(e) =>
                  setEdits((s) => ({ ...s, value: e.target.value }))
                }
              />
            </div>
          </>
        ) : rule.rule_type === 'segment' ? (
          <div className="m-sheet-field">
            <label htmlFor="re-seg" className="m-sheet-label">Segment ID</label>
            <input
              id="re-seg"
              className="m-input"
              type="text"
              value={edits.segment_id}
              onChange={(e) =>
                setEdits((s) => ({ ...s, segment_id: e.target.value }))
              }
            />
          </div>
        ) : rule.rule_type === 'schedule' ? (
          <>
            <div className="m-sheet-field">
              <label htmlFor="re-start" className="m-sheet-label">Start time</label>
              <input
                id="re-start"
                className="m-input"
                type="datetime-local"
                value={edits.start_time}
                onChange={(e) =>
                  setEdits((s) => ({ ...s, start_time: e.target.value }))
                }
              />
            </div>
            <div className="m-sheet-field">
              <label htmlFor="re-end" className="m-sheet-label">End time</label>
              <input
                id="re-end"
                className="m-input"
                type="datetime-local"
                value={edits.end_time}
                onChange={(e) =>
                  setEdits((s) => ({ ...s, end_time: e.target.value }))
                }
              />
            </div>
          </>
        ) : null}
      </div>
      <div className="m-sheet-footer">
        <p className="m-muted m-sheet-preview" data-testid="rule-preview">
          {ruleSummary(previewRule(rule, edits))}
        </p>
        {error && (
          <p role="alert" className="m-sheet-error">
            {error}
          </p>
        )}
      </div>
    </div>
  );
}
