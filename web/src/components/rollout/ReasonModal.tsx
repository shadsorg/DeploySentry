import { useState } from 'react';

interface Props {
  title: string;
  placeholder?: string;
  required?: boolean;
  onConfirm: (reason: string) => void | Promise<void>;
  onCancel: () => void;
}

export function ReasonModal({ title, placeholder, required = false, onConfirm, onCancel }: Props) {
  const [reason, setReason] = useState('');
  const [submitting, setSubmitting] = useState(false);

  async function handleConfirm() {
    if (required && !reason.trim()) return;
    setSubmitting(true);
    try {
      await onConfirm(reason.trim());
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="modal-backdrop" onClick={onCancel}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <h3>{title}</h3>
        <textarea
          autoFocus
          placeholder={placeholder || (required ? 'Reason (required)' : 'Reason (optional)')}
          value={reason}
          onChange={(e) => setReason(e.target.value)}
          rows={4}
        />
        <div className="modal-actions">
          <button type="button" onClick={onCancel} disabled={submitting}>
            Cancel
          </button>
          <button
            type="button"
            onClick={handleConfirm}
            disabled={submitting || (required && !reason.trim())}
            className="btn-primary"
          >
            Confirm
          </button>
        </div>
      </div>
    </div>
  );
}
