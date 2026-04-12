import { useState, useEffect } from 'react';
import { useParams } from 'react-router-dom';
import type { Member } from '@/types';
import { membersApi } from '@/api';

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' });
}

export default function MembersPage() {
  const { orgSlug } = useParams<{ orgSlug: string }>();
  const [activeTab, setActiveTab] = useState<'members' | 'groups'>('members');
  const [members, setMembers] = useState<Member[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  // Add member form
  const [newEmail, setNewEmail] = useState('');
  const [newRole, setNewRole] = useState<string>('member');

  // Delete confirm
  const [confirmDelete, setConfirmDelete] = useState<string | null>(null);

  const fetchMembers = useCallback(async () => {
    if (!orgSlug) return;
    setLoading(true);
    setError(null);
    try {
      const result = await membersApi.listByOrg(orgSlug);
      setMembers(result.members);
    } catch (err: unknown) {
      const errorMessage = err instanceof Error ? err.message : 'Failed to load members';
      setError(errorMessage);
    } finally {
      setLoading(false);
    }
  }, [orgSlug]);

  useEffect(() => {
    fetchMembers();
  }, [fetchMembers]);

  async function handleAddMember() {
    if (!newEmail.trim() || !orgSlug) return;
    setActionError(null);
    try {
      const result = await membersApi.addToOrg(orgSlug, newEmail.trim(), newRole);
      setMembers((prev) => [...prev, result.member]);
      setNewEmail('');
      setNewRole('member');
    } catch (err: unknown) {
      const errorMessage = err instanceof Error ? err.message : 'Failed to add member';
      setActionError(errorMessage);
    }
  }

  async function handleChangeRole(userId: string, role: string) {
    if (!orgSlug) return;
    setActionError(null);
    try {
      await membersApi.updateOrgRole(orgSlug, userId, role);
      setMembers((prev) => prev.map((m) => (m.user_id === userId ? { ...m, role: role as Member['role'] } : m)));
    } catch (err: unknown) {
      const errorMessage = err instanceof Error ? err.message : 'Failed to update role';
      setActionError(errorMessage);
    }
  }

  async function handleRemoveMember(userId: string) {
    if (!orgSlug) return;
    setActionError(null);
    try {
      await membersApi.removeFromOrg(orgSlug, userId);
      setMembers((prev) => prev.filter((m) => m.user_id !== userId));
      setConfirmDelete(null);
    } catch (err: unknown) {
      const errorMessage = err instanceof Error ? err.message : 'Failed to remove member';
      setActionError(errorMessage);
      setConfirmDelete(null);
    }
  }

  return (
    <div className="page-content">
      <h2>Members &amp; Groups</h2>

      <div className="detail-tabs">
        <button
          className={`detail-tab${activeTab === 'members' ? ' active' : ''}`}
          onClick={() => setActiveTab('members')}
        >
          Members
        </button>
        <button
          className={`detail-tab${activeTab === 'groups' ? ' active' : ''}`}
          onClick={() => setActiveTab('groups')}
        >
          Groups
        </button>
      </div>

      {/* ---- Members Tab ---- */}
      {activeTab === 'members' && (
        <div>
          <div className="inline-form-row" style={{ marginBottom: 16 }}>
            <input
              type="email"
              placeholder="Email address"
              value={newEmail}
              onChange={(e) => setNewEmail(e.target.value)}
            />
            <select value={newRole} onChange={(e) => setNewRole(e.target.value)}>
              <option value="member">Member</option>
              <option value="admin">Admin</option>
              <option value="viewer">Viewer</option>
            </select>
            <button className="btn btn-primary" onClick={handleAddMember}>
              Add
            </button>
          </div>

          {error && <p className="form-error" style={{ marginBottom: 8 }}>{error}</p>}
          {actionError && <p className="form-error" style={{ marginBottom: 8 }}>{actionError}</p>}

          {loading ? (
            <p className="text-muted">Loading members...</p>
          ) : members.length === 0 ? (
            <p className="empty-state">No members yet. Add one above.</p>
          ) : (
            <table className="data-table">
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Email</th>
                  <th>Org Role</th>
                  <th>Joined</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>
                {members.map((m) => (
                  <tr key={m.id}>
                    <td>{m.name}</td>
                    <td>{m.email}</td>
                    <td>
                      <span className={`badge badge-${m.role}`}>{m.role}</span>
                    </td>
                    <td>{formatDate(m.joined_at)}</td>
                    <td>
                      <div style={{ display: 'flex', gap: 8, alignItems: 'center', flexWrap: 'wrap' }}>
                        {m.role === 'owner' ? (
                          <span className="text-muted">Owner</span>
                        ) : (
                          <select
                            value={m.role}
                            onChange={(e) => handleChangeRole(m.user_id, e.target.value)}
                          >
                            <option value="admin">Admin</option>
                            <option value="member">Member</option>
                            <option value="viewer">Viewer</option>
                          </select>
                        )}

                        {confirmDelete === m.user_id ? (
                          <span className="inline-confirm">
                            Are you sure?{' '}
                            <button className="btn btn-sm btn-danger" onClick={() => handleRemoveMember(m.user_id)}>
                              Yes
                            </button>{' '}
                            <button className="btn btn-sm" onClick={() => setConfirmDelete(null)}>
                              No
                            </button>
                          </span>
                        ) : (
                          <button className="btn btn-sm btn-danger" onClick={() => setConfirmDelete(m.user_id)}>
                            Remove
                          </button>
                        )}
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      )}

      {/* ---- Groups Tab ---- */}
      {activeTab === 'groups' && (
        <div>
          <p className="empty-state">Groups management coming soon.</p>
        </div>
      )}
    </div>
  );
}
