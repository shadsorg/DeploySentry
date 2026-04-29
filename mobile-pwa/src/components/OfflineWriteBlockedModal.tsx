interface OfflineWriteBlockedModalProps {
  open: boolean;
  onClose: () => void;
}

export function OfflineWriteBlockedModal({ open, onClose }: OfflineWriteBlockedModalProps) {
  if (!open) return null;
  return (
    <div className="m-offline-modal" role="presentation">
      <div
        className="m-offline-modal-backdrop"
        onClick={onClose}
        data-testid="m-offline-modal-backdrop"
      />
      <div
        className="m-offline-modal-card"
        role="alertdialog"
        aria-labelledby="m-offline-modal-title"
      >
        <h2 id="m-offline-modal-title" className="m-offline-modal-title">
          You're offline
        </h2>
        <p className="m-offline-modal-body">Connect to make changes.</p>
        <button
          type="button"
          className="m-button m-button-primary m-offline-modal-action"
          onClick={onClose}
        >
          Got it
        </button>
      </div>
    </div>
  );
}
