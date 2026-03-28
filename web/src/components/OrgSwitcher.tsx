import { useState, useRef, useEffect } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { getMockOrgs, getOrgName } from '@/mocks/hierarchy';

export default function OrgSwitcher() {
  const { orgSlug } = useParams();
  const navigate = useNavigate();
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  const orgs = getMockOrgs();

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

  function handleSelect(slug: string) {
    setOpen(false);
    if (slug !== orgSlug) {
      localStorage.setItem('ds_last_org', slug);
      navigate(`/orgs/${slug}/projects`);
    }
  }

  return (
    <div className="switcher" ref={ref}>
      <button className="switcher-btn" onClick={() => setOpen(!open)}>
        <span className="switcher-label">{orgSlug ? getOrgName(orgSlug) : 'Select Org'}</span>
        <span className="switcher-arrow">{open ? '\u25B4' : '\u25BE'}</span>
      </button>
      {open && (
        <div className="switcher-dropdown">
          {orgs.map((org) => (
            <button
              key={org.id}
              className={`switcher-option${org.slug === orgSlug ? ' active' : ''}`}
              onClick={() => handleSelect(org.slug)}
            >
              {org.name}
            </button>
          ))}
          <div className="switcher-divider" />
          <button
            className="switcher-option switcher-option-action"
            onClick={() => { setOpen(false); navigate('/orgs/new'); }}
          >
            + Create Organization
          </button>
        </div>
      )}
    </div>
  );
}
