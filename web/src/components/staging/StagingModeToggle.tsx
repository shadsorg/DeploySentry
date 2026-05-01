import { useStagingEnabled, setStagingEnabled } from '@/hooks/useStagingEnabled';

interface Props {
  orgSlug: string;
}

/**
 * Operator-facing on/off control for the per-org staging mode. Reads and
 * writes the same `ds_staging_enabled:<orgSlug>` localStorage key the
 * `useStagingEnabled` hook watches; flipping it broadcasts a custom event
 * so every same-tab consumer (the header banner, every converted page)
 * picks up the change without a refresh.
 *
 * Mounted on `DeployChangesPage` so operators have a discoverable surface
 * without needing DevTools. A future "real" feature-flag-backed toggle
 * (Settings tab) can replace this without changing the API.
 */
export default function StagingModeToggle({ orgSlug }: Props) {
  const enabled = useStagingEnabled(orgSlug);
  const onToggle = () => setStagingEnabled(orgSlug, !enabled);

  return (
    <div
      className={`staging-mode-toggle ${enabled ? 'on' : 'off'}`}
      data-testid="staging-mode-toggle"
    >
      <div className="staging-mode-toggle-text">
        <strong>Staging mode is {enabled ? 'on' : 'off'}</strong>
        <p>
          {enabled
            ? 'New dashboard mutations queue here for review. Existing direct writes from SDKs, CLI, and webhooks are unaffected.'
            : 'Dashboard mutations write to production immediately. Turn on to queue them for review and one-click deploy.'}
        </p>
      </div>
      <button
        type="button"
        className={enabled ? 'btn-secondary danger' : 'btn-primary'}
        onClick={onToggle}
      >
        {enabled ? 'Disable staging' : 'Enable staging'}
      </button>
    </div>
  );
}
