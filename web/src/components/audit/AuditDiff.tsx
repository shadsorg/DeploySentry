import { useMemo } from 'react';

interface Props {
  oldValue: string;
  newValue: string;
}

function pretty(value: string): string {
  if (!value) return '(empty)';
  try {
    return JSON.stringify(JSON.parse(value), null, 2);
  } catch {
    return value;
  }
}

export default function AuditDiff({ oldValue, newValue }: Props) {
  const oldText = useMemo(() => pretty(oldValue), [oldValue]);
  const newText = useMemo(() => pretty(newValue), [newValue]);

  return (
    <div className="audit-diff">
      <div className="audit-diff-pane">
        <div className="audit-diff-pane-label">Before</div>
        <pre className="audit-diff-pre">{oldText}</pre>
      </div>
      <div className="audit-diff-pane">
        <div className="audit-diff-pane-label">After</div>
        <pre className="audit-diff-pre">{newText}</pre>
      </div>
    </div>
  );
}
