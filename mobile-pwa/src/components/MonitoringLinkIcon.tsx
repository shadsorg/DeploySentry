import type { MonitoringLink } from '../types';

function glyph(icon?: string): string {
  switch (icon) {
    case 'grafana':
      return '📊';
    case 'sentry':
      return '🛡';
    case 'datadog':
      return '🐶';
    case 'pagerduty':
      return '🚨';
    case 'slack':
      return '💬';
    default:
      return '↗';
  }
}

export function MonitoringLinkIcon({ link }: { link: MonitoringLink }) {
  return (
    <a
      href={link.url}
      target="_blank"
      rel="noopener noreferrer"
      className="m-monitor-link"
      title={link.label}
      aria-label={link.label}
      onClick={(e) => e.stopPropagation()}
    >
      {glyph(link.icon)}
    </a>
  );
}
