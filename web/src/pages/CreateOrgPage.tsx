import { useState } from 'react';
import { useNavigate } from 'react-router-dom';

export default function CreateOrgPage() {
  const navigate = useNavigate();
  const [name, setName] = useState('');
  const [slug, setSlug] = useState('');

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!name || !slug) return;
    // Stub — in the future this calls orgsApi.create()
    localStorage.setItem('ds_last_org', slug);
    navigate(`/orgs/${slug}/projects`);
  }

  function handleNameChange(value: string) {
    setName(value);
    // Auto-generate slug from name
    setSlug(value.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, ''));
  }

  return (
    <div className="page-center">
      <div className="form-card">
        <h1 className="page-header">Create Organization</h1>
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
          <button type="submit" className="btn btn-primary" style={{ width: '100%' }}>
            Create Organization
          </button>
        </form>
      </div>
    </div>
  );
}
