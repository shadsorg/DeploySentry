import { useState, useEffect } from 'react';
import { Link, useParams, useNavigate } from 'react-router-dom';
import type { FlagType, FlagCategory } from '@/types';
import { flagsApi, entitiesApi } from '@/api';
import { useStagingEnabled } from '@/hooks/useStagingEnabled';
import { stageOrCall } from '@/hooks/stageOrCall';
import { newProvisionalId } from '@/lib/provisional';

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
  const navigate = useNavigate();
  const backPath = appSlug
    ? `/orgs/${orgSlug}/projects/${projectSlug}/apps/${appSlug}/flags`
    : `/orgs/${orgSlug}/projects/${projectSlug}/flags`;

  const stagingEnabled = useStagingEnabled(orgSlug);

  const [form, setForm] = useState<FormState>({
    ...INITIAL,
    category: appSlug ? 'release' : 'feature',
  });
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [projectId, setProjectId] = useState<string | null>(null);
  const [appId, setAppId] = useState<string | null>(null);

  useEffect(() => {
    if (!orgSlug || !projectSlug) return;
    entitiesApi
      .getProject(orgSlug, projectSlug)
      .then((p) => setProjectId(p.id))
      .catch(() => {});
  }, [orgSlug, projectSlug]);

  // Resolve appId only when the URL puts us at app level. Project-level
  // creation must leave application_id null so the flag is project-scoped
  // (visible to every app under the project). Earlier code fell back to
  // the first app, silently turning project-scoped intent into an
  // app-scoped flag pinned to one sibling.
  useEffect(() => {
    if (!orgSlug || !projectSlug || !appSlug) {
      setAppId(null);
      return;
    }
    entitiesApi
      .getApp(orgSlug, projectSlug, appSlug)
      .then((a) => setAppId(a.id))
      .catch(() => setAppId(null));
  }, [orgSlug, projectSlug, appSlug]);

  const set = <K extends keyof FormState>(field: K, value: FormState[K]) =>
    setForm((prev) => ({ ...prev, [field]: value }));

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!projectId) {
      setError('Project not found');
      return;
    }
    setError(null);
    setSubmitting(true);
    try {
      const payload = {
        project_id: projectId,
        application_id: appId || undefined,
        key: form.key,
        name: form.name,
        description: form.description,
        flag_type: form.flag_type,
        category: form.category,
        purpose: form.purpose,
        owners: form.owners.split(',').map((s) => s.trim()).filter(Boolean),
        is_permanent: form.is_permanent,
        expires_at: form.is_permanent
          ? undefined
          : form.expires_at
            ? form.expires_at + ':00Z'
            : undefined,
        default_value: form.default_value,
        tags: form.tags.split(',').map((s) => s.trim()).filter(Boolean),
      };

      const provisionalId = newProvisionalId();
      const result = await stageOrCall({
        staged: stagingEnabled,
        orgSlug: orgSlug!,
        stage: {
          resource_type: 'flag',
          action: 'create',
          provisional_id: provisionalId,
          new_value: payload,
        },
        direct: () => flagsApi.create(payload),
      });

      if (result.mode === 'staged') {
        navigate(backPath); // list page; banner / overlay surfaces the new pending row
      } else {
        navigate(backPath);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create flag');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div>
      <div className="page-header-row" style={{ marginBottom: 24 }}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
          <Link
            to={backPath}
            style={{
              display: 'inline-flex',
              alignItems: 'center',
              gap: 4,
              fontSize: 13,
              color: 'var(--color-text-muted)',
              textDecoration: 'none',
            }}
          >
            <span className="ms" style={{ fontSize: 14 }}>
              arrow_back
            </span>
            Flags
          </Link>
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
                toggle_on
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
                Create Feature Flag
              </h1>
              <p>Define a new flag with targeting rules and ownership.</p>
            </div>
          </div>
        </div>
      </div>

      <form onSubmit={handleSubmit}>
        <div className="card">
          {error && (
            <p className="form-error" style={{ marginBottom: 16 }}>
              {error}
            </p>
          )}

          <div className="form-row">
            <div className="form-group">
              <label className="form-label" htmlFor="flag-key">
                Key
              </label>
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
              <label className="form-label" htmlFor="flag-name">
                Name
              </label>
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
            <label className="form-label" htmlFor="flag-description">
              Description
            </label>
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
              <label className="form-label" htmlFor="flag-type">
                Flag Type
              </label>
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
              <label className="form-label" htmlFor="flag-category">
                Category
              </label>
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
            <label className="form-label" htmlFor="flag-purpose">
              Purpose
            </label>
            <textarea
              id="flag-purpose"
              className="form-textarea"
              rows={2}
              value={form.purpose}
              onChange={(e) => set('purpose', e.target.value)}
            />
          </div>

          <div className="form-group">
            <label className="form-label" htmlFor="flag-owners">
              Owners
            </label>
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
              <label className="form-label" htmlFor="flag-expires">
                Expires At
              </label>
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
            <label className="form-label" htmlFor="flag-default">
              Default Value
            </label>
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
            <label className="form-label" htmlFor="flag-tags">
              Tags
            </label>
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

        <div style={{ display: 'flex', gap: 12, marginTop: 20 }}>
          <button
            type="submit"
            className="btn btn-primary"
            disabled={submitting}
            style={{ display: 'inline-flex', alignItems: 'center', gap: 6 }}
          >
            <span className="ms" style={{ fontSize: 16 }}>
              {submitting ? 'hourglass_empty' : 'add_circle'}
            </span>
            {submitting ? 'Creating...' : 'Create Flag'}
          </button>
          <Link
            to={backPath}
            className="btn btn-secondary"
            style={{ display: 'inline-flex', alignItems: 'center', gap: 6 }}
          >
            <span className="ms" style={{ fontSize: 16 }}>
              close
            </span>
            Cancel
          </Link>
        </div>
      </form>
    </div>
  );
}
