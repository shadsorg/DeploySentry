import { useNavigate, useLocation, useParams } from 'react-router-dom';

const TABS = [
  { key: 'status', label: 'Status', icon: '●' },
  { key: 'history', label: 'History', icon: '▦' },
  { key: 'flags', label: 'Flags', icon: '⚑' },
] as const;

type TabKey = (typeof TABS)[number]['key'];

function activeTab(pathname: string, orgSlug?: string): TabKey | null {
  if (!orgSlug) return null;
  const prefix = `/orgs/${orgSlug}/`;
  if (!pathname.startsWith(prefix)) return null;
  const rest = pathname.slice(prefix.length).split('/')[0];
  if (rest === 'status' || rest === 'history' || rest === 'flags') return rest;
  return null;
}

export function TabBar() {
  const { orgSlug } = useParams<{ orgSlug: string }>();
  const nav = useNavigate();
  const loc = useLocation();
  const current = activeTab(loc.pathname, orgSlug);
  if (!orgSlug) return null;

  return (
    <nav className="m-tab-bar" aria-label="Primary">
      {TABS.map((t) => (
        <button
          key={t.key}
          type="button"
          aria-current={current === t.key ? 'page' : undefined}
          onClick={() => nav(`/orgs/${orgSlug}/${t.key}`)}
        >
          <span className="m-tab-icon" aria-hidden>{t.icon}</span>
          <span className="m-tab-label">{t.label}</span>
        </button>
      ))}
    </nav>
  );
}
