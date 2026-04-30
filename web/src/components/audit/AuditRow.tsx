import { useState } from 'react';
import type { AuditLogEntry } from '@/types';
import AuditDiff from './AuditDiff';
import { actionLabel, nonRevertibleReason } from './labels';

interface Props {
  entry: AuditLogEntry;
  /** Optional resolved breadcrumb for the "Where" column (e.g. "acme / web / staging"). */
  where?: string;
  onRevert: (entry: AuditLogEntry) => void;
}

function relativeTime(iso: string): string {
  const t = new Date(iso).getTime();
  const diff = Date.now() - t;
  if (diff < 60_000)         return 'just now';
  if (diff < 3_600_000)      return `${Math.floor(diff / 60_000)}m ago`;
  if (diff < 86_400_000)     return `${Math.floor(diff / 3_600_000)}h ago`;
  if (diff < 7 * 86_400_000) return `${Math.floor(diff / 86_400_000)}d ago`;
  return new Date(iso).toLocaleDateString();
}

export default function AuditRow({ entry, where, onRevert }: Props) {
  const [expanded, setExpanded] = useState(false);

  const handleRevertClick = (e: React.MouseEvent) => {
    e.stopPropagation();
    onRevert(entry);
  };

  return (
    <>
      <div
        className={`audit-row${expanded ? ' expanded' : ''}`}
        onClick={() => setExpanded((x) => !x)}
        role="button"
        tabIndex={0}
        onKeyDown={(e) => {
          if (e.key === 'Enter' || e.key === ' ') {
            e.preventDefault();
            setExpanded((x) => !x);
          }
        }}
      >
        <div className="audit-row-when" title={entry.created_at}>{relativeTime(entry.created_at)}</div>
        <div className="audit-row-who">{entry.actor_name || '(unknown)'}</div>
        <div className="audit-row-what">{actionLabel(entry)}</div>
        <div className="audit-row-where">{where ?? ''}</div>
        <div className="audit-row-actions">
          {entry.revertible ? (
            <button
              type="button"
              className="btn btn-sm audit-revert-btn"
              onClick={handleRevertClick}
            >
              Revert
            </button>
          ) : (
            <span
              className="ms audit-revert-disabled"
              title={nonRevertibleReason(entry)}
              style={{ fontSize: 16, color: 'var(--color-text-muted)' }}
            >
              lock
            </span>
          )}
        </div>
      </div>
      {expanded && (
        <div className="audit-row-diff">
          <AuditDiff oldValue={entry.old_value} newValue={entry.new_value} />
        </div>
      )}
    </>
  );
}
