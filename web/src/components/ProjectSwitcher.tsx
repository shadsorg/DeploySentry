import { useState, useRef, useEffect } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useProjects } from '@/hooks/useEntities';

export default function ProjectSwitcher() {
  const { orgSlug, projectSlug } = useParams();
  const navigate = useNavigate();
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  const { projects } = useProjects(orgSlug);

  // Close dropdown on outside click
  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    document.addEventListener('mousedown', handleClick);
    return () => document.removeEventListener('mousedown', handleClick);
  }, []);

  if (!orgSlug) return null;

  function handleSelect(slug: string) {
    setOpen(false);
    if (slug !== projectSlug) {
      navigate(`/orgs/${orgSlug}/projects/${slug}/flags`);
    }
  }

  return (
    <div className="switcher" ref={ref}>
      <button className="switcher-btn" onClick={() => setOpen(!open)}>
        <span className="switcher-label">
          {projectSlug
            ? (projects.find((p) => p.slug === projectSlug)?.name ?? projectSlug)
            : 'Select Project'}
        </span>
        <span className="switcher-arrow">{open ? '\u25B4' : '\u25BE'}</span>
      </button>
      {open && (
        <div className="switcher-dropdown">
          {projects.map((proj) => (
            <button
              key={proj.id}
              className={`switcher-option${proj.slug === projectSlug ? ' active' : ''}`}
              onClick={() => handleSelect(proj.slug)}
            >
              {proj.name}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
