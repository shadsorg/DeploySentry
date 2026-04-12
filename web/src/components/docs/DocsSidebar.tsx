import { NavLink } from 'react-router-dom';
import { docsManifest } from '@/docs';

export default function DocsSidebar() {
  return (
    <aside className="docs-sidebar">
      <div className="docs-sidebar-heading">DOCUMENTATION</div>
      <nav className="docs-sidebar-nav">
        {docsManifest.map((doc) => (
          <NavLink
            key={doc.slug}
            to={`/docs/${doc.slug}`}
            className={({ isActive }) => `docs-sidebar-link${isActive ? ' active' : ''}`}
          >
            {doc.title}
          </NavLink>
        ))}
      </nav>
    </aside>
  );
}
