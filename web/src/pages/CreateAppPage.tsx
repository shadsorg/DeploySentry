import { useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { entitiesApi } from '@/api';

export default function CreateAppPage() {
  const { orgSlug, projectSlug } = useParams();
  const navigate = useNavigate();
  const [name, setName] = useState('');
  const [slug, setSlug] = useState('');
  const [description, setDescription] = useState('');
  const [repoUrl, setRepoUrl] = useState('');
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);

  function handleNameChange(value: string) {
    setName(value);
    setSlug(
      value
        .toLowerCase()
        .replace(/[^a-z0-9]+/g, '-')
        .replace(/^-|-$/g, ''),
    );
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!name || !slug || !orgSlug || !projectSlug) return;
    setSubmitting(true);
    setError('');
    try {
      await entitiesApi.createApp(orgSlug, projectSlug, { name, slug, description });
      localStorage.setItem('ds_last_app', slug);
      navigate(`/orgs/${orgSlug}/projects/${projectSlug}/apps/${slug}/deployments`);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create application');
    } finally {
      setSubmitting(false);
    }
  }

  const backPath = `/orgs/${orgSlug}/projects/${projectSlug}/flags`;

  return (
    <div>
      <div className="page-header-row" style={{ marginBottom: 24 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
          <div
            style={{
              width: 36,
              height: 36,
              borderRadius: 10,
              background: 'var(--color-primary-bg)',
              border: '1px solid rgba(99,102,241,0.25)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              flexShrink: 0,
            }}
          >
            <span className="ms" style={{ fontSize: 20, color: 'var(--color-primary)' }}>
              apps
            </span>
          </div>
          <div className="page-header" style={{ marginBottom: 0 }}>
            <h1
              style={{
                fontFamily: 'var(--font-display)',
                fontWeight: 800,
                letterSpacing: '-0.02em',
              }}
            >
              Create Application
            </h1>
            <p>Add a new application to this project.</p>
          </div>
        </div>
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
            <p className="text-muted text-sm">
              Environments will be inherited from the organization.
            </p>
          </div>
          {error && (
            <div className="form-error" style={{ marginBottom: 8 }}>
              {error}
            </div>
          )}
          <div style={{ display: 'flex', gap: 8 }}>
            <button
              type="submit"
              className="btn btn-primary"
              disabled={submitting}
              style={{ display: 'inline-flex', alignItems: 'center', gap: 6 }}
            >
              <span className="ms" style={{ fontSize: 16 }}>
                {submitting ? 'hourglass_empty' : 'add_circle'}
              </span>
              {submitting ? 'Creating...' : 'Create Application'}
            </button>
            <button
              type="button"
              className="btn btn-secondary"
              onClick={() => navigate(backPath)}
              style={{ display: 'inline-flex', alignItems: 'center', gap: 6 }}
            >
              <span className="ms" style={{ fontSize: 16 }}>
                close
              </span>
              Cancel
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
