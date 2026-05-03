export type StagedMarker = {
  provisional_id?: string;
  action: string;
  staged_at: string;
};

type Props = {
  marker: StagedMarker | null | undefined;
};

/**
 * Small inline badge that surfaces a staged mutation on any list row or
 * detail header. Reads the `_staged` envelope emitted by the staging
 * read-overlay; absent on plain production rows. Hidden entirely when
 * marker is null/undefined so consumers can blindly pass `row._staged ?? null`.
 */
export function StagedBadge({ marker }: Props) {
  if (!marker) return null;
  const tooltip = `Staged ${marker.action} at ${new Date(marker.staged_at).toLocaleString()}`;
  return (
    <span
      className="inline-flex items-center rounded-md bg-amber-100 px-2 py-0.5 text-xs font-medium text-amber-800"
      title={tooltip}
    >
      pending {marker.action}
    </span>
  );
}
