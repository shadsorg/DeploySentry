import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { entitiesApi } from '@/api';
import SiteHeader from '@/components/SiteHeader';

export default function CreateOrgPage() {
  const navigate = useNavigate();
  const [name, setName] = useState('');
  const [slug, setSlug] = useState('');
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
    if (!name || !slug) return;
    setSubmitting(true);
    setError('');
    try {
      await entitiesApi.createOrg({ name, slug });
      localStorage.setItem('ds_last_org', slug);
      navigate(`/orgs/${slug}/projects`);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create organization');
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <>
      <SiteHeader variant="landing" size="large" />
      <div className="page-center">
        <div className="form-card">
          <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 20 }}>
            <div
              style={{
                width: 40,
                height: 40,
                borderRadius: 10,
                flexShrink: 0,
                background: 'var(--color-primary-bg)',
                border: '1px solid rgba(99,102,241,0.25)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
              }}
            >
              <span className="ms" style={{ fontSize: 22, color: 'var(--color-primary)' }}>
                corporate_fare
              </span>
            </div>
            <div>
              <h1
                style={{
                  fontFamily: 'var(--font-display)',
                  fontWeight: 800,
                  fontSize: 22,
                  letterSpacing: '-0.02em',
                  margin: 0,
                }}
              >
                Create Organization
              </h1>
              <p style={{ color: 'var(--color-text-muted)', fontSize: 13, marginTop: 2 }}>
                Set up a new workspace for your team.
              </p>
            </div>
          </div>
          {error && (
            <div className="form-error" style={{ marginBottom: 16 }}>
              {error}
            </div>
          )}
          <form onSubmit={handleSubmit}>
            <div className="form-group">
              <label className="form-label">Organization Name</label>
              <input
                type="text"
                className="form-input"
                value={name}
                onChange={(e) => handleNameChange(e.target.value)}
                placeholder="Acme Corp"
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
                placeholder="acme-corp"
                required
              />
            </div>
            <button
              type="submit"
              className="btn btn-primary btn-full"
              disabled={submitting}
              style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 6 }}
            >
              <span className="ms" style={{ fontSize: 16 }}>
                {submitting ? 'hourglass_empty' : 'add_circle'}
              </span>
              {submitting ? 'Creating...' : 'Create Organization'}
            </button>
          </form>
        </div>
      </div>
    </>
  );
}
