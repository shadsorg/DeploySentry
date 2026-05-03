import { useState, useEffect, useCallback, useMemo } from 'react';
import { useParams } from 'react-router-dom';
import type { Member } from '@/types';
import { membersApi, groupsApi } from '@/api';
import type { Group } from '@/api';
import { useGroups, useGroupMembers } from '@/hooks/useGroups';
import { useStagingEnabled } from '@/hooks/useStagingEnabled';
import { stageOrCall } from '@/hooks/stageOrCall';
import RecentMemberActivity from '@/components/members/RecentMemberActivity';

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  });
}

export default function MembersPage() {
  const { orgSlug } = useParams<{ orgSlug: string }>();
  const stagingEnabled = useStagingEnabled(orgSlug);
  const [activeTab, setActiveTab] = useState<'members' | 'groups'>('members');
  const [members, setMembers] = useState<Member[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  // Add member form
  const [newEmail, setNewEmail] = useState('');
  const [newRole, setNewRole] = useState<string>('member');

  // Role filter
  const [roleFilter, setRoleFilter] = useState<'all' | 'owner' | 'admin' | 'member' | 'viewer'>(
    'all',
  );

  // ⚡ Bolt: Pre-compute role counts to avoid O(N*M) complexity in the render loop.
  // We also memoize visibleMembers to prevent redundant O(N) array filtering on every render.
  const { roleCounts, visibleMembers } = useMemo(() => {
    const counts: Record<string, number> = {
      all: members.length,
      owner: 0,
      admin: 0,
      member: 0,
      viewer: 0,
    };
    for (const m of members) {
      if (counts[m.role] !== undefined) counts[m.role]++;
    }
    const visible = roleFilter === 'all' ? members : members.filter((m) => m.role === roleFilter);
    return { roleCounts: counts, visibleMembers: visible };
  }, [members, roleFilter]);

  // Delete confirm
  const [confirmDelete, setConfirmDelete] = useState<string | null>(null);

  // Groups state
  const {
    groups,
    loading: groupsLoading,
    error: groupsError,
    refresh: refreshGroups,
  } = useGroups(orgSlug);
  const [selectedGroup, setSelectedGroup] = useState<Group | null>(null);
  const {
    members: groupMembers,
    loading: groupMembersLoading,
    error: groupMembersError,
    refresh: refreshGroupMembers,
  } = useGroupMembers(orgSlug, selectedGroup?.slug);
  const [showCreateGroup, setShowCreateGroup] = useState(false);
  const [newGroupName, setNewGroupName] = useState('');
  const [newGroupDescription, setNewGroupDescription] = useState('');
  const [confirmDeleteGroup, setConfirmDeleteGroup] = useState<string | null>(null);
  const [newMemberUserId, setNewMemberUserId] = useState('');
  const [confirmRemoveGroupMember, setConfirmRemoveGroupMember] = useState<string | null>(null);
  const [groupActionError, setGroupActionError] = useState<string | null>(null);

  // ⚡ Bolt: Pre-compute available members to avoid O(N) array mapping, O(N) Set creation,
  // and O(N) array filtering during every render.
  const availableGroupMembers = useMemo(() => {
    if (activeTab !== 'groups' || !selectedGroup) return [];
    const groupMemberIds = new Set(groupMembers.map((gm) => gm.user_id));
    return members.filter((m) => !groupMemberIds.has(m.user_id));
  }, [members, groupMembers, activeTab, selectedGroup]);

  const fetchMembers = useCallback(async () => {
    if (!orgSlug) return;
    setLoading(true);
    setError(null);
    try {
      const result = await membersApi.listByOrg(orgSlug);
      setMembers(result.members);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load members');
    } finally {
      setLoading(false);
    }
  }, [orgSlug]);

  useEffect(() => {
    fetchMembers();
  }, [orgSlug, fetchMembers]);

  async function handleAddMember() {
    if (!newEmail.trim() || !orgSlug) return;
    setActionError(null);
    try {
      const result = await membersApi.addToOrg(orgSlug, newEmail.trim(), newRole);
      setMembers((prev) => [...prev, result.member]);
      setNewEmail('');
      setNewRole('member');
    } catch (err) {
      setActionError(err instanceof Error ? err.message : 'Failed to add member');
    }
  }

  async function handleChangeRole(userId: string, role: string) {
    if (!orgSlug) return;
    setActionError(null);
    const prior = members.find((m) => m.user_id === userId);
    try {
      await stageOrCall({
        staged: stagingEnabled,
        orgSlug,
        // Phase C-3 backend handler: SetRoleChanged dispatches
        // UpdateOrgMemberRole(orgID=row.OrgID, userID=row.ResourceID,
        // role=payload.role). resource_id is the user id; org_id is
        // already on every staged row.
        stage: {
          resource_type: 'member',
          resource_id: userId,
          action: 'role_changed',
          old_value: prior ? { role: prior.role } : undefined,
          new_value: { role },
        },
        direct: () => membersApi.updateOrgRole(orgSlug, userId, role),
      });
      setMembers((prev) =>
        prev.map((m) => (m.user_id === userId ? { ...m, role: role as Member['role'] } : m)),
      );
    } catch (err) {
      setActionError(err instanceof Error ? err.message : 'Failed to update role');
    }
  }

  async function handleRemoveMember(userId: string) {
    if (!orgSlug) return;
    setActionError(null);
    try {
      await membersApi.removeFromOrg(orgSlug, userId);
      setMembers((prev) => prev.filter((m) => m.user_id !== userId));
      setConfirmDelete(null);
    } catch (err) {
      setActionError(err instanceof Error ? err.message : 'Failed to remove member');
      setConfirmDelete(null);
    }
  }

  return (
    <div className="page-content">
      <div className="page-header-row">
        <div className="page-header" style={{ marginBottom: 0 }}>
          <h1>Members &amp; Groups</h1>
          <p>Manage your team's access levels across the organization.</p>
        </div>
        <button className="btn btn-primary" onClick={() => {}}>
          <span className="ms" style={{ fontSize: 16 }}>
            person_add
          </span>
          Invite Member
        </button>
      </div>

      <div className="detail-tabs" style={{ marginTop: 20 }}>
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
          {orgSlug && <RecentMemberActivity orgSlug={orgSlug} />}
          <div className="inline-form-row" style={{ marginBottom: 16 }}>
            <input
              type="email"
              className="form-input"
              placeholder="Email address"
              value={newEmail}
              onChange={(e) => setNewEmail(e.target.value)}
            />
            <select
              className="form-select"
              value={newRole}
              onChange={(e) => setNewRole(e.target.value)}
            >
              <option value="member">Member</option>
              <option value="admin">Admin</option>
              <option value="viewer">Viewer</option>
            </select>
            <button className="btn btn-primary" onClick={handleAddMember}>
              Add
            </button>
          </div>

          {error && (
            <p className="form-error" style={{ marginBottom: 8 }}>
              {error}
            </p>
          )}
          {actionError && (
            <p className="form-error" style={{ marginBottom: 8 }}>
              {actionError}
            </p>
          )}

          {!loading && members.length > 0 && (
            <div
              className="org-status-filter-pills"
              role="tablist"
              aria-label="Filter members by role"
              style={{ marginBottom: 12 }}
            >
              {(['all', 'owner', 'admin', 'member', 'viewer'] as const).map((role) => {
                const count = roleCounts[role] || 0;
                return (
                  <button
                    key={role}
                    type="button"
                    role="tab"
                    aria-selected={roleFilter === role}
                    className={`filter-pill ${roleFilter === role ? 'active' : ''}`}
                    onClick={() => setRoleFilter(role)}
                  >
                    {role === 'all' ? 'All' : role.charAt(0).toUpperCase() + role.slice(1)}{' '}
                    <span className="dim">{count}</span>
                  </button>
                );
              })}
            </div>
          )}

          {loading ? (
            <div className="empty-state">Loading members…</div>
          ) : members.length === 0 ? (
            <div className="empty-state card">
              <p>No members yet. Add one above.</p>
            </div>
          ) : (
            <div className="card" style={{ padding: 0, overflow: 'hidden' }}>
              <div
                style={{
                  padding: '12px 20px',
                  borderBottom: '1px solid var(--color-border)',
                  display: 'flex',
                  justifyContent: 'space-between',
                  alignItems: 'center',
                }}
              >
                <span style={{ fontFamily: 'var(--font-display)', fontWeight: 700, fontSize: 14 }}>
                  Team Directory
                </span>
                <span className="text-xs text-secondary">
                  {visibleMembers.length === members.length
                    ? `${members.length} member${members.length !== 1 ? 's' : ''}`
                    : `${visibleMembers.length} of ${members.length} member${members.length !== 1 ? 's' : ''}`}
                </span>
              </div>
              <table>
                <thead>
                  <tr>
                    <th>Member</th>
                    <th>Role</th>
                    <th>Joined</th>
                    <th>Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {visibleMembers.map((m) => (
                    <tr key={m.id}>
                      <td>
                        <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                          <div
                            style={{
                              width: 34,
                              height: 34,
                              borderRadius: '50%',
                              background: 'var(--color-primary-bg)',
                              border: '1px solid var(--color-border)',
                              display: 'flex',
                              alignItems: 'center',
                              justifyContent: 'center',
                              fontSize: 11,
                              fontWeight: 700,
                              color: 'var(--color-primary)',
                              flexShrink: 0,
                            }}
                          >
                            {m.name
                              ? m.name
                                  .split(' ')
                                  .map((n) => n[0])
                                  .join('')
                                  .slice(0, 2)
                                  .toUpperCase()
                              : '?'}
                          </div>
                          <div>
                            <div style={{ fontWeight: 600, fontSize: 14 }}>{m.name}</div>
                            <div style={{ fontSize: 12, color: 'var(--color-text-muted)' }}>
                              {m.email}
                            </div>
                          </div>
                        </div>
                      </td>
                      <td>
                        <span className={`badge badge-${m.role}`}>{m.role}</span>
                      </td>
                      <td>{formatDate(m.joined_at)}</td>
                      <td>
                        <div
                          style={{
                            display: 'flex',
                            gap: 8,
                            alignItems: 'center',
                            flexWrap: 'wrap',
                          }}
                        >
                          {m.role === 'owner' ? (
                            <span className="text-muted">Owner</span>
                          ) : (
                            <select
                              className="form-select"
                              style={{ maxWidth: 120 }}
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
                              <button
                                className="btn btn-sm btn-danger"
                                onClick={() => handleRemoveMember(m.user_id)}
                              >
                                Yes
                              </button>
                              <button className="btn btn-sm" onClick={() => setConfirmDelete(null)}>
                                No
                              </button>
                            </span>
                          ) : (
                            <button
                              className="btn btn-sm btn-danger"
                              onClick={() => setConfirmDelete(m.user_id)}
                            >
                              <span className="ms" style={{ fontSize: 14 }}>
                                person_remove
                              </span>
                            </button>
                          )}
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      )}

      {/* ---- Groups Tab ---- */}
      {activeTab === 'groups' && (
        <div>
          {groupActionError && (
            <p className="form-error" style={{ marginBottom: 8 }}>
              {groupActionError}
            </p>
          )}

          {selectedGroup ? (
            /* --- Group Detail View --- */
            <div>
              <button
                className="btn btn-sm"
                style={{ marginBottom: 16 }}
                onClick={() => setSelectedGroup(null)}
              >
                &larr; Back to groups
              </button>
              <h3>{selectedGroup.name}</h3>
              {selectedGroup.description && (
                <p className="text-muted" style={{ marginBottom: 16 }}>
                  {selectedGroup.description}
                </p>
              )}

              {(() => {
                return availableGroupMembers.length > 0 ? (
                  <div className="inline-form-row" style={{ marginBottom: 16 }}>
                    <select
                      className="form-select"
                      value={newMemberUserId}
                      onChange={(e) => setNewMemberUserId(e.target.value)}
                    >
                      <option value="">Select a member...</option>
                      {availableGroupMembers.map((m) => (
                        <option key={m.user_id} value={m.user_id}>
                          {m.name} ({m.email})
                        </option>
                      ))}
                    </select>
                    <button
                      className="btn btn-primary"
                      disabled={!newMemberUserId}
                      onClick={async () => {
                        if (!newMemberUserId || !orgSlug) return;
                        setGroupActionError(null);
                        try {
                          await groupsApi.addMember(orgSlug, selectedGroup.slug, newMemberUserId);
                          setNewMemberUserId('');
                          refreshGroupMembers();
                          refreshGroups();
                        } catch (err) {
                          setGroupActionError(
                            err instanceof Error ? err.message : 'Failed to add member',
                          );
                        }
                      }}
                    >
                      Add Member
                    </button>
                  </div>
                ) : (
                  <p className="text-muted" style={{ marginBottom: 16 }}>
                    All organization members are already in this group.
                  </p>
                );
              })()}

              {groupMembersError && (
                <p className="form-error" style={{ marginBottom: 8 }}>
                  {groupMembersError}
                </p>
              )}

              {groupMembersLoading ? (
                <p className="text-muted">Loading members...</p>
              ) : groupMembers.length === 0 ? (
                <p className="empty-state">No members in this group yet.</p>
              ) : (
                <table className="data-table">
                  <thead>
                    <tr>
                      <th>Name</th>
                      <th>Email</th>
                      <th>Actions</th>
                    </tr>
                  </thead>
                  <tbody>
                    {groupMembers.map((gm) => (
                      <tr key={gm.user_id}>
                        <td>{gm.name}</td>
                        <td>{gm.email}</td>
                        <td>
                          {confirmRemoveGroupMember === gm.user_id ? (
                            <span className="inline-confirm">
                              Are you sure?{' '}
                              <button
                                className="btn btn-sm btn-danger"
                                onClick={async () => {
                                  if (!orgSlug) return;
                                  setGroupActionError(null);
                                  try {
                                    await groupsApi.removeMember(
                                      orgSlug,
                                      selectedGroup.slug,
                                      gm.user_id,
                                    );
                                    setConfirmRemoveGroupMember(null);
                                    refreshGroupMembers();
                                  } catch (err) {
                                    setGroupActionError(
                                      err instanceof Error
                                        ? err.message
                                        : 'Failed to remove member',
                                    );
                                    setConfirmRemoveGroupMember(null);
                                  }
                                }}
                              >
                                Yes
                              </button>{' '}
                              <button
                                className="btn btn-sm"
                                onClick={() => setConfirmRemoveGroupMember(null)}
                              >
                                No
                              </button>
                            </span>
                          ) : (
                            <button
                              className="btn btn-sm btn-danger"
                              onClick={() => setConfirmRemoveGroupMember(gm.user_id)}
                            >
                              Remove
                            </button>
                          )}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
            </div>
          ) : (
            /* --- Group List View --- */
            <div>
              {!showCreateGroup && (
                <button
                  className="btn btn-primary"
                  style={{ marginBottom: 16 }}
                  onClick={() => setShowCreateGroup(true)}
                >
                  Create Group
                </button>
              )}

              {showCreateGroup && (
                <div className="inline-form-row" style={{ marginBottom: 16 }}>
                  <input
                    type="text"
                    className="form-input"
                    placeholder="Group name"
                    value={newGroupName}
                    onChange={(e) => setNewGroupName(e.target.value)}
                  />
                  <input
                    type="text"
                    className="form-input"
                    placeholder="Description (optional)"
                    value={newGroupDescription}
                    onChange={(e) => setNewGroupDescription(e.target.value)}
                  />
                  <button
                    className="btn btn-primary"
                    onClick={async () => {
                      if (!newGroupName.trim() || !orgSlug) return;
                      setGroupActionError(null);
                      try {
                        await groupsApi.create(orgSlug, {
                          name: newGroupName.trim(),
                          description: newGroupDescription.trim() || undefined,
                        });
                        setNewGroupName('');
                        setNewGroupDescription('');
                        setShowCreateGroup(false);
                        refreshGroups();
                      } catch (err) {
                        setGroupActionError(
                          err instanceof Error ? err.message : 'Failed to create group',
                        );
                      }
                    }}
                  >
                    Create
                  </button>
                  <button
                    className="btn btn-sm"
                    onClick={() => {
                      setShowCreateGroup(false);
                      setNewGroupName('');
                      setNewGroupDescription('');
                    }}
                  >
                    Cancel
                  </button>
                </div>
              )}

              {groupsError && (
                <p className="form-error" style={{ marginBottom: 8 }}>
                  {groupsError}
                </p>
              )}

              {groupsLoading ? (
                <div className="empty-state">Loading groups…</div>
              ) : groups.length === 0 ? (
                <div className="empty-state card">
                  <p>No groups yet. Create one above.</p>
                </div>
              ) : (
                <div className="card" style={{ padding: 0, overflow: 'hidden' }}>
                  <table>
                    <thead>
                      <tr>
                        <th>Name</th>
                        <th>Description</th>
                        <th>Members</th>
                        <th>Actions</th>
                      </tr>
                    </thead>
                    <tbody>
                      {groups.map((g) => (
                        <tr key={g.id}>
                          <td>
                            <button
                              className="btn-link"
                              style={{ color: 'var(--color-primary)', fontWeight: 600 }}
                              onClick={() => setSelectedGroup(g)}
                            >
                              {g.name}
                            </button>
                          </td>
                          <td className="text-secondary">{g.description}</td>
                          <td>
                            <span
                              className="badge"
                              style={{
                                background: 'var(--color-bg-elevated)',
                                color: 'var(--color-text-secondary)',
                              }}
                            >
                              {g.member_count}
                            </span>
                          </td>
                          <td>
                            {confirmDeleteGroup === g.id ? (
                              <span className="inline-confirm">
                                <button
                                  className="btn btn-sm btn-danger"
                                  onClick={async () => {
                                    if (!orgSlug) return;
                                    setGroupActionError(null);
                                    try {
                                      await groupsApi.delete(orgSlug, g.slug);
                                      setConfirmDeleteGroup(null);
                                      refreshGroups();
                                    } catch (err) {
                                      setGroupActionError(
                                        err instanceof Error
                                          ? err.message
                                          : 'Failed to delete group',
                                      );
                                      setConfirmDeleteGroup(null);
                                    }
                                  }}
                                >
                                  Yes
                                </button>
                                <button
                                  className="btn btn-sm"
                                  onClick={() => setConfirmDeleteGroup(null)}
                                >
                                  No
                                </button>
                              </span>
                            ) : (
                              <button
                                className="btn btn-sm btn-danger"
                                onClick={() => setConfirmDeleteGroup(g.id)}
                              >
                                Delete
                              </button>
                            )}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
