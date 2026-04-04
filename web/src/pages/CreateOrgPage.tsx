import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { entitiesApi } from '@/api';

export default function CreateOrgPage() {
  const navigate = useNavigate();
  const [name, setName] = useState('');
  const [slug, setSlug] = useState('');
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);

  function handleNameChange(value: string) {
    setName(value);
    setSlug(value.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, ''));
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
    } catch (err: unknown) {
      setError(err.message || 'Failed to create organization');
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="page-center">
      <div className="form-card">
        <h1 className="page-header">Create Organization</h1>
        {error && <div className="form-error">{error}</div>}
        <form onSubmit={handleSubmit}>
          <div className="form-group">
            <label className="form-label">Organization Name</label>
            <input type="text" className="form-input" value={name}
              onChange={(e) => handleNameChange(e.target.value)} placeholder="Acme Corp" required />
          </div>
          <div className="form-group">
            <label className="form-label">Slug</label>
            <input type="text" className="form-input" value={slug}
              onChange={(e) => setSlug(e.target.value)} placeholder="acme-corp" required />
          </div>
          <button type="submit" className="btn btn-primary" style={{ width: '100%' }}
            disabled={submitting}>
            {submitting ? 'Creating...' : 'Create Organization'}
          </button>
        </form>
      </div>
    </div>
  );
}
