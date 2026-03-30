import { useState, useEffect } from 'react';
import { useParams } from 'react-router-dom';
import type { Member } from '@/types';
import { entitiesApi, membersApi } from '@/api';

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' });
}

export default function MembersPage() {
  const { orgSlug } = useParams<{ orgSlug: string }>();
  const [activeTab, setActiveTab] = useState<'members' | 'groups'>('members');
  const [members, setMembers] = useState<Member[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Add member form
  const [newEmail, setNewEmail] = useState('');
  const [newRole, setNewRole] = useState<'owner' | 'member'>('member');

  // Delete confirm
  const [confirmDelete, setConfirmDelete] = useState<string | null>(null);

  useEffect(() => {
    if (!orgSlug) return;
    let cancelled = false;

    async function fetchMembers() {
      setLoading(true);
      setError(null);
      try {
        const org = await entitiesApi.getOrg(orgSlug!);
        const result = await membersApi.listByOrg(org.id);
        if (!cancelled) {
          setMembers(result.members as Member[]);
        }
      } catch (err: any) {
        if (!cancelled) {
          setError(err.message || 'Failed to load members');
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    }

    fetchMembers();
    return () => { cancelled = true; };
  }, [orgSlug]);

  // --- Member actions (client-only) ---

  function handleAddMember() {
    if (!newEmail.trim()) return;
    const member: Member = {
      id: `user-${Date.now()}`,
      name: newEmail.split('@')[0],
      email: newEmail.trim(),
      role: newRole,
      group_ids: [],
      joined_at: new Date().toISOString(),
    };
    setMembers((prev) => [...prev, member]);
    setNewEmail('');
    setNewRole('member');
  }

  function handleChangeRole(memberId: string, role: 'owner' | 'member') {
    setMembers((prev) => prev.map((m) => (m.id === memberId ? { ...m, role } : m)));
  }

  function handleRemoveMember(memberId: string) {
    setMembers((prev) => prev.filter((m) => m.id !== memberId));
    setConfirmDelete(null);
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
            <select value={newRole} onChange={(e) => setNewRole(e.target.value as 'owner' | 'member')}>
              <option value="member">Member</option>
              <option value="owner">Owner</option>
            </select>
            <button className="btn btn-primary" onClick={handleAddMember}>
              Add
            </button>
          </div>

          {error && <p className="form-error" style={{ marginBottom: 8 }}>{error}</p>}

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
                        <select
                          value={m.role}
                          onChange={(e) => handleChangeRole(m.id, e.target.value as 'owner' | 'member')}
                        >
                          <option value="member">Member</option>
                          <option value="owner">Owner</option>
                        </select>

                        {confirmDelete === m.id ? (
                          <span className="inline-confirm">
                            Are you sure?{' '}
                            <button className="btn btn-sm btn-danger" onClick={() => handleRemoveMember(m.id)}>
                              Yes
                            </button>{' '}
                            <button className="btn btn-sm" onClick={() => setConfirmDelete(null)}>
                              No
                            </button>
                          </span>
                        ) : (
                          <button className="btn btn-sm btn-danger" onClick={() => setConfirmDelete(m.id)}>
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
