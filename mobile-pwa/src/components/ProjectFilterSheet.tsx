import type { Project } from '../types';

export function ProjectFilterSheet({
  open,
  projects,
  value,
  onSelect,
  onClose,
}: {
  open: boolean;
  projects: Project[];
  value: string;
  onSelect: (projectId: string) => void;
  onClose: () => void;
}) {
  if (!open) return null;

  const choose = (id: string) => {
    onSelect(id);
    onClose();
  };

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label="Filter by project"
      style={{
        position: 'fixed',
        inset: 0,
        background: 'rgba(0,0,0,0.6)',
        display: 'flex',
        alignItems: 'flex-end',
        zIndex: 900,
      }}
      onClick={onClose}
    >
      <div
        onClick={(e) => e.stopPropagation()}
        style={{
          background: 'var(--color-bg-elevated, #1b2339)',
          border: '1px solid var(--color-border, #1e293b)',
          borderRadius: '16px 16px 0 0',
          padding: '16px 20px',
          width: '100%',
          maxHeight: '70vh',
          overflowY: 'auto',
          paddingBottom: 'calc(env(safe-area-inset-bottom) + 16px)',
        }}
      >
        <h3 style={{ margin: '0 0 12px' }}>Project</h3>
        <button
          type="button"
          className="m-button"
          aria-pressed={value === ''}
          onClick={() => choose('')}
          style={{ width: '100%', justifyContent: 'flex-start', marginBottom: 8 }}
        >
          All projects
        </button>
        {projects.map((p) => (
          <button
            key={p.id}
            type="button"
            className="m-button"
            aria-pressed={value === p.id}
            onClick={() => choose(p.id)}
            style={{ width: '100%', justifyContent: 'flex-start', marginBottom: 6 }}
          >
            {p.name}
          </button>
        ))}
      </div>
    </div>
  );
}
