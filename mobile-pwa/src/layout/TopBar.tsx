import { Link, useParams } from 'react-router-dom';

export function TopBar() {
  const { orgSlug } = useParams<{ orgSlug: string }>();
  return (
    <header className="m-top-bar">
      {orgSlug ? (
        <Link to="/orgs" className="m-org-chip" aria-label="Switch organization">
          <span aria-hidden>●</span>
          {orgSlug}
        </Link>
      ) : null}
    </header>
  );
}
