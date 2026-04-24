import { useState } from 'react';
import { entitiesApi } from '@/api';
import type { MonitoringLink } from '@/types';

/** Curated icon allow-list. Must match the server's `AllowedMonitoringIcons`. */
const ICON_OPTIONS = [
  { value: '', label: '— none —' },
  { value: 'github', label: 'GitHub' },
  { value: 'datadog', label: 'Datadog' },
  { value: 'newrelic', label: 'New Relic' },
  { value: 'grafana', label: 'Grafana' },
  { value: 'pagerduty', label: 'PagerDuty' },
  { value: 'sentry', label: 'Sentry' },
  { value: 'slack', label: 'Slack' },
  { value: 'loki', label: 'Loki' },
  { value: 'prometheus', label: 'Prometheus' },
  { value: 'cloudwatch', label: 'CloudWatch' },
  { value: 'custom', label: 'Custom (favicon)' },
] as const;

const MAX_LINKS = 10;
const MAX_LABEL = 60;

interface Props {
  orgSlug: string;
  projectSlug: string;
  appSlug: string;
  initial: MonitoringLink[];
  onSaved?: (links: MonitoringLink[]) => void;
}

export default function MonitoringLinksEditor({
  orgSlug,
  projectSlug,
  appSlug,
  initial,
  onSaved,
}: Props) {
  const [links, setLinks] = useState<MonitoringLink[]>(initial.map((l) => ({ ...l })));
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);

  function update(i: number, patch: Partial<MonitoringLink>) {
    setLinks((prev) => prev.map((l, idx) => (idx === i ? { ...l, ...patch } : l)));
  }

  function remove(i: number) {
    setLinks((prev) => prev.filter((_, idx) => idx !== i));
  }

  function add() {
    if (links.length >= MAX_LINKS) return;
    setLinks((prev) => [...prev, { label: '', url: '', icon: '' }]);
  }

  function move(i: number, delta: -1 | 1) {
    const j = i + delta;
    if (j < 0 || j >= links.length) return;
    setLinks((prev) => {
      const next = [...prev];
      const [row] = next.splice(i, 1);
      next.splice(j, 0, row);
      return next;
    });
  }

  async function save() {
    setError(null);
    setSaved(false);
    setSaving(true);
    try {
      const payload = links
        .map((l) => ({
          label: l.label.trim(),
          url: l.url.trim(),
          icon: (l.icon || '').trim() || undefined,
        }))
        .filter((l) => l.label || l.url);
      const updated = await entitiesApi.updateAppMonitoringLinks(
        orgSlug,
        projectSlug,
        appSlug,
        payload,
      );
      setLinks(updated.monitoring_links ?? []);
      setSaved(true);
      onSaved?.(updated.monitoring_links ?? []);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="card" style={{ marginTop: 16 }}>
      <div className="card-header">
        <span className="card-title">Monitoring links</span>
        <span style={{ fontSize: 12, opacity: 0.6, marginLeft: 'auto' }}>
          {links.length} / {MAX_LINKS}
        </span>
      </div>
      <p style={{ fontSize: 13, opacity: 0.75, marginTop: -4 }}>
        Shown alongside this application on the org-level Status page. Links open in a new tab.
      </p>

      <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
        {links.map((link, i) => (
          <div
            key={i}
            style={{
              display: 'grid',
              gridTemplateColumns: '130px 1fr 1fr auto auto auto',
              gap: 8,
              alignItems: 'center',
            }}
          >
            <select
              className="form-input"
              value={link.icon ?? ''}
              onChange={(e) => update(i, { icon: e.target.value })}
            >
              {ICON_OPTIONS.map((o) => (
                <option key={o.value} value={o.value}>
                  {o.label}
                </option>
              ))}
            </select>
            <input
              className="form-input"
              type="text"
              placeholder="Label"
              value={link.label}
              maxLength={MAX_LABEL}
              onChange={(e) => update(i, { label: e.target.value })}
            />
            <input
              className="form-input"
              type="url"
              placeholder="https://…"
              value={link.url}
              onChange={(e) => update(i, { url: e.target.value })}
            />
            <button
              className="btn"
              type="button"
              onClick={() => move(i, -1)}
              disabled={i === 0}
              title="Move up"
              style={{ opacity: i === 0 ? 0.4 : 1 }}
            >
              ↑
            </button>
            <button
              className="btn"
              type="button"
              onClick={() => move(i, 1)}
              disabled={i === links.length - 1}
              title="Move down"
              style={{ opacity: i === links.length - 1 ? 0.4 : 1 }}
            >
              ↓
            </button>
            <button
              className="btn"
              type="button"
              onClick={() => remove(i)}
              title="Remove"
              style={{ color: 'var(--color-danger)' }}
            >
              ✕
            </button>
          </div>
        ))}
      </div>

      <div style={{ display: 'flex', gap: 8, alignItems: 'center', marginTop: 12 }}>
        <button
          className="btn"
          type="button"
          onClick={add}
          disabled={links.length >= MAX_LINKS}
        >
          + Add link
        </button>
        <button className="btn btn-primary" type="button" onClick={save} disabled={saving}>
          {saving ? 'Saving…' : 'Save'}
        </button>
        {saved && <span style={{ fontSize: 13, color: 'var(--color-success)' }}>Saved.</span>}
        {error && <span style={{ fontSize: 13, color: 'var(--color-danger)' }}>{error}</span>}
      </div>
    </div>
  );
}
