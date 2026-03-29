import { useState, useEffect } from 'react';
import { NavLink, Link, useParams } from 'react-router-dom';
import { getMockApps } from '@/mocks/hierarchy';

export default function AppAccordion() {
  const { orgSlug, projectSlug, appSlug } = useParams();
  const [expandedApp, setExpandedApp] = useState<string | null>(null);

  // Auto-expand the app from the URL
  useEffect(() => {
    if (appSlug) setExpandedApp(appSlug);
  }, [appSlug]);

  if (!orgSlug || !projectSlug) return null;

  const apps = getMockApps(projectSlug);

  if (apps.length === 0) return null;

  const basePath = `/orgs/${orgSlug}/projects/${projectSlug}/apps`;

  const appNavItems = [
    { path: 'deployments', label: 'Deployments' },
    { path: 'releases', label: 'Releases' },
    { path: 'flags', label: 'Flags' },
    { path: 'settings', label: 'Settings' },
  ];

  function toggleApp(slug: string) {
    setExpandedApp(expandedApp === slug ? null : slug);
  }

  return (
    <div className="app-accordion">
      <div className="sidebar-section">Applications</div>
      {apps.map((app) => {
        const isExpanded = expandedApp === app.slug;
        return (
          <div key={app.id} className="app-accordion-item">
            <button
              className={`app-accordion-header${app.slug === appSlug ? ' active' : ''}`}
              onClick={() => toggleApp(app.slug)}
            >
              <span className="app-accordion-arrow">{isExpanded ? '\u25BE' : '\u25B8'}</span>
              <span>{app.name}</span>
            </button>
            {isExpanded && (
              <div className="app-accordion-body">
                {appNavItems.map((item) => (
                  <NavLink
                    key={item.path}
                    to={`${basePath}/${app.slug}/${item.path}`}
                    className={({ isActive }) => `nav-item nav-item-nested${isActive ? ' active' : ''}`}
                  >
                    {item.label}
                  </NavLink>
                ))}
              </div>
            )}
          </div>
        );
      })}
      <Link
        to={`/orgs/${orgSlug}/projects/${projectSlug}/apps/new`}
        className="add-app-link"
      >
        + Add App
      </Link>
    </div>
  );
}
