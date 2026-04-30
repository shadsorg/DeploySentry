import { useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { docsManifest, docCategoryOrder, type DocCategory, type DocEntry } from '@/docs';

export default function DocsIndex() {
  const [query, setQuery] = useState('');
  const q = query.trim().toLowerCase();

  const grouped = useMemo(() => {
    const filtered = q
      ? docsManifest.filter(
          (d) =>
            d.title.toLowerCase().includes(q) ||
            d.summary.toLowerCase().includes(q) ||
            d.category.toLowerCase().includes(q),
        )
      : docsManifest;

    const buckets = new Map<DocCategory, DocEntry[]>();
    for (const cat of docCategoryOrder) buckets.set(cat, []);
    for (const doc of filtered) buckets.get(doc.category)?.push(doc);
    return docCategoryOrder
      .map((cat) => ({ category: cat, docs: buckets.get(cat) ?? [] }))
      .filter((g) => g.docs.length > 0);
  }, [q]);

  return (
    <div className="docs-index">
      <header className="docs-index-header">
        <h1>Documentation</h1>
        <p>Setup, configuration, and integration guides for DeploySentry.</p>
        <div className="docs-index-search">
          <span className="ms" aria-hidden="true">
            search
          </span>
          <input
            type="search"
            className="form-input"
            placeholder="Filter by title or summary…"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            autoFocus
          />
        </div>
      </header>

      {grouped.length === 0 && (
        <div className="empty-state">
          <p>No docs match “{query}”.</p>
        </div>
      )}

      {grouped.map(({ category, docs }) => (
        <section key={category} className="docs-index-section">
          <h2 className="docs-index-section-title">{category}</h2>
          <div className="docs-index-grid">
            {docs.map((doc) => (
              <Link key={doc.slug} to={`/docs/${doc.slug}`} className="docs-index-card">
                <div className="docs-index-card-icon">
                  <span className="ms" aria-hidden="true">
                    {doc.icon}
                  </span>
                </div>
                <div className="docs-index-card-body">
                  <div className="docs-index-card-title">{doc.title}</div>
                  <div className="docs-index-card-summary">{doc.summary}</div>
                </div>
                <span className="ms docs-index-card-arrow" aria-hidden="true">
                  arrow_forward
                </span>
              </Link>
            ))}
          </div>
        </section>
      ))}
    </div>
  );
}
