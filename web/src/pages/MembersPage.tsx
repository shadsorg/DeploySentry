import { useState, useRef, useEffect } from 'react';
import type { Member, Group, GroupRole } from '@/types';
import { MOCK_MEMBERS, MOCK_GROUPS, MOCK_ENVIRONMENTS, MOCK_APPLICATIONS, getEnvironmentName } from '@/mocks/hierarchy';

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' });
}

function appNameById(id: string): string {
  return MOCK_APPLICATIONS.find((a) => a.id === id)?.name ?? id;
}

export default function MembersPage() {
  const [activeTab, setActiveTab] = useState<'members' | 'groups'>('members');
  const [members, setMembers] = useState<Member[]>([...MOCK_MEMBERS]);
  const [groups, setGroups] = useState<Group[]>([...MOCK_GROUPS]);

  // Add member form
  const [newEmail, setNewEmail] = useState('');
  const [newRole, setNewRole] = useState<'owner' | 'member'>('member');

  // Create group form
  const [showCreateGroup, setShowCreateGroup] = useState(false);
  const [newGroupName, setNewGroupName] = useState('');
  const [newGroupRole, setNewGroupRole] = useState<GroupRole>('viewer');
  const [newGroupEnvIds, setNewGroupEnvIds] = useState<string[]>([]);
  const [newGroupAppIds, setNewGroupAppIds] = useState<string[]>([]);

  // Edit group
  const [editingGroupId, setEditingGroupId] = useState<string | null>(null);

  // Group management dropdown per member
  const [managingMemberId, setManagingMemberId] = useState<string | null>(null);

  // Delete confirm
  const [confirmDelete, setConfirmDelete] = useState<string | null>(null);

  // Close group-manage dropdown on outside click
  const groupManageRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (groupManageRef.current && !groupManageRef.current.contains(e.target as Node)) {
        setManagingMemberId(null);
      }
    }
    document.addEventListener('mousedown', handleClick);
    return () => document.removeEventListener('mousedown', handleClick);
  }, []);

  // --- Member actions ---

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
    setGroups((prev) =>
      prev.map((g) => ({
        ...g,
        member_ids: g.member_ids.filter((id) => id !== memberId),
      })),
    );
    setConfirmDelete(null);
  }

  function handleToggleGroupForMember(memberId: string, groupId: string) {
    setMembers((prev) =>
      prev.map((m) => {
        if (m.id !== memberId) return m;
        const inGroup = m.group_ids.includes(groupId);
        return {
          ...m,
          group_ids: inGroup ? m.group_ids.filter((id) => id !== groupId) : [...m.group_ids, groupId],
        };
      }),
    );
    setGroups((prev) =>
      prev.map((g) => {
        if (g.id !== groupId) return g;
        const hasMember = g.member_ids.includes(memberId);
        return {
          ...g,
          member_ids: hasMember ? g.member_ids.filter((id) => id !== memberId) : [...g.member_ids, memberId],
        };
      }),
    );
  }

  // --- Group actions ---

  function handleCreateGroup() {
    if (!newGroupName.trim()) return;
    const group: Group = {
      id: `grp-${Date.now()}`,
      name: newGroupName.trim(),
      role: newGroupRole,
      environment_ids: [...newGroupEnvIds],
      application_ids: [...newGroupAppIds],
      member_ids: [],
      created_at: new Date().toISOString(),
    };
    setGroups((prev) => [...prev, group]);
    setNewGroupName('');
    setNewGroupRole('viewer');
    setNewGroupEnvIds([]);
    setNewGroupAppIds([]);
    setShowCreateGroup(false);
  }

  function handleDeleteGroup(groupId: string) {
    setGroups((prev) => prev.filter((g) => g.id !== groupId));
    setMembers((prev) =>
      prev.map((m) => ({
        ...m,
        group_ids: m.group_ids.filter((id) => id !== groupId),
      })),
    );
    setConfirmDelete(null);
  }

  function handleSaveEditGroup(groupId: string, updated: Partial<Group>) {
    setGroups((prev) => prev.map((g) => (g.id === groupId ? { ...g, ...updated } : g)));
    setEditingGroupId(null);
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

          {members.length === 0 ? (
            <p className="empty-state">No members yet. Add one above.</p>
          ) : (
            <table className="data-table">
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Email</th>
                  <th>Org Role</th>
                  <th>Groups</th>
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
                    <td>
                      {m.group_ids.map((gid) => {
                        const g = groups.find((gr) => gr.id === gid);
                        return g ? (
                          <span key={gid} className="badge" style={{ marginRight: 4 }}>
                            {g.name}
                          </span>
                        ) : null;
                      })}
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

                        <div className="group-manage" ref={managingMemberId === m.id ? groupManageRef : undefined}>
                          <button
                            className="btn btn-sm"
                            onClick={() => setManagingMemberId(managingMemberId === m.id ? null : m.id)}
                          >
                            Groups
                          </button>
                          {managingMemberId === m.id && (
                            <div className="group-manage-dropdown">
                              {groups.map((g) => (
                                <label key={g.id} style={{ display: 'block', padding: '4px 0' }}>
                                  <input
                                    type="checkbox"
                                    checked={m.group_ids.includes(g.id)}
                                    onChange={() => handleToggleGroupForMember(m.id, g.id)}
                                  />{' '}
                                  {g.name}
                                </label>
                              ))}
                            </div>
                          )}
                        </div>

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
          <div style={{ marginBottom: 16 }}>
            <button className="btn btn-primary" onClick={() => setShowCreateGroup(!showCreateGroup)}>
              {showCreateGroup ? 'Cancel' : 'Create Group'}
            </button>
          </div>

          {showCreateGroup && (
            <div className="inline-form" style={{ marginBottom: 16 }}>
              <div className="inline-form-row">
                <input
                  type="text"
                  placeholder="Group name"
                  value={newGroupName}
                  onChange={(e) => setNewGroupName(e.target.value)}
                  required
                />
                <select value={newGroupRole} onChange={(e) => setNewGroupRole(e.target.value as GroupRole)}>
                  <option value="viewer">Viewer</option>
                  <option value="editor">Editor</option>
                  <option value="admin">Admin</option>
                </select>
              </div>
              <div className="checkbox-group">
                <label style={{ fontWeight: 600 }}>Environment Scope (leave unchecked for all)</label>
                {MOCK_ENVIRONMENTS.map((env) => (
                  <label key={env.id}>
                    <input
                      type="checkbox"
                      checked={newGroupEnvIds.includes(env.id)}
                      onChange={() =>
                        setNewGroupEnvIds((prev) =>
                          prev.includes(env.id) ? prev.filter((id) => id !== env.id) : [...prev, env.id],
                        )
                      }
                    />{' '}
                    {env.name}
                  </label>
                ))}
              </div>
              <div className="checkbox-group">
                <label style={{ fontWeight: 600 }}>Application Scope (leave unchecked for all)</label>
                {MOCK_APPLICATIONS.map((app) => (
                  <label key={app.id}>
                    <input
                      type="checkbox"
                      checked={newGroupAppIds.includes(app.id)}
                      onChange={() =>
                        setNewGroupAppIds((prev) =>
                          prev.includes(app.id) ? prev.filter((id) => id !== app.id) : [...prev, app.id],
                        )
                      }
                    />{' '}
                    {app.name}
                  </label>
                ))}
              </div>
              <button className="btn btn-primary" onClick={handleCreateGroup}>
                Create
              </button>
            </div>
          )}

          {groups.length === 0 ? (
            <p className="empty-state">No groups defined. Create one to manage access.</p>
          ) : (
            <table className="data-table">
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Role</th>
                  <th>Environments</th>
                  <th>Applications</th>
                  <th>Members</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>
                {groups.map((g) => (
                  <GroupRow
                    key={g.id}
                    group={g}
                    editingGroupId={editingGroupId}
                    confirmDelete={confirmDelete}
                    onEdit={() => setEditingGroupId(g.id)}
                    onCancelEdit={() => setEditingGroupId(null)}
                    onSaveEdit={(updated) => handleSaveEditGroup(g.id, updated)}
                    onDelete={() => handleDeleteGroup(g.id)}
                    onConfirmDelete={() => setConfirmDelete(g.id)}
                    onCancelDelete={() => setConfirmDelete(null)}
                  />
                ))}
              </tbody>
            </table>
          )}
        </div>
      )}
    </div>
  );
}

/* ------------------------------------------------------------------ */
/* Group row with inline edit                                         */
/* ------------------------------------------------------------------ */

interface GroupRowProps {
  group: Group;
  editingGroupId: string | null;
  confirmDelete: string | null;
  onEdit: () => void;
  onCancelEdit: () => void;
  onSaveEdit: (updated: Partial<Group>) => void;
  onDelete: () => void;
  onConfirmDelete: () => void;
  onCancelDelete: () => void;
}

function GroupRow({
  group,
  editingGroupId,
  confirmDelete,
  onEdit,
  onCancelEdit,
  onSaveEdit,
  onDelete,
  onConfirmDelete,
  onCancelDelete,
}: GroupRowProps) {
  const isEditing = editingGroupId === group.id;

  const [editName, setEditName] = useState(group.name);
  const [editRole, setEditRole] = useState<GroupRole>(group.role);
  const [editEnvIds, setEditEnvIds] = useState<string[]>([...group.environment_ids]);
  const [editAppIds, setEditAppIds] = useState<string[]>([...group.application_ids]);

  // Reset edit state when entering edit mode
  useEffect(() => {
    if (isEditing) {
      setEditName(group.name);
      setEditRole(group.role);
      setEditEnvIds([...group.environment_ids]);
      setEditAppIds([...group.application_ids]);
    }
  }, [isEditing, group]);

  return (
    <>
      <tr>
        <td>{group.name}</td>
        <td>
          <span className={`badge badge-${group.role}`}>{group.role}</span>
        </td>
        <td>
          {group.environment_ids.length === 0 ? (
            <span className="badge">All</span>
          ) : (
            group.environment_ids.map((eid) => (
              <span key={eid} className="badge" style={{ marginRight: 4 }}>
                {getEnvironmentName(eid)}
              </span>
            ))
          )}
        </td>
        <td>
          {group.application_ids.length === 0 ? (
            <span className="badge">All</span>
          ) : (
            group.application_ids.map((aid) => (
              <span key={aid} className="badge" style={{ marginRight: 4 }}>
                {appNameById(aid)}
              </span>
            ))
          )}
        </td>
        <td>{group.member_ids.length}</td>
        <td>
          <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
            <button className="btn btn-sm" onClick={onEdit}>
              Edit
            </button>
            {confirmDelete === group.id ? (
              <span className="inline-confirm">
                Are you sure?{' '}
                <button className="btn btn-sm btn-danger" onClick={onDelete}>
                  Yes
                </button>{' '}
                <button className="btn btn-sm" onClick={onCancelDelete}>
                  No
                </button>
              </span>
            ) : (
              <button className="btn btn-sm btn-danger" onClick={onConfirmDelete}>
                Delete
              </button>
            )}
          </div>
        </td>
      </tr>
      {isEditing && (
        <tr>
          <td colSpan={6}>
            <div className="inline-form">
              <div className="inline-form-row">
                <input
                  type="text"
                  value={editName}
                  onChange={(e) => setEditName(e.target.value)}
                  placeholder="Group name"
                  required
                />
                <select value={editRole} onChange={(e) => setEditRole(e.target.value as GroupRole)}>
                  <option value="viewer">Viewer</option>
                  <option value="editor">Editor</option>
                  <option value="admin">Admin</option>
                </select>
              </div>
              <div className="checkbox-group">
                <label style={{ fontWeight: 600 }}>Environment Scope (leave unchecked for all)</label>
                {MOCK_ENVIRONMENTS.map((env) => (
                  <label key={env.id}>
                    <input
                      type="checkbox"
                      checked={editEnvIds.includes(env.id)}
                      onChange={() =>
                        setEditEnvIds((prev) =>
                          prev.includes(env.id) ? prev.filter((id) => id !== env.id) : [...prev, env.id],
                        )
                      }
                    />{' '}
                    {env.name}
                  </label>
                ))}
              </div>
              <div className="checkbox-group">
                <label style={{ fontWeight: 600 }}>Application Scope (leave unchecked for all)</label>
                {MOCK_APPLICATIONS.map((app) => (
                  <label key={app.id}>
                    <input
                      type="checkbox"
                      checked={editAppIds.includes(app.id)}
                      onChange={() =>
                        setEditAppIds((prev) =>
                          prev.includes(app.id) ? prev.filter((id) => id !== app.id) : [...prev, app.id],
                        )
                      }
                    />{' '}
                    {app.name}
                  </label>
                ))}
              </div>
              <div style={{ display: 'flex', gap: 8 }}>
                <button
                  className="btn btn-primary"
                  onClick={() =>
                    onSaveEdit({
                      name: editName.trim(),
                      role: editRole,
                      environment_ids: editEnvIds,
                      application_ids: editAppIds,
                    })
                  }
                >
                  Save
                </button>
                <button className="btn" onClick={onCancelEdit}>
                  Cancel
                </button>
              </div>
            </div>
          </td>
        </tr>
      )}
    </>
  );
}
