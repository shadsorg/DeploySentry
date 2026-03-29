import { useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { MOCK_ENVIRONMENTS } from '@/mocks/hierarchy';

export default function CreateAppPage() {
  const { orgSlug, projectSlug } = useParams();
  const navigate = useNavigate();
  const [name, setName] = useState('');
  const [slug, setSlug] = useState('');
  const [description, setDescription] = useState('');
  const [repoUrl, setRepoUrl] = useState('');

  function handleNameChange(value: string) {
    setName(value);
    setSlug(value.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, ''));
  }

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!name || !slug) return;
    navigate(`/orgs/${orgSlug}/projects/${projectSlug}/apps/${slug}/deployments`);
  }

  const backPath = `/orgs/${orgSlug}/projects/${projectSlug}/flags`;

  return (
    <div>
      <div className="page-header-row">
        <h1 className="page-header">Create Application</h1>
      </div>

      <div className="card" style={{ maxWidth: 600 }}>
        <form onSubmit={handleSubmit}>
          <div className="form-group">
            <label className="form-label">Application Name</label>
            <input
              type="text"
              className="form-input"
              value={name}
              onChange={(e) => handleNameChange(e.target.value)}
              placeholder="API Server"
              required
            />
          </div>
          <div className="form-group">
            <label className="form-label">Slug</label>
            <input
              type="text"
              className="form-input"
              value={slug}
              onChange={(e) => setSlug(e.target.value)}
              placeholder="api-server"
              required
            />
            <div className="form-hint">URL-safe identifier. Auto-generated from name.</div>
          </div>
          <div className="form-group">
            <label className="form-label">Description</label>
            <textarea
              className="form-input"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Core REST API for the platform"
              rows={3}
            />
          </div>
          <div className="form-group">
            <label className="form-label">Repository URL (optional)</label>
            <input
              type="text"
              className="form-input"
              value={repoUrl}
              onChange={(e) => setRepoUrl(e.target.value)}
              placeholder="https://github.com/acme/api-server"
            />
          </div>
          <div className="form-group">
            <label className="form-label">Environments</label>
            <div className="form-hint" style={{ marginBottom: 8 }}>
              This application will inherit all org-level environments:
            </div>
            <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
              {MOCK_ENVIRONMENTS.map((env) => (
                <span key={env.id} className={`badge ${env.is_production ? 'badge-production' : 'badge-ops'}`}>
                  {env.name}
                </span>
              ))}
            </div>
          </div>
          <div style={{ display: 'flex', gap: 8 }}>
            <button type="submit" className="btn btn-primary">Create Application</button>
            <button type="button" className="btn btn-secondary" onClick={() => navigate(backPath)}>
              Cancel
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
