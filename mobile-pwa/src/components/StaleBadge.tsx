interface StaleBadgeProps {
  lastSuccess: number | null;
  inflight: boolean;
  /** How old data must be before we mark it as stale. Defaults to 30s. */
  thresholdMs?: number;
}

function relativeAgo(ms: number): string {
  const sec = Math.max(0, Math.floor(ms / 1000));
  if (sec < 60) return `${sec}s ago`;
  const min = Math.floor(sec / 60);
  if (min < 60) return `${min}m ago`;
  const hr = Math.floor(min / 60);
  if (hr < 24) return `${hr}h ago`;
  const day = Math.floor(hr / 24);
  return `${day}d ago`;
}

/**
 * Small pill rendered when a screen is showing data older than `thresholdMs`.
 * On the first ever load (`lastSuccess === null`) we render nothing — implying
 * staleness with no prior success would be misleading.
 *
 * `data-refreshing="true"` is set when a refresh fetch is in flight, giving CSS
 * a hook for a subtle "refreshing" indicator.
 */
export function StaleBadge({ lastSuccess, inflight, thresholdMs = 30_000 }: StaleBadgeProps) {
  if (lastSuccess === null) return null;
  const age = Date.now() - lastSuccess;
  if (age < thresholdMs) return null;

  return (
    <span
      className="m-stale-badge"
      role="status"
      aria-live="polite"
      data-refreshing={inflight ? 'true' : undefined}
    >
      Showing data from {relativeAgo(age)}
    </span>
  );
}
