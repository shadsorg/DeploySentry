import AuditDiff from '@/components/audit/AuditDiff';
import type { StagedChange } from '@/api';
import { actionLabel } from './labels';

export interface StagedChangeRowProps {
  row: StagedChange;
  selected: boolean;
  /**
   * True when production state has drifted away from the value the user
   * staged against — Deploy would clobber a more recent change. Renders
   * an inline warning and unchecks the row by default upstream.
   */
  conflict: boolean;
  onToggleSelected: (id: string) => void;
  onDiscard: (id: string) => void;
}

/**
 * One pending change in the review page. Pure presentational — selection
 * state lives one level up so bulk Deploy / Discard can address all rows.
 */
export default function StagedChangeRow({
  row,
  selected,
  conflict,
  onToggleSelected,
  onDiscard,
}: StagedChangeRowProps) {
  const oldText =
    row.old_value === undefined || row.old_value === null
      ? ''
      : typeof row.old_value === 'string'
        ? row.old_value
        : JSON.stringify(row.old_value);
  const newText =
    row.new_value === undefined || row.new_value === null
      ? ''
      : typeof row.new_value === 'string'
        ? row.new_value
        : JSON.stringify(row.new_value);

  const ridLabel = row.resource_id ?? row.provisional_id ?? '—';

  return (
    <div
      className={`staging-row ${conflict ? 'staging-row--conflict' : ''}`}
      data-testid={`staging-row-${row.id}`}
    >
      <div className="staging-row-header">
        <label className="staging-row-checkbox">
          <input
            type="checkbox"
            checked={selected}
            onChange={() => onToggleSelected(row.id)}
            aria-label={`Select ${actionLabel(row)} on ${ridLabel}`}
          />
          <span className="staging-row-action">{actionLabel(row)}</span>
        </label>
        <code className="staging-row-target">{ridLabel}</code>
        <span className="staging-row-staged-at">
          staged {new Date(row.created_at).toLocaleString()}
        </span>
        <button type="button" className="staging-row-discard" onClick={() => onDiscard(row.id)}>
          Discard
        </button>
      </div>
      {conflict && (
        <div className="staging-row-conflict-banner" role="alert">
          ⚠ This change may overwrite a newer commit. Review the diff carefully before deploying.
        </div>
      )}
      <AuditDiff oldValue={oldText} newValue={newText} />
    </div>
  );
}
