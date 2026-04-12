import { useParams, Navigate } from 'react-router-dom';
import SiteHeader from '@/components/SiteHeader';
import DocsSidebar from '@/components/docs/DocsSidebar';
import MarkdownRenderer from '@/components/docs/MarkdownRenderer';
import { findDoc, docsManifest } from '@/docs';

export default function DocsPage() {
  const { slug } = useParams();

  if (!slug) {
    return <Navigate to={`/docs/${docsManifest[0].slug}`} replace />;
  }

  const doc = findDoc(slug);
  if (!doc) {
    return <Navigate to={`/docs/${docsManifest[0].slug}`} replace />;
  }

  return (
    <div className="docs-shell">
      <SiteHeader variant="app" />
      <div className="docs-layout">
        <DocsSidebar />
        <main className="docs-content">
          <MarkdownRenderer source={doc.source} />
        </main>
      </div>
    </div>
  );
}
