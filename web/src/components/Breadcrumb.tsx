import { Link, useParams, useLocation } from 'react-router-dom';

/** Map the last URL segment to a human-readable page name.
 *  For detail routes like /flags/:id, the last segment is an ID —
 *  detect this and use the second-to-last segment instead. */
function pageName(pathname: string): string {
  const segments = pathname.split('/').filter(Boolean);
  const names: Record<string, string> = {
    projects: 'Projects',
    flags: 'Flags',
    new: 'Create',
    deployments: 'Deployments',
    releases: 'Releases',
    analytics: 'Analytics',
    sdks: 'SDKs & Docs',
    settings: 'Settings',
    members: 'Members',
    'api-keys': 'API Keys',
  };

  const last = segments[segments.length - 1] ?? '';
  if (names[last]) return names[last];

  // If last segment is an ID (not in names map), check parent segment
  const parent = segments[segments.length - 2] ?? '';
  const detailNames: Record<string, string> = {
    flags: 'Flag Detail',
    deployments: 'Deployment Detail',
    releases: 'Release Detail',
  };
  return detailNames[parent] ?? last;
}

export default function Breadcrumb() {
  const { orgSlug, projectSlug, appSlug } = useParams();
  const location = useLocation();

  if (!orgSlug) return null;

  const segments: { label: string; to: string }[] = [];

  // Org
  segments.push({
    label: orgSlug,
    to: `/orgs/${orgSlug}/projects`,
  });

  // Project
  if (projectSlug) {
    segments.push({
      label: projectSlug,
      to: `/orgs/${orgSlug}/projects/${projectSlug}/flags`,
    });
  }

  // App
  if (appSlug) {
    segments.push({
      label: appSlug,
      to: `/orgs/${orgSlug}/projects/${projectSlug}/apps/${appSlug}/deployments`,
    });
  }

  // Current page name (not a link)
  const page = pageName(location.pathname);

  return (
    <nav className="breadcrumb">
      {segments.map((seg) => (
        <span key={seg.to}>
          <Link to={seg.to} className="breadcrumb-link">
            {seg.label}
          </Link>
          <span className="breadcrumb-sep">/</span>
        </span>
      ))}
      <span className="breadcrumb-current">{page}</span>
    </nav>
  );
}
