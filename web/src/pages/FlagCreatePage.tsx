import { useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import type { FlagType, FlagCategory } from '@/types';

interface FormState {
  key: string;
  name: string;
  description: string;
  flag_type: FlagType;
  category: FlagCategory;
  purpose: string;
  owners: string;
  is_permanent: boolean;
  expires_at: string;
  default_value: string;
  tags: string;
}

const INITIAL: FormState = {
  key: '',
  name: '',
  description: '',
  flag_type: 'boolean',
  category: 'feature',
  purpose: '',
  owners: '',
  is_permanent: false,
  expires_at: '',
  default_value: '',
  tags: '',
};

export default function FlagCreatePage() {
  const { orgSlug, projectSlug, appSlug } = useParams();
  const backPath = appSlug
    ? `/orgs/${orgSlug}/projects/${projectSlug}/apps/${appSlug}/flags`
    : `/orgs/${orgSlug}/projects/${projectSlug}/flags`;

  const [form, setForm] = useState<FormState>({ ...INITIAL, category: appSlug ? 'release' : 'feature' });

  const set = <K extends keyof FormState>(field: K, value: FormState[K]) =>
    setForm((prev) => ({ ...prev, [field]: value }));

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    const payload = {
      project_id: 'proj-1',
      environment_id: 'env-prod',
      key: form.key,
      name: form.name,
      description: form.description,
      flag_type: form.flag_type,
      category: form.category,
      purpose: form.purpose,
      owners: form.owners
        .split(',')
        .map((s) => s.trim())
        .filter(Boolean),
      is_permanent: form.is_permanent,
      expires_at: form.is_permanent ? undefined : form.expires_at || undefined,
      default_value: form.default_value,
      tags: form.tags
        .split(',')
        .map((s) => s.trim())
        .filter(Boolean),
    };
    console.log('Create Flag payload:', payload);
  };

  return (
    <div>
      <h1 className="page-header">Create Feature Flag</h1>

      <form onSubmit={handleSubmit}>
        <div className="card">
          <div className="form-row">
            <div className="form-group">
              <label className="form-label" htmlFor="flag-key">Key</label>
              <input
                id="flag-key"
                className="form-input"
                type="text"
                required
                value={form.key}
                onChange={(e) => set('key', e.target.value)}
              />
              <span className="form-hint">Unique identifier, e.g. enable-dark-mode</span>
            </div>
            <div className="form-group">
              <label className="form-label" htmlFor="flag-name">Name</label>
              <input
                id="flag-name"
                className="form-input"
                type="text"
                required
                value={form.name}
                onChange={(e) => set('name', e.target.value)}
              />
            </div>
          </div>

          <div className="form-group">
            <label className="form-label" htmlFor="flag-description">Description</label>
            <textarea
              id="flag-description"
              className="form-textarea"
              rows={3}
              value={form.description}
              onChange={(e) => set('description', e.target.value)}
            />
          </div>

          <div className="form-row">
            <div className="form-group">
              <label className="form-label" htmlFor="flag-type">Flag Type</label>
              <select
                id="flag-type"
                className="form-select"
                value={form.flag_type}
                onChange={(e) => set('flag_type', e.target.value as FlagType)}
              >
                <option value="boolean">Boolean</option>
                <option value="string">String</option>
                <option value="integer">Integer</option>
                <option value="json">JSON</option>
              </select>
            </div>
            <div className="form-group">
              <label className="form-label" htmlFor="flag-category">Category</label>
              <select
                id="flag-category"
                className="form-select"
                value={form.category}
                onChange={(e) => set('category', e.target.value as FlagCategory)}
              >
                <option value="release">Release</option>
                <option value="feature">Feature</option>
                <option value="experiment">Experiment</option>
                <option value="ops">Ops</option>
                <option value="permission">Permission</option>
              </select>
            </div>
          </div>

          <div className="form-group">
            <label className="form-label" htmlFor="flag-purpose">Purpose</label>
            <textarea
              id="flag-purpose"
              className="form-textarea"
              rows={2}
              value={form.purpose}
              onChange={(e) => set('purpose', e.target.value)}
            />
          </div>

          <div className="form-group">
            <label className="form-label" htmlFor="flag-owners">Owners</label>
            <input
              id="flag-owners"
              className="form-input"
              type="text"
              placeholder="frontend-team, design-team"
              value={form.owners}
              onChange={(e) => set('owners', e.target.value)}
            />
            <span className="form-hint">Comma-separated list of teams or individuals</span>
          </div>

          <div className="form-group">
            <label className="toggle">
              <input
                type="checkbox"
                checked={form.is_permanent}
                onChange={(e) => set('is_permanent', e.target.checked)}
              />
              <span>Permanent flag (no expiration)</span>
            </label>
          </div>

          {!form.is_permanent && (
            <div className="form-group">
              <label className="form-label" htmlFor="flag-expires">Expires At</label>
              <input
                id="flag-expires"
                className="form-input"
                type="datetime-local"
                value={form.expires_at}
                onChange={(e) => set('expires_at', e.target.value)}
              />
            </div>
          )}

          <div className="form-group">
            <label className="form-label" htmlFor="flag-default">Default Value</label>
            <input
              id="flag-default"
              className="form-input"
              type="text"
              required
              value={form.default_value}
              onChange={(e) => set('default_value', e.target.value)}
            />
          </div>

          <div className="form-group">
            <label className="form-label" htmlFor="flag-tags">Tags</label>
            <input
              id="flag-tags"
              className="form-input"
              type="text"
              placeholder="ui, experiment, rollout"
              value={form.tags}
              onChange={(e) => set('tags', e.target.value)}
            />
            <span className="form-hint">Comma-separated list of tags</span>
          </div>
        </div>

        <div style={{ display: 'flex', gap: '0.75rem', marginTop: '1rem' }}>
          <button type="submit" className="btn btn-primary">
            Create Flag
          </button>
          <Link to={backPath} className="btn btn-secondary">
            Cancel
          </Link>
        </div>
      </form>
    </div>
  );
}
