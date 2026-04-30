import { useEffect, useRef, useState } from 'react';

interface ConfirmDialogProps {
  open: boolean;
  title: string;
  message: string;
  confirmLabel?: string;
  confirmVariant?: 'danger' | 'primary';
  onConfirm: () => void;
  onCancel: () => void;
  loading?: boolean;
  /**
   * If set, the user must type this exact string and tick the
   * acknowledgement checkbox before confirm is enabled. Used for
   * destructive actions where we want eyes-on-target verification.
   */
  requireTypedConfirm?: string;
  acknowledgement?: string;
}

export default function ConfirmDialog({
  open,
  title,
  message,
  confirmLabel = 'Confirm',
  confirmVariant = 'primary',
  onConfirm,
  onCancel,
  loading = false,
  requireTypedConfirm,
  acknowledgement,
}: ConfirmDialogProps) {
  const dialogRef = useRef<HTMLDialogElement>(null);
  const [typed, setTyped] = useState('');
  const [acked, setAcked] = useState(false);

  useEffect(() => {
    const el = dialogRef.current;
    if (!el) return;
    if (open && !el.open) el.showModal();
    if (!open && el.open) el.close();
    if (!open) {
      setTyped('');
      setAcked(false);
    }
  }, [open]);

  if (!open) return null;

  const typedOk = !requireTypedConfirm || typed === requireTypedConfirm;
  const ackOk = !acknowledgement || acked;
  const canConfirm = typedOk && ackOk && !loading;

  return (
    <dialog ref={dialogRef} className="confirm-dialog" onCancel={onCancel}>
      <div className="confirm-dialog-content">
        <h3 className="confirm-dialog-title">{title}</h3>
        <p className="confirm-dialog-message">{message}</p>
        {requireTypedConfirm && (
          <div className="confirm-dialog-typed">
            <label htmlFor="confirm-typed">
              Type <code>{requireTypedConfirm}</code> to confirm:
            </label>
            <input
              id="confirm-typed"
              className="form-input"
              type="text"
              value={typed}
              onChange={(e) => setTyped(e.target.value)}
              autoComplete="off"
              autoFocus
            />
          </div>
        )}
        {acknowledgement && (
          <label className="confirm-dialog-ack">
            <input type="checkbox" checked={acked} onChange={(e) => setAcked(e.target.checked)} />{' '}
            {acknowledgement}
          </label>
        )}
        <div className="confirm-dialog-actions">
          <button className="btn btn-secondary" onClick={onCancel} disabled={loading}>
            Cancel
          </button>
          <button
            className={`btn btn-${confirmVariant}`}
            onClick={onConfirm}
            disabled={!canConfirm}
          >
            {loading ? 'Working...' : confirmLabel}
          </button>
        </div>
      </div>
    </dialog>
  );
}
