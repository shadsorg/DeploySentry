import { Link, useParams } from 'react-router-dom';

/**
 * Placeholder — the full chronological filter + table ships in Phase 3.
 *
 * Phase 2 registers the route so deep-links from the Status page's
 * per-app "History" link don't 404; the page invites the user to use the
 * per-app deployments page in the meantime.
 */
export default function OrgDeploymentsPage() {
  const { orgSlug } = useParams();
  return (
    <div className="page-container">
      <div className="page-header">
        <h1>Deploy History</h1>
        <p>Chronological view across every application in this org.</p>
      </div>
      <div className="card">
        <p>
          The filterable org-wide history list is landing in a follow-up release. Until then,
          open the deploy history for a specific application from the{' '}
          <Link to={`/orgs/${orgSlug}/status`}>Status page</Link>.
        </p>
      </div>
    </div>
  );
}
