import { useEffect } from 'react';
import { auditApi } from '@/api';

interface Props {
  open: boolean;
  newEntryId: string;
  newAction: string;
  onClose: () => void;
  onUndoSuccess: () => void;
}

export default function RevertToast({ open, newEntryId, newAction, onClose, onUndoSuccess }: Props) {
  useEffect(() => {
    if (!open) return;
    const t = setTimeout(onClose, 6000);
    return () => clearTimeout(t);
  }, [open, onClose]);

  if (!open) return null;

  const handleUndo = async () => {
    try {
      await auditApi.revert(newEntryId, false);
      onUndoSuccess();
    } finally {
      onClose();
    }
  };

  return (
    <div className="revert-toast" role="status" aria-live="polite">
      <span className="ms" style={{ fontSize: 16, color: 'var(--color-success, #16a34a)' }}>
        check_circle
      </span>
      <span>Reverted ({newAction})</span>
      <button className="revert-toast-undo" onClick={handleUndo}>
        Undo Revert
      </button>
      <button className="revert-toast-close ms" aria-label="Dismiss" onClick={onClose}>
        close
      </button>
    </div>
  );
}
