import { useEffect, useRef, useState } from 'react';
import type { AuditLogEntry } from '@/types';
import { auditApi } from '@/api';
import AuditDiff from './AuditDiff';
import { actionLabel } from './labels';

interface Props {
  open: boolean;
  entry: AuditLogEntry | null;
  onClose: () => void;
  onSuccess: (newEntryId: string, newAction: string) => void;
}

type Phase = 'confirm' | 'race' | 'error';

export default function RevertConfirmDialog({ open, entry, onClose, onSuccess }: Props) {
  const dialogRef = useRef<HTMLDialogElement>(null);
  const [phase, setPhase] = useState<Phase>('confirm');
  const [errorMsg, setErrorMsg] = useState<string>('');
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    const el = dialogRef.current;
    if (!el) return;
    if (open && !el.open) el.showModal();
    if (!open && el.open) el.close();
    if (!open) {
      setPhase('confirm');
      setErrorMsg('');
      setBusy(false);
    }
  }, [open]);

  if (!open || !entry) return null;

  const doRevert = async (force: boolean) => {
    setBusy(true);
    setErrorMsg('');
    try {
      const resp = await auditApi.revert(entry.id, force);
      onSuccess(resp.audit_entry_id, resp.action);
      onClose();
    } catch (err: unknown) {
      // request() throws new Error(body.error) on non-2xx.
      // The 409 race body.error is "resource has changed since this entry; pass..."
      // The `code: "race"` field is lost in the throw — match on the literal substring.
      const msg = err instanceof Error ? err.message : String(err);
      if (!force && msg.includes('resource has changed')) {
        setPhase('race');
      } else {
        setErrorMsg(msg || 'Unknown error.');
        setPhase('error');
      }
    } finally {
      setBusy(false);
    }
  };

  return (
    <dialog ref={dialogRef} className="confirm-dialog revert-dialog" onCancel={onClose}>
      <div className="confirm-dialog-content">
        {phase === 'confirm' && (
          <>
            <h3 className="confirm-dialog-title">Revert: {actionLabel(entry)}</h3>
            <p className="confirm-dialog-message">
              This will revert the change recorded below. The revert is itself audit-logged.
            </p>
            <div className="revert-dialog-preview">
              <AuditDiff oldValue={entry.old_value} newValue={entry.new_value} />
            </div>
            <div className="confirm-dialog-actions">
              <button className="btn btn-secondary" onClick={onClose} disabled={busy}>
                Cancel
              </button>
              <button className="btn btn-primary" onClick={() => doRevert(false)} disabled={busy}>
                {busy ? 'Reverting…' : 'Revert'}
              </button>
            </div>
          </>
        )}
        {phase === 'race' && (
          <>
            <h3 className="confirm-dialog-title">Resource has changed</h3>
            <p className="confirm-dialog-message" style={{ color: 'var(--color-warning, #d97706)' }}>
              This entry&apos;s resource has been modified since the change was made.
              Reverting will overwrite the newer change.
            </p>
            <div className="confirm-dialog-actions">
              <button className="btn btn-secondary" onClick={onClose} disabled={busy}>
                Cancel
              </button>
              <button className="btn btn-danger" onClick={() => doRevert(true)} disabled={busy}>
                {busy ? 'Reverting…' : 'Revert anyway'}
              </button>
            </div>
          </>
        )}
        {phase === 'error' && (
          <>
            <h3 className="confirm-dialog-title">Revert failed</h3>
            <p className="confirm-dialog-message">{errorMsg}</p>
            <div className="confirm-dialog-actions">
              <button className="btn btn-secondary" onClick={onClose}>
                Close
              </button>
              <button
                className="btn btn-primary"
                onClick={() => {
                  setPhase('confirm');
                  setErrorMsg('');
                }}
              >
                Try again
              </button>
            </div>
          </>
        )}
      </div>
    </dialog>
  );
}
