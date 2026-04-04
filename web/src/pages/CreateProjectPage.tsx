import { useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { entitiesApi } from '@/api';

export default function CreateProjectPage() {
  const { orgSlug } = useParams();
  const navigate = useNavigate();
  const [name, setName] = useState('');
  const [slug, setSlug] = useState('');
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);

  if (!orgSlug) return null;

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
      await entitiesApi.createProject(orgSlug!, { name, slug });
      localStorage.setItem('ds_last_project', slug);
      navigate(`/orgs/${orgSlug}/projects/${slug}/flags`);
    } catch (err: unknown) {
      const errorMessage = err instanceof Error ? err.message : 'Failed to create project';
      setError(errorMessage);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="page-center">
      <div className="form-card">
        <h1 className="page-header">Create Project</h1>
        {error && <div className="form-error">{error}</div>}
        <form onSubmit={handleSubmit}>
          <div className="form-group">
            <label className="form-label">Project Name</label>
            <input type="text" className="form-input" value={name}
              onChange={(e) => handleNameChange(e.target.value)} placeholder="My Project" required />
          </div>
          <div className="form-group">
            <label className="form-label">Slug</label>
            <input type="text" className="form-input" value={slug}
              onChange={(e) => setSlug(e.target.value)} placeholder="my-project" required />
          </div>
          <button type="submit" className="btn btn-primary" style={{ width: '100%' }}
            disabled={submitting}>
            {submitting ? 'Creating...' : 'Create Project'}
          </button>
        </form>
      </div>
    </div>
  );
}
